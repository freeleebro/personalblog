package api

import (
	"encoding/json"
	"net/http"
)

// Meta 是分页元信息。
type Meta struct {
	Page       int `json:"page,omitempty"`
	PerPage    int `json:"per_page,omitempty"`
	Total      int `json:"total,omitempty"`
	TotalPages int `json:"total_pages,omitempty"`
}

// envelope 统一响应外壳：成功走 data，失败走 error。
type envelope struct {
	Data any   `json:"data"`
	Meta *Meta `json:"meta,omitempty"`
}

type errorPayload struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const contentTypeJSON = "application/json; charset=utf-8"

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeData 输出成功响应。列表类接口统一带上 30 秒的公共缓存，
// 让反向代理与浏览器可以做一层兜底。
//
// 调用方如果已经自己设过 Cache-Control（比如草稿预览要求 no-store），
// 这里就不覆盖——否则私有内容会被浏览器缓存下来。
func writeData(w http.ResponseWriter, data any, meta *Meta) {
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "public, max-age=30, stale-while-revalidate=300")
	}
	writeJSON(w, http.StatusOK, envelope{Data: data, Meta: meta})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var p errorPayload
	p.Error.Code = code
	p.Error.Message = message
	writeJSON(w, status, p)
}
