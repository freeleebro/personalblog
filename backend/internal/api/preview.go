package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/freeleebro/personalblog/backend/internal/store"
)

// previewTTL 是草稿预览链接的有效期。够发给自己在手机上开一眼。
const previewTTL = time.Hour

// signPreview 用 ADMIN_SECRET 给「slug + 过期时间」签名。
// 换成任何别的 secret 都会失效，所以不需要额外存表。
func (s *Server) signPreview(slug string, exp int64) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.AdminSecret))
	fmt.Fprintf(mac, "%s|%d", slug, exp)
	return hex.EncodeToString(mac.Sum(nil))
}

// handleAdminPreviewLink 给后台生成一条带签名的草稿预览链接。
func (s *Server) handleAdminPreviewLink(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if _, err := s.store.GetPostBySlugAnyStatus(r.Context(), slug); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "post_not_found", "文章不存在")
			return
		}
		s.fail(w, "查询文章失败", err)
		return
	}

	exp := time.Now().Add(previewTTL).Unix()
	link := fmt.Sprintf("/preview/%s?t=%s&exp=%d",
		url.PathEscape(slug), s.signPreview(slug, exp), exp)

	w.Header().Set("Cache-Control", "no-store")
	writeData(w, map[string]any{
		"url":        link,
		"expires_at": time.Unix(exp, 0).UTC(),
	}, nil)
}

// handlePreview 校验签名与有效期后返回任意状态的文章（含草稿）。
//
// 语义：签名不对 → 404（当作不存在，不提示 token 有误）；
// 签名对但过期 → 410，方便页面提示「链接已过期，回后台重新生成」。
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	query := r.URL.Query()

	exp, err := strconv.ParseInt(strings.TrimSpace(query.Get("exp")), 10, 64)
	if err != nil {
		writeError(w, http.StatusNotFound, "preview_not_found", "预览链接无效")
		return
	}

	want := s.signPreview(slug, exp)
	got := strings.TrimSpace(query.Get("t"))
	if !hmac.Equal([]byte(want), []byte(got)) {
		writeError(w, http.StatusNotFound, "preview_not_found", "预览链接无效")
		return
	}
	if time.Now().Unix() > exp {
		writeError(w, http.StatusGone, "preview_expired", "预览链接已过期，请回后台重新生成")
		return
	}

	post, err := s.store.GetPostBySlugAnyStatus(r.Context(), slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "post_not_found", "文章不存在")
			return
		}
		s.fail(w, "查询文章失败", err)
		return
	}

	// 预览内容不进搜索引擎，也不让中间层缓存。
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	writeData(w, post, nil)
}
