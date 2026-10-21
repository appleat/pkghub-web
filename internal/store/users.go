package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrNotFound 记录不存在。
	ErrNotFound = errors.New("记录不存在")
	// ErrDuplicate 唯一约束冲突。
	ErrDuplicate = errors.New("名称已被占用")
	// ErrInvalid 参数不合法。
	ErrInvalid = errors.New("参数不合法")
)

// CreateUser 创建用户。
func (s *Store) CreateUser(username, password, displayName string, role Role) (*User, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 6 {
		return nil, ErrInvalid
	}
	if role != RoleAdmin && role != RoleUser {
		role = RoleUser
	}
	if displayName == "" {
		displayName = username
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := Now()
	res, err := s.db.Exec(`INSERT INTO users (username, password_hash, display_name, role, status, created_at, updated_at) VALUES (?,?,?,?,1,?,?)`,
		username, string(hash), displayName, role, now, now)
	if err != nil {
		if isUniqueErr(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetUser(id)
}

// Authenticate 校验账号密码，成功返回用户信息。
func (s *Store) Authenticate(username, password string) (*User, error) {
	var hash string
	var lastLogin sql.NullString
	u := &User{}
	err := s.db.QueryRow(`SELECT `+userColumns+` FROM users WHERE username = ?`, strings.TrimSpace(username)).
		Scan(&u.ID, &u.Username, &hash, &u.DisplayName, &u.Role, &u.Status, &u.CreatedAt, &u.UpdatedAt, &lastLogin)
	if err != nil {
		return nil, ErrInvalid
	}
	u.LastLoginAt = lastLogin.String
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, ErrInvalid
	}
	if u.Status != 1 {
		return nil, errors.New("账号已被禁用，请联系管理员")
	}
	return u, nil
}

// user 内部查询结构（含密码哈希）。
type userRow struct {
	User
	passwordHash string
}

func scanUser(rows *sql.Rows) (*userRow, error) {
	var u userRow
	var lastLogin sql.NullString
	if err := rows.Scan(&u.ID, &u.Username, &u.passwordHash, &u.DisplayName, &u.Role, &u.Status,
		&u.CreatedAt, &u.UpdatedAt, &lastLogin); err != nil {
		return nil, err
	}
	u.LastLoginAt = lastLogin.String
	return &u, nil
}

const userColumns = `id, username, password_hash, display_name, role, status, created_at, updated_at, last_login_at`

// GetUser 按 ID 查询用户。
func (s *Store) GetUser(id int64) (*User, error) {
	rows, err := s.db.Query(`SELECT `+userColumns+` FROM users WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, ErrNotFound
	}
	u, err := scanUser(rows)
	if err != nil {
		return nil, err
	}
	return &u.User, nil
}

// GetUserByUsername 按用户名查询用户。
func (s *Store) GetUserByUsername(username string) (*User, error) {
	rows, err := s.db.Query(`SELECT `+userColumns+` FROM users WHERE username = ?`, strings.TrimSpace(username))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, ErrNotFound
	}
	u, err := scanUser(rows)
	if err != nil {
		return nil, err
	}
	return &u.User, nil
}

// ListUsers 查询用户列表，keyword 为空时返回全部。
func (s *Store) ListUsers(keyword string) ([]User, error) {
	query := `SELECT ` + userColumns + ` FROM users`
	var args []any
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query += ` WHERE username LIKE ? OR display_name LIKE ?`
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	query += ` ORDER BY CASE role WHEN 'admin' THEN 0 ELSE 1 END, username`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, u.User)
	}
	return list, rows.Err()
}

// UpdateUser 修改用户的显示名、角色与状态。
func (s *Store) UpdateUser(id int64, displayName string, role Role, status int) (*User, error) {
	if displayName == "" {
		return nil, ErrInvalid
	}
	if role != RoleAdmin && role != RoleUser {
		return nil, ErrInvalid
	}
	if status != 0 && status != 1 {
		status = 1
	}
	res, err := s.db.Exec(`UPDATE users SET display_name = ?, role = ?, status = ?, updated_at = ? WHERE id = ?`,
		displayName, role, status, Now(), id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetUser(id)
}

// SetPassword 重置用户密码。
func (s *Store) SetPassword(id int64, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("密码长度至少 6 位")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`, string(hash), Now(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser 删除用户（管理员不能删除自己）。
func (s *Store) DeleteUser(id int64) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateLastLogin 记录最近登录时间。
func (s *Store) UpdateLastLogin(id int64) error {
	_, err := s.db.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, Now(), id)
	return err
}

// CountUsers 统计用户数量。
func (s *Store) CountUsers() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// AdminExists 判断是否存在管理员。
func (s *Store) AdminExists() (bool, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&n)
	return n > 0, err
}

func isUniqueErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "constraint failed")
}

// EnsureDefaultAdmin 在首次运行时创建默认管理员账号，返回是否创建。
func (s *Store) EnsureDefaultAdmin(username, password string) (bool, error) {
	n, err := s.CountUsers()
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	if _, err := s.CreateUser(username, password, "系统管理员", RoleAdmin); err != nil {
		return false, fmt.Errorf("创建默认管理员失败: %w", err)
	}
	return true, nil
}
