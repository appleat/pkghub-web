package store

import "time"

// CreateSession 为用户创建会话，返回令牌。
func (s *Store) CreateSession(userID int64, ttl time.Duration) (string, error) {
	token, err := RandomToken(32)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	_, err = s.db.Exec(`INSERT INTO sessions (token, user_id, created_at, expires_at) VALUES (?,?,?,?)`,
		token, userID, now.Format(time.RFC3339), now.Add(ttl).Format(time.RFC3339))
	if err != nil {
		return "", err
	}
	return token, nil
}

// UserByToken 根据令牌查询有效会话对应的用户。
func (s *Store) UserByToken(token string) (*User, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	var userID int64
	var expires string
	err := s.db.QueryRow(`SELECT user_id, expires_at FROM sessions WHERE token = ?`, token).Scan(&userID, &expires)
	if err != nil {
		return nil, ErrNotFound
	}
	exp, err := time.Parse(time.RFC3339, expires)
	if err != nil || time.Now().UTC().After(exp) {
		_ = s.DeleteSession(token)
		return nil, ErrNotFound
	}
	u, err := s.GetUser(userID)
	if err != nil {
		return nil, err
	}
	if u.Status != 1 {
		return nil, ErrNotFound
	}
	return u, nil
}

// DeleteSession 注销会话。
func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// DeleteUserSessions 删除指定用户的全部会话。
func (s *Store) DeleteUserSessions(userID int64) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// CleanupSessions 清理过期会话。
func (s *Store) CleanupSessions() error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, Now())
	return err
}
