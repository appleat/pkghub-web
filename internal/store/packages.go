package store

import (
	"strconv"
	"strings"
)

// PackageFilter 列表查询条件。
type PackageFilter struct {
	Keyword    string // 名称/描述模糊匹配
	CategoryID int64  // 分类过滤，0 表示全部
	Sort       string // updated | created | name | downloads | size
	Page       int
	PageSize   int
}

const packageColumns = `p.id, p.name, p.slug, p.description,
	COALESCE(p.category_id, 0), COALESCE(c.name, '未分类'), COALESCE(c.color, '#94a3b8'),
	COALESCE(p.owner_id, 0), COALESCE(NULLIF(u.display_name, ''), u.username, '—'),
	p.created_at, p.updated_at,
	(SELECT COUNT(*) FROM versions v WHERE v.package_id = p.id),
	(SELECT COALESCE(SUM(v.downloads), 0) FROM versions v WHERE v.package_id = p.id),
	(SELECT COALESCE(SUM(v.size), 0) FROM versions v WHERE v.package_id = p.id),
	COALESCE((SELECT v.version FROM versions v WHERE v.package_id = p.id ORDER BY v.created_at DESC, v.id DESC LIMIT 1), '')`

const packageFrom = ` FROM packages p
	LEFT JOIN categories c ON c.id = p.category_id
	LEFT JOIN users u ON u.id = p.owner_id`

func scanPackage(rows interface{ Scan(...any) error }) (*Package, error) {
	var p Package
	err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.CategoryID, &p.CategoryName, &p.CategoryColor,
		&p.OwnerID, &p.OwnerName, &p.CreatedAt, &p.UpdatedAt, &p.VersionCount, &p.Downloads, &p.TotalSize, &p.LatestVersion)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func orderBy(sort string) string {
	switch sort {
	case "name":
		return "p.name COLLATE NOCASE ASC"
	case "downloads":
		return "(SELECT COALESCE(SUM(v.downloads),0) FROM versions v WHERE v.package_id = p.id) DESC, p.updated_at DESC"
	case "size":
		return "(SELECT COALESCE(SUM(v.size),0) FROM versions v WHERE v.package_id = p.id) DESC, p.updated_at DESC"
	case "created":
		return "p.created_at DESC"
	default:
		return "p.updated_at DESC"
	}
}

// ListPackages 分页查询软件包。
func (s *Store) ListPackages(f PackageFilter) ([]Package, int64, error) {
	where := " WHERE 1=1"
	var args []any
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		where += " AND (p.name LIKE ? OR p.slug LIKE ? OR p.description LIKE ?)"
		like := "%" + kw + "%"
		args = append(args, like, like, like)
	}
	if f.CategoryID > 0 {
		where += " AND p.category_id = ?"
		args = append(args, f.CategoryID)
	} else if f.CategoryID == -1 {
		where += " AND p.category_id IS NULL"
	}

	var total int64
	if err := s.db.QueryRow(`SELECT COUNT(*)`+packageFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	page, size := f.Page, f.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 12
	}
	query := `SELECT ` + packageColumns + packageFrom + where + ` ORDER BY ` + orderBy(f.Sort) + ` LIMIT ? OFFSET ?`
	args = append(args, size, (page-1)*size)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := []Package{}
	for rows.Next() {
		p, err := scanPackage(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *p)
	}
	return list, total, rows.Err()
}

// GetPackage 按 ID 查询软件包。
func (s *Store) GetPackage(id int64) (*Package, error) {
	row := s.db.QueryRow(`SELECT `+packageColumns+packageFrom+` WHERE p.id = ?`, id)
	p, err := scanPackage(row)
	if err != nil {
		return nil, ErrNotFound
	}
	return p, nil
}

// GetPackageBySlug 按标识查询软件包。
func (s *Store) GetPackageBySlug(slug string) (*Package, error) {
	row := s.db.QueryRow(`SELECT `+packageColumns+packageFrom+` WHERE p.slug = ?`, slug)
	p, err := scanPackage(row)
	if err != nil {
		return nil, ErrNotFound
	}
	return p, nil
}

// ResolvePackage 支持使用 ID 或 slug 定位软件包。
func (s *Store) ResolvePackage(idOrSlug string) (*Package, error) {
	if id, err := strconv.ParseInt(idOrSlug, 10, 64); err == nil {
		return s.GetPackage(id)
	}
	return s.GetPackageBySlug(idOrSlug)
}

// CreatePackage 新建软件包。
func (s *Store) CreatePackage(name, description string, categoryID, ownerID int64) (*Package, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalid
	}
	slug, err := s.uniqueSlug("packages", Slugify(name), 0)
	if err != nil {
		return nil, err
	}
	var cat any
	if categoryID > 0 {
		cat = categoryID
	}
	now := Now()
	res, err := s.db.Exec(`INSERT INTO packages (name, slug, description, category_id, owner_id, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`,
		name, slug, description, cat, ownerID, now, now)
	if err != nil {
		if isUniqueErr(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetPackage(id)
}

// UpdatePackage 更新软件包信息。
func (s *Store) UpdatePackage(id int64, name, description string, categoryID int64) (*Package, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalid
	}
	slug, err := s.uniqueSlug("packages", Slugify(name), id)
	if err != nil {
		return nil, err
	}
	var cat any
	if categoryID > 0 {
		cat = categoryID
	}
	res, err := s.db.Exec(`UPDATE packages SET name = ?, slug = ?, description = ?, category_id = ?, updated_at = ? WHERE id = ?`,
		name, slug, description, cat, Now(), id)
	if err != nil {
		if isUniqueErr(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetPackage(id)
}

// DeletePackage 删除软件包（数据库级联删除版本，文件由调用方清理）。
func (s *Store) DeletePackage(id int64) error {
	res, err := s.db.Exec(`DELETE FROM packages WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchPackage 更新软件包的修改时间。
func (s *Store) TouchPackage(id int64) error {
	_, err := s.db.Exec(`UPDATE packages SET updated_at = ? WHERE id = ?`, Now(), id)
	return err
}
