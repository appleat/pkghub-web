package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

func ensureDir(dir string) error {
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s 已存在但不是目录", dir)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

// RandomToken 生成随机令牌。
func RandomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// uniqueSlug 在给定 slug 已被占用时追加序号，保证唯一。
func (s *Store) uniqueSlug(table, base string, excludeID int64) (string, error) {
	slug := base
	for i := 2; i < 1000; i++ {
		var id int64
		err := s.db.QueryRow(fmt.Sprintf(`SELECT id FROM %s WHERE slug = ?`, table), slug).Scan(&id)
		if errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
			return slug, nil
		}
		if err != nil {
			return "", err
		}
		if excludeID != 0 && id == excludeID {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	return "", errors.New("无法生成唯一标识")
}
