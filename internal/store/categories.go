package store

import (
	"errors"
	"strings"
)

const categoryColumns = `c.id, c.name, c.slug, c.description, c.color, c.created_at,
	(SELECT COUNT(*) FROM packages p WHERE p.category_id = c.id)`

// ListCategories 返回分类列表（含软件包数量）。
func (s *Store) ListCategories() ([]Category, error) {
	rows, err := s.db.Query(`SELECT ` + categoryColumns + ` FROM categories c ORDER BY c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Color, &c.CreatedAt, &c.PackageCount); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// GetCategory 按 ID 查询分类。
func (s *Store) GetCategory(id int64) (*Category, error) {
	var c Category
	err := s.db.QueryRow(`SELECT `+categoryColumns+` FROM categories c WHERE c.id = ?`, id).
		Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Color, &c.CreatedAt, &c.PackageCount)
	if err != nil {
		return nil, ErrNotFound
	}
	return &c, nil
}

// CreateCategory 新建分类。
func (s *Store) CreateCategory(name, description, color string) (*Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalid
	}
	if color == "" {
		color = "#6366f1"
	}
	slug, err := s.uniqueSlug("categories", Slugify(name), 0)
	if err != nil {
		return nil, err
	}
	res, err := s.db.Exec(`INSERT INTO categories (name, slug, description, color, created_at) VALUES (?,?,?,?,?)`,
		name, slug, description, color, Now())
	if err != nil {
		if isUniqueErr(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetCategory(id)
}

// UpdateCategory 更新分类信息。
func (s *Store) UpdateCategory(id int64, name, description, color string) (*Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalid
	}
	if color == "" {
		color = "#6366f1"
	}
	slug, err := s.uniqueSlug("categories", Slugify(name), id)
	if err != nil {
		return nil, err
	}
	res, err := s.db.Exec(`UPDATE categories SET name = ?, slug = ?, description = ?, color = ? WHERE id = ?`,
		name, slug, description, color, id)
	if err != nil {
		if isUniqueErr(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetCategory(id)
}

// DeleteCategory 删除分类；若分类下仍有软件包则拒绝删除。
func (s *Store) DeleteCategory(id int64) error {
	var count int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM packages WHERE category_id = ?`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该分类下仍有软件包，无法删除")
	}
	res, err := s.db.Exec(`DELETE FROM categories WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// EnsureDefaultCategory 保证至少存在一个默认分类。
func (s *Store) EnsureDefaultCategory() error {
	var n int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM categories`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err := s.CreateCategory("通用软件", "默认分类，用于未明确归类的软件包", "#6366f1")
	return err
}
