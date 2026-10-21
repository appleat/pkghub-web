package web

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"goPkgMgr/internal/store"
)

// sanitizeFileName 清理上传文件名，避免路径穿越与非法字符。
func sanitizeFileName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r == '.' || r == '-' || r == '_' || r == '+' || r == '(' || r == ')' || r == '[' || r == ']':
			b.WriteRune(r)
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ':
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	out = strings.Trim(out, ".")
	if out == "" || out == ".." {
		out = "package.bin"
	}
	if len(out) > 120 {
		out = out[len(out)-120:]
	}
	return out
}

// uploadVersion 处理软件包新版本上传（multipart/form-data）。
func (s *Server) uploadVersion(w http.ResponseWriter, r *http.Request) {
	u := CurrentUser(r)
	pkg, err := s.store.ResolvePackage(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	// 普通用户只能向自己创建的软件包上传版本。
	if !u.IsAdmin() && pkg.OwnerID != u.ID {
		fail(w, errForbidden)
		return
	}

	maxBytes := s.cfg.MaxUpload + (2 << 20)
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		badRequest(w, fmt.Sprintf("上传数据解析失败，单个文件不得超过 %d MB", s.cfg.MaxUpload/(1<<20)))
		return
	}
	version := strings.TrimSpace(r.PostFormValue("version"))
	notes := strings.TrimSpace(r.PostFormValue("notes"))
	if version == "" {
		badRequest(w, "请填写版本号")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		badRequest(w, "请选择要上传的文件")
		return
	}
	defer file.Close()

	safeName := sanitizeFileName(header.Filename)
	relDir := filepath.Join("packages", strconv.FormatInt(pkg.ID, 10))
	absDir := filepath.Join(s.filesDir, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		fail(w, err)
		return
	}
	token, err := store.RandomToken(8)
	if err != nil {
		fail(w, err)
		return
	}
	relPath := filepath.Join(relDir, token+"-"+safeName)
	absPath := filepath.Join(s.filesDir, relPath)
	tmpPath := absPath + ".part"

	out, err := os.Create(tmpPath)
	if err != nil {
		fail(w, err)
		return
	}
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, hasher), file)
	out.Close()
	if err != nil {
		os.Remove(tmpPath)
		badRequest(w, fmt.Sprintf("文件写入失败：%v", err))
		return
	}
	if written == 0 {
		os.Remove(tmpPath)
		badRequest(w, "不能上传空文件")
		return
	}
	if written > s.cfg.MaxUpload {
		os.Remove(tmpPath)
		badRequest(w, fmt.Sprintf("文件大小超出限制（最大 %d MB）", s.cfg.MaxUpload/(1<<20)))
		return
	}
	if err := os.Rename(tmpPath, absPath); err != nil {
		os.Remove(tmpPath)
		fail(w, err)
		return
	}

	v, err := s.store.CreateVersion(pkg.ID, version, safeName, filepath.ToSlash(relPath),
		hex.EncodeToString(hasher.Sum(nil)), notes, written, u.ID)
	if err != nil {
		os.Remove(absPath)
		fail(w, err)
		return
	}
	ok(w, v)
}

func (s *Server) deleteVersion(w http.ResponseWriter, r *http.Request) {
	u := CurrentUser(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		badRequest(w, "版本 ID 非法")
		return
	}
	v, err := s.store.GetVersion(id)
	if err != nil {
		fail(w, err)
		return
	}
	if !u.IsAdmin() && v.UploaderID != u.ID {
		// 软件包的拥有者同样可以删除其版本
		pkg, perr := s.store.GetPackage(v.PackageID)
		if perr != nil || pkg.OwnerID != u.ID {
			fail(w, errForbidden)
			return
		}
	}
	if _, err := s.store.DeleteVersion(id); err != nil {
		fail(w, err)
		return
	}
	removeFiles(s.filesDir, []string{v.FilePath})
	ok(w, nil)
}

// downloadVersion 下载指定版本文件。
func (s *Server) downloadVersion(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		badRequest(w, "版本 ID 非法")
		return
	}
	v, err := s.store.GetVersion(id)
	if err != nil {
		fail(w, err)
		return
	}
	s.serveFile(w, r, v)
}

// downloadLatest 下载软件包的最新版本。
func (s *Server) downloadLatest(w http.ResponseWriter, r *http.Request) {
	pkg, err := s.store.ResolvePackage(r.PathValue("id"))
	if err != nil {
		fail(w, err)
		return
	}
	v, err := s.store.LatestVersion(pkg.ID)
	if err != nil {
		fail(w, errors.New("该软件包还没有任何版本"))
		return
	}
	s.serveFile(w, r, v)
}

// serveFile 以附件形式返回文件，支持断点续传。
func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, v *store.Version) {
	abs, err := filepath.Abs(filepath.Join(s.filesDir, filepath.FromSlash(v.FilePath)))
	if err != nil {
		fail(w, err)
		return
	}
	root, _ := filepath.Abs(s.filesDir)
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		fail(w, store.ErrNotFound)
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		fail(w, errors.New("文件已不存在，可能已被清理"))
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		fail(w, err)
		return
	}

	_ = s.store.BumpDownloads(v.ID)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", contentDisposition(v.FileName))
	w.Header().Set("X-File-Sha256", v.SHA256)
	w.Header().Set("X-Package-Version", v.Version)
	http.ServeContent(w, r, v.FileName, st.ModTime(), f)
}

// contentDisposition 生成兼容中文文件名的下载头。
func contentDisposition(name string) string {
	var ascii strings.Builder
	for _, r := range name {
		if r >= 32 && r < 127 && r != '"' && r != '\\' {
			ascii.WriteRune(r)
		} else {
			ascii.WriteRune('_')
		}
	}
	fallback := strings.TrimSpace(ascii.String())
	if fallback == "" || !utf8.ValidString(fallback) {
		fallback = "package.bin"
	}
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, fallback, url.PathEscape(name))
}

// removeFiles 清理磁盘上的版本文件（忽略错误）。
func removeFiles(root string, relPaths []string) {
	for _, p := range relPaths {
		if p == "" {
			continue
		}
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(p)))
	}
}
