package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"goPkgMgr/internal/store"
)

type packageRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CategoryID  int64  `json:"category_id"`
}

// canManagePackage 判断当前用户是否有权修改/删除该软件包。
func canManagePackage(u *store.User, p *store.Package) bool {
	if u == nil {
		return false
	}
	return u.IsAdmin() || (p.OwnerID == u.ID)
}

func (s *Server) listPackages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	size, _ := strconv.Atoi(q.Get("page_size"))
	catID, _ := strconv.ParseInt(q.Get("category_id"), 10, 64)
	f := store.PackageFilter{
		Keyword:    q.Get("keyword"),
		CategoryID: catID,
		Sort:       q.Get("sort"),
		Page:       page,
		PageSize:   size,
	}
	list, total, err := s.store.ListPackages(f)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, map[string]any{"items": list, "total": total, "page": f.Page, "page_size": f.PageSize})
}

func (s *Server) createPackage(w http.ResponseWriter, r *http.Request) {
	u := CurrentUser(r)
	var req packageRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		badRequest(w, "请求格式错误")
		return
	}
	p, err := s.store.CreatePackage(req.Name, req.Description, req.CategoryID, u.ID)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, p)
}

func (s *Server) getPackage(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.ResolvePackage(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	versions, err := s.store.ListVersions(p.ID)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, map[string]any{"package": p, "versions": versions})
}

func (s *Server) updatePackage(w http.ResponseWriter, r *http.Request) {
	u := CurrentUser(r)
	p, err := s.store.ResolvePackage(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	if !canManagePackage(u, p) {
		fail(w, errForbidden)
		return
	}
	var req packageRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		badRequest(w, "请求格式错误")
		return
	}
	updated, err := s.store.UpdatePackage(p.ID, req.Name, req.Description, req.CategoryID)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, updated)
}

func (s *Server) deletePackage(w http.ResponseWriter, r *http.Request) {
	u := CurrentUser(r)
	p, err := s.store.ResolvePackage(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	if !canManagePackage(u, p) {
		fail(w, errForbidden)
		return
	}
	paths, err := s.store.PackageFilePaths(p.ID)
	if err != nil {
		fail(w, err)
		return
	}
	if err := s.store.DeletePackage(p.ID); err != nil {
		fail(w, err)
		return
	}
	removeFiles(s.filesDir, paths)
	ok(w, nil)
}

func (s *Server) listVersions(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.ResolvePackage(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	list, err := s.store.ListVersions(p.ID)
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, list)
}
