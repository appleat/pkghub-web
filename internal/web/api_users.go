package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"goPkgMgr/internal/store"
)

type userRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Status      int    `json:"status"`
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListUsers(r.URL.Query().Get("keyword"))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, list)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req userRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		badRequest(w, "请求格式错误")
		return
	}
	role := store.Role(req.Role)
	if role != store.RoleAdmin && role != store.RoleUser {
		role = store.RoleUser
	}
	if len(req.Password) < 6 {
		badRequest(w, "密码长度至少 6 位")
		return
	}
	u, err := s.store.CreateUser(strings.TrimSpace(req.Username), req.Password, strings.TrimSpace(req.DisplayName), role)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, u)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	current := CurrentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		badRequest(w, "用户 ID 非法")
		return
	}
	target, err := s.store.GetUser(id)
	if err != nil {
		fail(w, err)
		return
	}
	var req userRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		badRequest(w, "请求格式错误")
		return
	}
	role := target.Role
	if req.Role != "" {
		role = store.Role(req.Role)
		if role != store.RoleAdmin && role != store.RoleUser {
			badRequest(w, "角色参数非法")
			return
		}
	}
	status := req.Status
	if status != 0 && status != 1 {
		status = target.Status
	}
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = target.DisplayName
	}
	// 不允许把自己降级或禁用，避免出现无管理员的系统
	if target.ID == current.ID {
		if role != store.RoleAdmin || status != 1 {
			badRequest(w, "不能修改自己的角色或停用自己")
			return
		}
	}
	// 禁止停用最后一个管理员
	if target.Role == store.RoleAdmin && (role != store.RoleAdmin || status != 1) {
		var admins int64
		if err := s.store.DB().QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 1`).Scan(&admins); err == nil && admins <= 1 {
			badRequest(w, "系统至少需要保留一个启用状态的管理员")
			return
		}
	}
	u, err := s.store.UpdateUser(id, displayName, role, status)
	if err != nil {
		fail(w, err)
		return
	}
	if status == 0 {
		_ = s.store.DeleteUserSessions(id)
	}
	ok(w, u)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	current := CurrentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		badRequest(w, "用户 ID 非法")
		return
	}
	if id == current.ID {
		badRequest(w, "不能删除当前登录的账号")
		return
	}
	target, err := s.store.GetUser(id)
	if err != nil {
		fail(w, err)
		return
	}
	if target.Role == store.RoleAdmin {
		admins, cerr := s.store.CountUsers()
		if cerr == nil && admins <= 1 {
			badRequest(w, "系统至少需要保留一个管理员")
			return
		}
	}
	if err := s.store.DeleteUser(id); err != nil {
		fail(w, err)
		return
	}
	ok(w, nil)
}

func (s *Server) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		badRequest(w, "用户 ID 非法")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		badRequest(w, "请求格式错误")
		return
	}
	if len(req.Password) < 6 {
		badRequest(w, "密码长度至少 6 位")
		return
	}
	if err := s.store.SetPassword(id, req.Password); err != nil {
		fail(w, err)
		return
	}
	_ = s.store.DeleteUserSessions(id)
	ok(w, nil)
}
