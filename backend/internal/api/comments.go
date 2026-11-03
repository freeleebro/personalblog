package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/freeleebro/personalblog/backend/internal/domain"
	"github.com/freeleebro/personalblog/backend/internal/middleware"
	"github.com/freeleebro/personalblog/backend/internal/store"
)

const (
	// 评论字段上限。按 rune 计，避免中文被按字节数误判超长。
	maxCommentAuthorRunes = 40
	maxCommentBodyRunes   = 2000
	maxCommentEmailRunes  = 120
	maxCommentListLimit   = 200

	// 同一 IP 在窗口内最多提交几条，挡脚本刷屏。
	commentRateLimit  = 5
	commentRateWindow = 10 * time.Minute

	// 评论请求体上限：正文 2000 字（中文 6KB）+ JSON 外壳，8KB 够用。
	maxCommentBodyBytes = 8 << 10
)

// commentLimiter 是进程内的简单滑动窗口限流器。
// 博客只有一个后端进程，不需要 Redis；重启丢失计数可以接受。
type commentLimiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

// allow 记录一次提交并返回是否放行。
func (l *commentLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.hits == nil {
		l.hits = make(map[string][]time.Time)
	}

	cutoff := now.Add(-commentRateWindow)
	recent := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= commentRateLimit {
		l.hits[key] = recent
		return false
	}
	l.hits[key] = append(recent, now)

	// 顺手清掉整体已过期的 key，避免 map 无限增长。
	if len(l.hits) > 10_000 {
		for k, v := range l.hits {
			if len(v) == 0 || v[len(v)-1].Before(cutoff) {
				delete(l.hits, k)
			}
		}
	}
	return true
}

// commentInput 是公开接口的提交结构。Website 是蜜罐字段：
// 真人看不到它，机器人往往会把所有 input 都填上。
type commentInput struct {
	Author  string `json:"author"`
	Email   string `json:"email"`
	Body    string `json:"body"`
	Website string `json:"website"`
}

// normalizeComment 校验并规范化评论内容。
// 返回的错误信息会直接展示给访客，所以用中文、说清哪里不对。
func normalizeComment(in commentInput) (author, email, body string, err error) {
	author = strings.TrimSpace(in.Author)
	if n := utf8.RuneCountInString(author); n == 0 {
		return "", "", "", errors.New("请填写昵称")
	} else if n > maxCommentAuthorRunes {
		return "", "", "", errors.New("昵称太长（最多 40 字）")
	}

	email = strings.TrimSpace(in.Email)
	if utf8.RuneCountInString(email) > maxCommentEmailRunes {
		return "", "", "", errors.New("邮箱太长")
	}
	if email != "" && (!strings.Contains(email, "@") || strings.ContainsAny(email, " \t\r\n")) {
		return "", "", "", errors.New("邮箱格式看起来不对")
	}

	body = strings.ReplaceAll(in.Body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	body = strings.TrimSpace(body)
	if n := utf8.RuneCountInString(body); n < 2 {
		return "", "", "", errors.New("评论内容太短")
	} else if n > maxCommentBodyRunes {
		return "", "", "", errors.New("评论太长（最多 2000 字）")
	}

	return author, email, body, nil
}

// hashCommentIP 把来源 IP 和密钥混在一起后截断保存：
// 后台能看出「这几条来自同一来源」，但拿到数据库也反推不出 IP。
func hashCommentIP(ip, secret string) string {
	sum := sha256.Sum256([]byte(ip + "|" + secret))
	return hex.EncodeToString(sum[:])[:16]
}

// commentListResponse 是公开评论列表的响应。
type commentListResponse struct {
	Enabled  bool             `json:"enabled"`
	Comments []domain.Comment `json:"comments"`
	Total    int              `json:"total"`
}

// parseID 解析路径里的正整数 id。
func parseID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// handleListComments 返回某篇文章已通过的评论。
func (s *Server) handleListComments(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	if !s.cfg.CommentsEnabled {
		writeData(w, commentListResponse{Enabled: false, Comments: []domain.Comment{}}, nil)
		return
	}

	// 只有公开可见的文章才谈得上评论：草稿、定时未发布的一律 404。
	if _, err := s.store.GetPostBySlug(r.Context(), slug); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "post_not_found", "文章不存在")
			return
		}
		s.fail(w, "查询文章失败", err)
		return
	}

	comments, err := s.store.ListPublishedComments(r.Context(), slug, maxCommentListLimit)
	if err != nil {
		s.fail(w, "查询评论失败", err)
		return
	}
	total, err := s.store.CountPublishedComments(r.Context(), slug)
	if err != nil {
		s.fail(w, "统计评论失败", err)
		return
	}

	// 评论里不该出现邮箱，即使被误存了也不外泄。
	for i := range comments {
		comments[i].Email = ""
	}
	writeData(w, commentListResponse{Enabled: true, Comments: comments, Total: total}, nil)
}

// handleCreateComment 接收访客评论。默认进审核队列，站长通过后才公开。
func (s *Server) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	if !s.cfg.CommentsEnabled {
		writeError(w, http.StatusForbidden, "comments_disabled", "评论功能未开放")
		return
	}

	ip := middleware.ClientIP(r)
	if !s.comments.allow(ip, time.Now()) {
		writeError(w, http.StatusTooManyRequests, "too_many_requests", "评论太频繁了，请稍后再试")
		return
	}

	var in commentInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCommentBodyBytes))
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求体非法")
		return
	}

	// 蜜罐命中：假装成功，什么都不存，让机器人以为发出来了。
	if strings.TrimSpace(in.Website) != "" {
		writeData(w, map[string]string{"status": "received"}, nil)
		return
	}

	author, email, body, err := normalizeComment(in)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_comment", err.Error())
		return
	}

	if _, err := s.store.GetPostBySlug(r.Context(), slug); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "post_not_found", "文章不存在")
			return
		}
		s.fail(w, "查询文章失败", err)
		return
	}

	status := domain.CommentPending
	if s.cfg.CommentAutoApprove {
		status = domain.CommentPublished
	}
	comment := domain.Comment{
		PostSlug: slug,
		Author:   author,
		Email:    email,
		Body:     body,
		Status:   status,
	}
	if err := s.store.CreateComment(r.Context(), &comment, hashCommentIP(ip, s.cfg.AdminSecret)); err != nil {
		s.fail(w, "保存评论失败", err)
		return
	}

	comment.Email = ""
	writeData(w, map[string]any{
		"status":  status,
		"comment": comment,
	}, nil)
}

// handleAdminListComments 返回评论列表与各状态数量，供后台审核。
func (s *Server) handleAdminListComments(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	switch status {
	case "", domain.CommentPending, domain.CommentPublished, domain.CommentHidden:
	default:
		writeError(w, http.StatusBadRequest, "bad_request", "status 只能是 pending / published / hidden")
		return
	}

	limit := maxCommentListLimit
	comments, err := s.store.AdminListComments(r.Context(), status, limit)
	if err != nil {
		s.fail(w, "查询评论失败", err)
		return
	}
	counts, err := s.store.CommentCounts(r.Context())
	if err != nil {
		s.fail(w, "统计评论失败", err)
		return
	}
	writeData(w, map[string]any{"comments": comments, "counts": counts}, nil)
}

// handleAdminUpdateComment 通过 / 隐藏 / 重新待审一条评论。
func (s *Server) handleAdminUpdateComment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_request", "评论 id 非法")
		return
	}

	var in struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求体非法")
		return
	}
	switch in.Status {
	case domain.CommentPending, domain.CommentPublished, domain.CommentHidden:
	default:
		writeError(w, http.StatusBadRequest, "bad_request", "status 只能是 pending / published / hidden")
		return
	}

	found, err := s.store.SetCommentStatus(r.Context(), id, in.Status)
	if err != nil {
		s.fail(w, "更新评论状态失败", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "comment_not_found", "评论不存在")
		return
	}
	writeData(w, map[string]any{"id": id, "status": in.Status}, nil)
}

// handleAdminDeleteComment 删除一条评论。
func (s *Server) handleAdminDeleteComment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "bad_request", "评论 id 非法")
		return
	}

	found, err := s.store.DeleteComment(r.Context(), id)
	if err != nil {
		s.fail(w, "删除评论失败", err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "comment_not_found", "评论不存在")
		return
	}
	writeData(w, map[string]any{"deleted": true}, nil)
}
