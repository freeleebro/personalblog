package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/freeleebro/personalblog/backend/internal/middleware"
	"github.com/freeleebro/personalblog/backend/internal/store"
)

const (
	// 统计请求很小：路径 + 来源站点，1KB 足够。
	maxCollectBodyBytes = 1 << 10
	maxCollectPathRunes = 200
	maxReferrerRunes    = 120

	// 访问记录保留 90 天，够看季度趋势，又不会把库撑大。
	pageViewRetention = 90 * 24 * time.Hour
)

type collectInput struct {
	Path     string `json:"path"`
	Referrer string `json:"referrer"`
}

// normalizeCollectPath 只接受站内页面路径：去掉查询串和锚点，
// 丢弃后台、接口、静态资源，避免把统计表写成日志。
func normalizeCollectPath(raw string) (string, bool) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", false
	}
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if !strings.HasPrefix(p, "/") {
		return "", false
	}
	for _, prefix := range []string{"/admin", "/api", "/api-proxy", "/_next", "/preview", "/media"} {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return "", false
		}
	}
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		p = "/"
	}
	if utf8.RuneCountInString(p) > maxCollectPathRunes {
		return "", false
	}
	return p, true
}

// normalizeReferrer 只留来源站点的 host，不存完整 URL（可能带查询参数）。
func normalizeReferrer(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.ToLower(u.Host)
	if utf8.RuneCountInString(host) > maxReferrerRunes {
		return ""
	}
	return host
}

// isBotUA 用最常见的爬虫关键字过滤，宁可放过也不误杀真人。
func isBotUA(ua string) bool {
	ua = strings.ToLower(ua)
	for _, kw := range []string{"bot", "crawler", "spider", "slurp", "bingpreview", "headless", "curl/", "wget/"} {
		if strings.Contains(ua, kw) {
			return true
		}
	}
	return false
}

// visitorHash 生成「按天加盐」的访客标识：
// 同一 IP + UA 在同一天是同一个 UV，跨天无法关联，也不存原文。
func visitorHash(ip, ua, day, secret string) string {
	sum := sha256.Sum256([]byte(ip + "|" + ua + "|" + day + "|" + secret))
	return hex.EncodeToString(sum[:8])
}

// handleCollect 记录一次页面访问。公开接口，前端用 sendBeacon 调用。
func (s *Server) handleCollect(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCollectBodyBytes)
	var in collectInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求格式不正确")
		return
	}

	path, ok := normalizeCollectPath(in.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_request", "路径不合法")
		return
	}

	ua := r.UserAgent()
	now := time.Now().In(s.cfg.Location())
	view := store.PageView{
		Path:         path,
		ReferrerHost: normalizeReferrer(in.Referrer),
		Visitor:      visitorHash(middleware.ClientIP(r), ua, now.Format("2006-01-02"), s.cfg.AdminSecret),
		IsBot:        isBotUA(ua),
	}
	if err := s.store.RecordPageView(r.Context(), view); err != nil {
		s.log.Error("记录访问失败", "err", err, "path", path)
		writeError(w, http.StatusInternalServerError, "internal_error", "记录失败")
		return
	}

	// 统计是旁路能力，失败不影响页面；返回 204 让前端不必解析响应。
	w.WriteHeader(http.StatusNoContent)
}

// handleAdminStats 返回最近 N 天的访问汇总，顺带清理过期记录。
func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	days := 7
	if raw := r.URL.Query().Get("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "bad_request", "days 必须是正整数")
			return
		}
		if n > 90 {
			n = 90
		}
		days = n
	}

	if removed, err := s.store.PurgePageViews(r.Context(), pageViewRetention); err != nil {
		s.log.Warn("清理访问记录失败", "err", err)
	} else if removed > 0 {
		s.log.Info("清理访问记录", "removed", removed)
	}

	summary, err := s.store.StatsSummary(r.Context(), days, s.cfg.Location().String())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "读取统计失败")
		return
	}
	writeData(w, summary, nil)
}
