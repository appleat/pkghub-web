package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type categoryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListCategories()
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, list)
}

func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) {
	var req categoryRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		badRequest(w, "请求格式错误")
		return
	}
	c, err := s.store.CreateCategory(strings.TrimSpace(req.Name), strings.TrimSpace(req.Description), strings.TrimSpace(req.Color))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, c)
}

func (s *Server) updateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		badRequest(w, "分类 ID 非法")
		return
	}
	var req categoryRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		badRequest(w, "请求格式错误")
		return
	}
	c, err := s.store.UpdateCategory(id, strings.TrimSpace(req.Name), strings.TrimSpace(req.Description), strings.TrimSpace(req.Color))
	if err != nil {
		fail(w, err)
		return
	}
	ok(w, c)
}

func (s *Server) deleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		badRequest(w, "分类 ID 非法")
		return
	}
	if err := s.store.DeleteCategory(id); err != nil {
		fail(w, err)
		return
	}
	ok(w, nil)
}
