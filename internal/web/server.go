// Package web 提供 HTTP 服务：页面、REST API、上传与下载。
package web

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goPkgMgr/internal/store"
)

// Config 服务运行配置。
type Config struct {
	AppName    string        // 平台名称（页面标题、登录页展示）
	DataDir    string        // 数据根目录
	DBPath     string        // 数据库文件路径
	FilesDir   string        // 软件包文件存储目录（为空则使用 <DataDir>/files）
	MaxUpload  int64         // 单文件上传上限（字节）
	SessionTTL time.Duration // 会话有效期
	Version    string        // 程序版本，用于页面展示
}

// Server HTTP 服务。
type Server struct {
	cfg      Config
	store    *store.Store
	filesDir string
	mux      *http.ServeMux
	index    *template.Template
	started  time.Time
}

// New 创建服务实例。
func New(st *store.Store, cfg Config) (*Server, error) {
	filesDir := strings.TrimSpace(cfg.FilesDir)
	if filesDir == "" {
		filesDir = filepath.Join(cfg.DataDir, "files")
	}
	if abs, err := filepath.Abs(filesDir); err == nil {
		filesDir = abs
	}
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		return nil, fmt.Errorf("无法创建文件存储目录 %s: %w", filesDir, err)
	}
	tmpl, err := template.New("index").Parse(string(IndexTemplate()))
	if err != nil {
		return nil, fmt.Errorf("解析首页模板失败: %w", err)
	}
	s := &Server{cfg: cfg, store: st, filesDir: filesDir, mux: http.NewServeMux(), index: tmpl, started: time.Now()}
	s.routes()
	return s, nil
}

func (s *Server) routes() {
	mux := s.mux

	// 页面与静态资源
	mux.HandleFunc("GET /", s.serveIndex)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(StaticFS()))))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(faviconSVG))
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"status": "ok", "uptime": time.Since(s.started).String(), "version": s.cfg.Version})
	})

	// 认证
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("GET /api/auth/me", s.requireAuth(s.me))

	// 统计
	mux.HandleFunc("GET /api/stats/overview", s.requireAuth(s.overview))

	// 分类
	mux.HandleFunc("GET /api/categories", s.requireAuth(s.listCategories))
	mux.HandleFunc("POST /api/categories", s.requireAdmin(s.createCategory))
	mux.HandleFunc("PUT /api/categories/{id}", s.requireAdmin(s.updateCategory))
	mux.HandleFunc("DELETE /api/categories/{id}", s.requireAdmin(s.deleteCategory))

	// 软件包（新增与编辑仅管理员可用，普通用户只能浏览、下载与上传版本）
	mux.HandleFunc("GET /api/packages", s.requireAuth(s.listPackages))
	mux.HandleFunc("POST /api/packages", s.requireAdmin(s.createPackage))
	mux.HandleFunc("GET /api/packages/{id}", s.requireAuth(s.getPackage))
	mux.HandleFunc("PUT /api/packages/{id}", s.requireAdmin(s.updatePackage))
	mux.HandleFunc("DELETE /api/packages/{id}", s.requireAuth(s.deletePackage))
	mux.HandleFunc("GET /api/packages/{id}/versions", s.requireAuth(s.listVersions))
	mux.HandleFunc("POST /api/packages/{id}/versions", s.requireAuth(s.uploadVersion))
	mux.HandleFunc("GET /api/packages/{id}/download", s.requireAuth(s.downloadLatest))

	// 版本
	mux.HandleFunc("GET /api/versions/{id}/download", s.requireAuth(s.downloadVersion))
	mux.HandleFunc("DELETE /api/versions/{id}", s.requireAuth(s.deleteVersion))

	// 用户
	mux.HandleFunc("GET /api/users", s.requireAdmin(s.listUsers))
	mux.HandleFunc("POST /api/users", s.requireAdmin(s.createUser))
	mux.HandleFunc("PUT /api/users/{id}", s.requireAdmin(s.updateUser))
	mux.HandleFunc("DELETE /api/users/{id}", s.requireAdmin(s.deleteUser))
	mux.HandleFunc("POST /api/users/{id}/password", s.requireAdmin(s.resetUserPassword))

	// 个人中心
	mux.HandleFunc("PUT /api/account/profile", s.requireAuth(s.updateProfile))
	mux.HandleFunc("POST /api/account/password", s.requireAuth(s.changePassword))

	// 未匹配的非 API 请求回退到首页，便于前端路由与深链访问
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusNotFound, envelope{OK: false, Error: "接口不存在"})
			return
		}
		s.serveIndex(w, r)
	})
}

// Handler 返回 http.Handler。
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	appName := strings.TrimSpace(s.cfg.AppName)
	if appName == "" {
		appName = "PkgHub"
	}
	data := map[string]any{
		"AppName": appName,
		"Version": s.cfg.Version,
		"MaxMB":   s.cfg.MaxUpload / (1 << 20),
	}
	if err := s.index.Execute(w, data); err != nil {
		log.Printf("[error] 渲染首页失败: %v", err)
	}
}

const faviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">
<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#6366f1"/><stop offset="1" stop-color="#8b5cf6"/></linearGradient></defs>
<rect width="64" height="64" rx="16" fill="url(#g)"/>
<path d="M32 14l16 8v20l-16 8-16-8V22z" fill="none" stroke="#fff" stroke-width="4" stroke-linejoin="round"/>
<path d="M16 22l16 8 16-8M32 30v20" fill="none" stroke="#fff" stroke-width="4" stroke-linejoin="round"/>
</svg>`
