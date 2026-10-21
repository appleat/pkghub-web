// Package store 提供基于 SQLite 的数据持久化能力。
package store

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // 纯 Go 实现的 SQLite 驱动，无需 CGO
)

// Store 是数据库访问的入口。
type Store struct {
	db   *sql.DB
	Path string
}

// Open 打开（必要时创建）指定路径的 SQLite 数据库，并执行结构迁移。
func Open(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if dir := filepath.Dir(abs); dir != "" {
		if err := ensureDir(dir); err != nil {
			return nil, err
		}
	}

	dsn := (&url.URL{
		Scheme:   "file",
		Path:     "/" + filepath.ToSlash(abs),
		RawQuery: "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)",
	}).String()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite 对并发写入支持有限，限制连接池规模可有效避免 "database is locked"。
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	s := &Store{db: db, Path: abs}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// DB 暴露底层 *sql.DB，便于扩展。
func (s *Store) DB() *sql.DB { return s.db }

// Close 关闭数据库连接。
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("初始化数据库结构失败: %w", err)
	}
	return nil
}

// Now 返回统一格式（RFC3339/UTC）的当前时间字符串。
func Now() string { return time.Now().UTC().Format(time.RFC3339) }

// NormalizeTime 尝试把任意时间字符串转换为 RFC3339 输出，失败时原样返回。
func NormalizeTime(v string) string {
	if v == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return v
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	username      TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	display_name  TEXT NOT NULL DEFAULT '',
	role          TEXT NOT NULL DEFAULT 'user',
	status        INTEGER NOT NULL DEFAULT 1,
	created_at    TEXT NOT NULL,
	updated_at    TEXT NOT NULL,
	last_login_at TEXT
);

CREATE TABLE IF NOT EXISTS categories (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT NOT NULL UNIQUE,
	slug        TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	color       TEXT NOT NULL DEFAULT '#6366f1',
	created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS packages (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT NOT NULL UNIQUE,
	slug        TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
	owner_id    INTEGER REFERENCES users(id) ON DELETE SET NULL,
	created_at  TEXT NOT NULL,
	updated_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_packages_category ON packages(category_id);

CREATE TABLE IF NOT EXISTS versions (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	package_id  INTEGER NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
	version     TEXT NOT NULL,
	file_name   TEXT NOT NULL,
	file_path   TEXT NOT NULL,
	size        INTEGER NOT NULL DEFAULT 0,
	sha256      TEXT NOT NULL DEFAULT '',
	downloads   INTEGER NOT NULL DEFAULT 0,
	notes       TEXT NOT NULL DEFAULT '',
	uploader_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
	created_at  TEXT NOT NULL,
	UNIQUE (package_id, version)
);
CREATE INDEX IF NOT EXISTS idx_versions_package ON versions(package_id);

CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created_at TEXT NOT NULL,
	expires_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`

// Slugify 将名称转换为可用于 URL 的标识。
func Slugify(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune('-')
		case r == ' ':
			b.WriteRune('-')
		case r > 0x7f: // 保留中文等字符
			b.WriteRune(r)
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len([]rune(slug)) > 64 {
		slug = string([]rune(slug)[:64])
	}
	if slug == "" {
		slug = "pkg"
	}
	return slug
}
