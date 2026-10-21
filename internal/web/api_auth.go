package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		badRequest(w, "请求格式错误")
		return
	}
	u, err := s.store.Authenticate(req.Username, req.Password)
	if err != nil {
		fail(w, errors.New("用户名或密码错误"))
		return
	}
	ttl := s.cfg.SessionTTL
	if req.Remember {
		ttl = 7 * 24 * time.Hour
	}
	token, err := s.store.CreateSession(u.ID, ttl)
	if err != nil {
		fail(w, err)
		return
	}
	setSessionCookie(w, token, ttl)
	_ = s.store.UpdateLastLogin(u.ID)
	ok(w, u)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if token := tokenFromRequest(r); token != "" {
		_ = s.store.DeleteSession(token)
	}
	clearSessionCookie(w)
	ok(w, nil)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u := CurrentUser(r)
	appName := s.cfg.AppName
	if strings.TrimSpace(appName) == "" {
		appName = "PkgHub"
	}
	ok(w, map[string]any{
		"user":        u,
		"app_name":    appName,
		"version":     s.cfg.Version,
		"max_mb":      s.cfg.MaxUpload / (1 << 20),
		"data_dir":    s.cfg.DataDir,
		"storage_dir": s.filesDir,
		"server_at":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	u := CurrentUser(r)
	var req struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		badRequest(w, "请求格式错误")
		return
	}
	name := strings.TrimSpace(req.DisplayName)
	if name == "" {
		badRequest(w, "昵称不能为空")
		return
	}
	updated, err := s.store.UpdateUser(u.ID, name, u.Role, u.Status)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, updated)
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	u := CurrentUser(r)
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		badRequest(w, "请求格式错误")
		return
	}
	if _, err := s.store.Authenticate(u.Username, req.OldPassword); err != nil {
		badRequest(w, "原密码不正确")
		return
	}
	if err := s.store.SetPassword(u.ID, req.NewPassword); err != nil {
		fail(w, err)
		return
	}
	// 修改密码后强制重新登录
	if token := tokenFromRequest(r); token != "" {
		_ = s.store.DeleteUserSessions(u.ID)
	}
	clearSessionCookie(w)
	ok(w, nil)
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.Overview()
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, data)
}
