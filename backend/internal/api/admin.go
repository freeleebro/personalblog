// 管理后台：登录 + 受保护的写接口。
// 所有写操作都要求携带有效的 Bearer token，token 由 /admin/login 签发。
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/freeleebro/personalblog/backend/internal/auth"
	"github.com/freeleebro/personalblog/backend/internal/domain"
	"github.com/freeleebro/personalblog/backend/internal/markdown"
	"github.com/freeleebro/personalblog/backend/internal/middleware"
	"github.com/freeleebro/personalblog/backend/internal/store"
)

// tokenTTL 是登录 token 的有效期。
const tokenTTL = 7 * 24 * time.Hour

// ---------------------------------------------------------------- 登录

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleAdminLogin 校验用户名密码并签发 token。
func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求体非法")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "用户名和密码不能为空")
		return
	}

	// 同一 IP + 用户名连续失败会被锁定，避免公网上的字典爆破。
	guardKey := middleware.ClientIP(r) + "|" + req.Username
	if wait := s.login.blockedFor(guardKey); wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		w.Header().Set("Cache-Control", "no-store")
		writeError(w, http.StatusTooManyRequests, "too_many_attempts", "登录尝试过于频繁，请稍后再试")
		return
	}

	user, err := s.store.GetAdminUserByUsername(r.Context(), req.Username)
	if errors.Is(err, store.ErrNotFound) {
		// 统一返回「用户名或密码错误」，不泄露账号是否存在。
		s.login.fail(guardKey)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}
	if err != nil {
		s.fail(w, "查询管理员失败", err)
		return
	}

	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		s.login.fail(guardKey)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "用户名或密码错误")
		return
	}
	s.login.reset(guardKey)

	token, err := auth.SignToken(s.cfg.AdminSecret, user.Username, tokenTTL)
	if err != nil {
		s.fail(w, "签发 token 失败", err)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	writeData(w, map[string]any{
		"token":       token,
		"username":    user.Username,
		"displayName": user.DisplayName,
	}, nil)
}

// handleAdminWhoami 返回当前登录账号信息，供前端校验 token 是否有效。
func (s *Server) handleAdminWhoami(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(ctxAdminUser{}).(string)
	user, err := s.store.GetAdminUserByUsername(r.Context(), username)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "invalid_token", "账号不存在")
		return
	}
	if err != nil {
		s.fail(w, "查询管理员失败", err)
		return
	}
	writeData(w, map[string]any{
		"username":    user.Username,
		"displayName": user.DisplayName,
	}, nil)
}

// ---------------------------------------------------------------- 认证中间件

type ctxAdminUser struct{}

// requireAuth 校验 Bearer token，通过后把用户名写入 context。
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		if !strings.HasPrefix(authz, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "unauthorized", "缺少登录凭证")
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
		username, err := auth.VerifyToken(s.cfg.AdminSecret, token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_token", "登录已过期，请重新登录")
			return
		}
		next(w, r.WithContext(contextWithAdminUser(r, username)))
	}
}

func contextWithAdminUser(r *http.Request, username string) context.Context {
	return context.WithValue(r.Context(), ctxAdminUser{}, username)
}

// ---------------------------------------------------------------- 文章管理

// adminPostInput 是管理后台写文章时的入参。
type adminPostInput struct {
	Slug       string  `json:"slug"`
	Title      string  `json:"title"`
	Summary    string  `json:"summary"`
	BodyMD     string  `json:"body_md"`
	CoverColor string  `json:"cover_color"`
	CoverURL   string  `json:"cover_url"`
	Status     string  `json:"status"`
	Featured   bool    `json:"featured"`
	TagIDs     []int64 `json:"tag_ids"`
	Published  *string `json:"published_at"` // RFC3339 字符串，空则用当前时间
}

func (in *adminPostInput) toStore(authorID *int64) (store.PostInput, error) {
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "published"
	}
	if status != "draft" && status != "published" && status != "archived" {
		return store.PostInput{}, errors.New("status 必须是 draft / published / archived")
	}

	var publishedAt *time.Time
	if in.Published != nil && strings.TrimSpace(*in.Published) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*in.Published))
		if err != nil {
			return store.PostInput{}, errors.New("published_at 需为 RFC3339 时间")
		}
		publishedAt = &t
	}

	coverURL := strings.TrimSpace(in.CoverURL)
	if coverURL != "" && !strings.HasPrefix(coverURL, "/") && !strings.HasPrefix(coverURL, "https://") {
		return store.PostInput{}, errors.New("cover_url 只能是站内路径（/media/...）或 https 链接")
	}

	return store.PostInput{
		Slug:           strings.TrimSpace(in.Slug),
		Title:          strings.TrimSpace(in.Title),
		Summary:        strings.TrimSpace(in.Summary),
		BodyMD:         in.BodyMD,
		CoverColor:     strings.TrimSpace(in.CoverColor),
		CoverURL:       coverURL,
		Status:         status,
		Featured:       in.Featured,
		ReadingMinutes: markdown.ReadingMinutes(in.BodyMD),
		AuthorID:       authorID,
		TagIDs:         in.TagIDs,
		PublishedAt:    publishedAt,
	}, nil
}

// handleAdminListPosts 返回全部文章（含草稿/归档），供后台列表用。
func (s *Server) handleAdminListPosts(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	filter := store.PostFilter{PerPage: 100}
	if status != "" {
		filter.Status = status
	} else {
		// 后台默认返回所有状态；PostFilter 零值会退化成 published，
		// 所以这里用特殊值标记「不过滤」，由下方 SQL 处理。
		filter.Status = "*"
	}
	page, err := s.store.ListPostsAnyStatus(r.Context(), filter)
	if err != nil {
		s.fail(w, "查询文章失败", err)
		return
	}
	writeData(w, page.Items, nil)
}

// handleAdminGetPost 返回任意状态的文章（含正文与标签），供后台编辑器回填。
func (s *Server) handleAdminGetPost(w http.ResponseWriter, r *http.Request) {
	post, err := s.store.GetPostBySlugAnyStatus(r.Context(), r.PathValue("slug"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "post_not_found", "文章不存在")
		return
	}
	if err != nil {
		s.fail(w, "查询文章失败", err)
		return
	}
	writeData(w, post, nil)
}

// handleAdminSavePost 创建或更新一篇文章。
func (s *Server) handleAdminSavePost(w http.ResponseWriter, r *http.Request) {
	var in adminPostInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求体非法")
		return
	}
	if strings.TrimSpace(in.Title) == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "标题不能为空")
		return
	}
	if strings.TrimSpace(in.Slug) == "" {
		in.Slug = postSlug(in.Title, time.Now().In(s.cfg.Location()))
	}

	authorID, err := s.defaultAuthorID(r)
	if err != nil {
		s.fail(w, "查询作者失败", err)
		return
	}

	post, err := in.toStore(authorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	id, err := s.store.UpsertPost(r.Context(), post)
	if err != nil {
		s.fail(w, "保存文章失败", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, map[string]any{"id": id, "slug": post.Slug}, nil)
}

// handleAdminDeletePost 删除一篇文章。
func (s *Server) handleAdminDeletePost(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeletePost(r.Context(), r.PathValue("slug")); err != nil {
		s.fail(w, "删除文章失败", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, map[string]any{"deleted": true}, nil)
}

// defaultAuthorID 取「作者」设置里配置的作者 ID，作为写文章时的默认作者。
func (s *Server) defaultAuthorID(r *http.Request) (*int64, error) {
	var authorSlug string
	if err := s.store.Setting(r.Context(), "author", &authorSlug); err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if authorSlug == "" {
		authorSlug = s.cfg.AuthorSlug
	}
	author, err := s.store.GetAuthorBySlug(r.Context(), authorSlug)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil // 没有作者也行，author_id 留空
	}
	if err != nil {
		return nil, err
	}
	return &author.ID, nil
}

// ---------------------------------------------------------------- 标签管理

type adminTagInput struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// handleAdminSaveTag 创建或更新标签。
func (s *Server) handleAdminSaveTag(w http.ResponseWriter, r *http.Request) {
	var in adminTagInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求体非法")
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "标签名不能为空")
		return
	}
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		slug = tagSlug(name)
	}
	if strings.TrimSpace(in.Color) == "" {
		in.Color = "#8b5cf6"
	}

	id, err := s.store.UpsertTag(r.Context(), store.TagInput{
		Slug:        slug,
		Name:        name,
		Color:       in.Color,
		Description: in.Description,
	})
	if err != nil {
		s.fail(w, "保存标签失败", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, map[string]any{"id": id, "slug": slug}, nil)
}

// handleAdminDeleteTag 删除标签（关联的文章标签会级联清除）。
func (s *Server) handleAdminDeleteTag(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteTag(r.Context(), r.PathValue("slug")); err != nil {
		s.fail(w, "删除标签失败", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, map[string]any{"deleted": true}, nil)
}

// ---------------------------------------------------------------- 页面管理

type adminPageInput struct {
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	BodyMD string `json:"body_md"`
}

// handleAdminSavePage 创建或更新独立页面（如 about）。
func (s *Server) handleAdminSavePage(w http.ResponseWriter, r *http.Request) {
	var in adminPageInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求体非法")
		return
	}
	slug := strings.TrimSpace(in.Slug)
	title := strings.TrimSpace(in.Title)
	if slug == "" || title == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "slug 和标题不能为空")
		return
	}
	if err := s.store.UpsertPage(r.Context(), slug, title, in.BodyMD); err != nil {
		s.fail(w, "保存页面失败", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, map[string]any{"slug": slug}, nil)
}

// handleAdminDeletePage 删除独立页面。
func (s *Server) handleAdminDeletePage(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeletePage(r.Context(), r.PathValue("slug")); err != nil {
		s.fail(w, "删除页面失败", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, map[string]any{"deleted": true}, nil)
}

// ---------------------------------------------------------------- 站点设置

type adminSiteInput struct {
	Name        string            `json:"name"`
	Tagline     string            `json:"tagline"`
	Description string            `json:"description"`
	URL         string            `json:"url"`
	Socials     []domain.Social   `json:"socials"`
	Texts       map[string]string `json:"texts"`
}

// handleAdminSaveSite 更新站点级设置。
func (s *Server) handleAdminSaveSite(w http.ResponseWriter, r *http.Request) {
	var in adminSiteInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求体非法")
		return
	}
	socials := in.Socials
	if socials == nil {
		socials = []domain.Social{}
	}
	if err := s.store.PutSetting(r.Context(), "site", map[string]any{
		"name":        strings.TrimSpace(in.Name),
		"tagline":     strings.TrimSpace(in.Tagline),
		"description": strings.TrimSpace(in.Description),
		"url":         strings.TrimSpace(in.URL),
		"socials":     socials,
		"texts":       sanitizeTexts(in.Texts),
	}); err != nil {
		s.fail(w, "保存站点设置失败", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, map[string]any{"saved": true}, nil)
}

// ---------------------------------------------------------------- 工具

// sanitizeTexts 收敛前端提交的短文案：只留合法键名、去空白、限长、丢空值。
// 这些字符串会直接渲染到页面上，所以宁可在这里砍掉，也别存进数据库。
func sanitizeTexts(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		if len(out) >= maxSiteTexts {
			break
		}
		k := strings.TrimSpace(key)
		if k == "" || len(k) > 64 {
			continue
		}
		valid := true
		for _, r := range k {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			default:
				valid = false
			}
		}
		if !valid {
			continue
		}
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		if runes := []rune(v); len(runes) > maxSiteTextLen {
			v = string(runes[:maxSiteTextLen])
		}
		out[k] = v
	}
	return out
}

const (
	maxSiteTexts   = 64
	maxSiteTextLen = 500
)

// maxSlugLen 是自动生成 slug 的最大长度（字符数）。
const maxSlugLen = 60

// postSlug 把标题转成 URL 友好的 slug。
// 纯 ASCII 标题压缩成 a-z0-9-；含中文等非 ASCII 字符时改用
// post-YYYYMMDD-<6 位十六进制>，避免公开 URL 里出现 %E4%B8%AD 这种长转义。
func postSlug(title string, now time.Time) string {
	if s := asciiSlug(title); s != "" {
		return s
	}
	sum := fnv.New32a()
	_, _ = sum.Write([]byte(title))
	return fmt.Sprintf("post-%s-%06x", now.Format("20060102"), sum.Sum32()&0xffffff)
}

// asciiSlug 只保留 a-z0-9，其余字符折叠成单个连字符。
// 含非 ASCII 字符（中文、日文、全角标点等）时返回空串，交给调用方兜底。
func asciiSlug(name string) string {
	var b strings.Builder
	pendingDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if pendingDash && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingDash = false
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			if pendingDash && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingDash = false
			b.WriteRune(r + ('a' - 'A'))
		case r > 127:
			return ""
		default:
			// 空格、标点、符号都视作分隔符，连续多个折叠成一个连字符。
			if b.Len() > 0 {
				pendingDash = true
			}
		}
	}
	out := b.String()
	if utf8.RuneCountInString(out) > maxSlugLen {
		runes := []rune(out)
		out = strings.TrimRight(string(runes[:maxSlugLen]), "-")
	}
	return out
}

// tagSlug 保留原有行为：中文标签直接作为 slug，便于 /tags/中文 这类可读 URL。
// 标签数量少、URL 短，这里不追求 ASCII 化。
func tagSlug(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		case r == ' ' || r == '/' || r == '\\' || r == '#' || r == '?' || r == '.':
			out = append(out, '-')
		case r > 127:
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return "untitled"
	}
	return string(out)
}
