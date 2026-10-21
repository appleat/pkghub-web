package web

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"goPkgMgr/internal/store"
)

type envelope struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// writeJSON 输出 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ok 输出成功响应。
func ok(w http.ResponseWriter, data any) {
	if data == nil {
		data = map[string]any{}
	}
	writeJSON(w, http.StatusOK, envelope{OK: true, Data: data})
}

// fail 输出失败响应，并自动映射常见错误的 HTTP 状态码。
func fail(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	msg := "服务器内部错误"
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			status, msg = http.StatusNotFound, err.Error()
		case errors.Is(err, store.ErrDuplicate):
			status, msg = http.StatusConflict, err.Error()
		case errors.Is(err, store.ErrInvalid):
			status, msg = http.StatusBadRequest, "参数不合法"
		case errors.Is(err, errForbidden):
			status, msg = http.StatusForbidden, err.Error()
		case errors.Is(err, errUnauthorized):
			status, msg = http.StatusUnauthorized, err.Error()
		default:
			msg = err.Error()
		}
		if status >= 500 {
			log.Printf("[error] %v", err)
		}
	}
	writeJSON(w, status, envelope{OK: false, Error: msg})
}

// badRequest 输出 400 响应。
func badRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, envelope{OK: false, Error: msg})
}
