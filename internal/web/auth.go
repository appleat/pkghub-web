package web

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"goPkgMgr/internal/store"
)

var (
	errUnauthorized = errors.New("未登录或登录已过期")
	errForbidden    = errors.New("没有操作权限")
)

const sessionCookie = "pkgmgr_session"

type ctxKey int

const ctxUser ctxKey = 1

// withUser 将用户写入请求上下文。
func withUser(r *http.Request, u *store.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxUser, u))
}

// CurrentUser 取出当前登录用户，未登录返回 nil。
func CurrentUser(r *http.Request) *store.User {
	u, _ := r.Context().Value(ctxUser).(*store.User)
	return u
}

// tokenFromRequest 依次从 Cookie 与 Authorization 头中提取会话令牌。
func tokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		return c.Value
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// authOptional 解析登录态但不强制要求登录。
func (s *Server) authOptional(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if u, err := s.store.UserByToken(tokenFromRequest(r)); err == nil {
			r = withUser(r, u)
		}
		next(w, r)
	}
}

// requireAuth 要求登录，否则返回 401。
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := s.store.UserByToken(tokenFromRequest(r))
		if err != nil || u == nil {
			fail(w, errUnauthorized)
			return
		}
		next(w, withUser(r, u))
	}
}

// requireAdmin 要求管理员权限。
func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		u := CurrentUser(r)
		if u == nil || !u.IsAdmin() {
			fail(w, errForbidden)
			return
		}
		next(w, r)
	})
}

// setSessionCookie 写入会话 Cookie。
func setSessionCookie(w http.ResponseWriter, token string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
		Expires:  time.Now().Add(ttl),
	})
}

// clearSessionCookie 注销会话 Cookie。
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Now().Add(-time.Hour),
	})
}
