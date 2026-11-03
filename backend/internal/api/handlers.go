package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/freeleebro/personalblog/backend/internal/domain"
	"github.com/freeleebro/personalblog/backend/internal/store"
)

// ---------------------------------------------------------------- 基础

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		s.log.Error("健康检查失败", "error", err)
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "数据库不可用")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"env":    s.cfg.Env,
		"time":   time.Now().UTC(),
	})
}

// ---------------------------------------------------------------- 站点

type siteSettings struct {
	Name        string          `json:"name"`
	Tagline     string          `json:"tagline"`
	Description string          `json:"description"`
	URL         string          `json:"url"`
	Socials     []domain.Social `json:"socials"`

	// Texts 是页面上那些「编辑想改、但不值得单独建表」的短文案。
	// 用扁平的 key/value 存，新增一句文案只要前后端各加一个 key，
	// 不用改数据库结构。键名见 web/lib/site-text.ts。
	Texts map[string]string `json:"texts"`
}

// handleSite 一次返回首屏所需的全部站点级数据，减少前端瀑布式请求。
func (s *Server) handleSite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cfg := siteSettings{
		Name:        s.cfg.SiteName,
		Tagline:     s.cfg.SiteTagline,
		Description: s.cfg.SiteDescription,
		URL:         s.cfg.SiteURL,
	}
	if err := s.store.Setting(ctx, "site", &cfg); err != nil && !errors.Is(err, store.ErrNotFound) {
		s.fail(w, "读取站点设置失败", err)
		return
	}
	if cfg.Name == "" {
		cfg.Name = s.cfg.SiteName
	}
	if cfg.Texts == nil {
		cfg.Texts = map[string]string{}
	}

	authorSlug := s.cfg.AuthorSlug
	if err := s.store.Setting(ctx, "author", &authorSlug); err != nil && !errors.Is(err, store.ErrNotFound) {
		s.fail(w, "读取作者设置失败", err)
		return
	}
	author, err := s.store.GetAuthorBySlug(ctx, authorSlug)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.fail(w, "读取作者失败", err)
		return
	}

	stats, err := s.store.Stats(ctx)
	if err != nil {
		s.fail(w, "读取站点统计失败", err)
		return
	}

	tags, err := s.store.ListTags(ctx)
	if err != nil {
		s.fail(w, "读取标签失败", err)
		return
	}

	recent, err := s.store.ListPosts(ctx, store.PostFilter{PerPage: 5})
	if err != nil {
		s.fail(w, "读取最新文章失败", err)
		return
	}
	for i := range recent.Items {
		recent.Items[i].BodyMD = ""
	}

	featuredOnly := true
	var featured *domain.Post
	featuredPage, err := s.store.ListPosts(ctx, store.PostFilter{Featured: &featuredOnly, PerPage: 1})
	if err != nil {
		s.fail(w, "读取推荐文章失败", err)
		return
	}
	if len(featuredPage.Items) > 0 {
		featuredPage.Items[0].BodyMD = ""
		featured = &featuredPage.Items[0]
	}

	writeData(w, domain.Site{
		Name:        cfg.Name,
		Tagline:     cfg.Tagline,
		Description: cfg.Description,
		URL:         cfg.URL,
		Author:      author,
		Socials:     cfg.Socials,
		Texts:       cfg.Texts,
		Stats:       stats,
		Tags:        tags,
		Featured:    featured,
		Recent:      recent.Items,
	}, nil)
}

// ---------------------------------------------------------------- 文章

func (s *Server) handleListPosts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := store.PostFilter{
		TagSlug: strings.TrimSpace(q.Get("tag")),
		Query:   strings.TrimSpace(q.Get("q")),
		Year:    intOrDefault(q.Get("year"), 0),
		Page:    intOrDefault(q.Get("page"), 1),
		PerPage: intOrDefault(q.Get("per_page"), 10),
	}
	if v := strings.TrimSpace(q.Get("featured")); v != "" {
		only := v == "true" || v == "1"
		filter.Featured = &only
	}

	page, err := s.store.ListPosts(r.Context(), filter)
	if err != nil {
		s.fail(w, "查询文章列表失败", err)
		return
	}
	for i := range page.Items {
		page.Items[i].BodyMD = ""
	}

	writeData(w, page.Items, &Meta{
		Page:       page.Page,
		PerPage:    page.PerPage,
		Total:      page.Total,
		TotalPages: page.TotalPages(),
	})
}

type postDetail struct {
	Post domain.Post  `json:"post"`
	Prev *domain.Post `json:"prev"`
	Next *domain.Post `json:"next"`
}

func (s *Server) handleGetPost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	ctx := r.Context()

	post, err := s.store.GetPostBySlug(ctx, slug)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "post_not_found", "文章不存在")
		return
	}
	if err != nil {
		s.fail(w, "查询文章失败", err)
		return
	}

	detail := postDetail{Post: *post}
	if post.PublishedAt != nil {
		prev, next, err := s.store.Neighbours(ctx, *post.PublishedAt, post.ID)
		if err != nil {
			s.fail(w, "查询相邻文章失败", err)
			return
		}
		detail.Prev = trimBody(prev)
		detail.Next = trimBody(next)
	}

	writeData(w, detail, nil)
}

// trimBody 清空正文。
// 列表、上下篇导航、相关文章只需要标题与 slug，带上 Markdown 全文会让
// 响应体膨胀几十倍——一篇 1.6KB 的文章，正文占了详情报文的 90% 以上。
func trimBody(p *domain.Post) *domain.Post {
	if p == nil {
		return nil
	}
	p.BodyMD = ""
	return p
}

func (s *Server) handleRelatedPosts(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	ctx := r.Context()

	post, err := s.store.GetPostBySlug(ctx, slug)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "post_not_found", "文章不存在")
		return
	}
	if err != nil {
		s.fail(w, "查询文章失败", err)
		return
	}

	limit := intOrDefault(r.URL.Query().Get("limit"), 3)
	if limit > 6 {
		limit = 6
	}

	items, err := s.store.RelatedPosts(ctx, post.ID, limit)
	if err != nil {
		s.fail(w, "查询相关文章失败", err)
		return
	}
	for i := range items {
		items[i].BodyMD = ""
	}
	writeData(w, items, nil)
}

func (s *Server) handleIncrementViews(w http.ResponseWriter, r *http.Request) {
	views, err := s.store.IncrementViews(r.Context(), r.PathValue("slug"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "post_not_found", "文章不存在")
		return
	}
	if err != nil {
		s.fail(w, "更新阅读量失败", err)
		return
	}
	// 写接口不做缓存
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, map[string]int{"views": views}, nil)
}

// ---------------------------------------------------------------- 标签

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.store.ListTags(r.Context())
	if err != nil {
		s.fail(w, "查询标签失败", err)
		return
	}
	writeData(w, tags, nil)
}

type tagDetail struct {
	Tag   domain.Tag    `json:"tag"`
	Posts []domain.Post `json:"posts"`
}

func (s *Server) handleGetTag(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	ctx := r.Context()

	tag, err := s.store.GetTagBySlug(ctx, slug)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "tag_not_found", "标签不存在")
		return
	}
	if err != nil {
		s.fail(w, "查询标签失败", err)
		return
	}

	posts, err := s.store.ListPosts(ctx, store.PostFilter{TagSlug: slug, PerPage: 50})
	if err != nil {
		s.fail(w, "查询标签下文章失败", err)
		return
	}
	for i := range posts.Items {
		posts.Items[i].BodyMD = ""
	}

	writeData(w, tagDetail{Tag: *tag, Posts: posts.Items}, nil)
}

// ---------------------------------------------------------------- 归档 / 页面

func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	years, err := s.store.PostCountByYear(r.Context())
	if err != nil {
		s.fail(w, "查询归档失败", err)
		return
	}
	writeData(w, years, nil)
}

func (s *Server) handleGetPage(w http.ResponseWriter, r *http.Request) {
	page, err := s.store.GetPage(r.Context(), r.PathValue("slug"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "page_not_found", "页面不存在")
		return
	}
	if err != nil {
		s.fail(w, "查询页面失败", err)
		return
	}
	writeData(w, page, nil)
}

// ---------------------------------------------------------------- 工具

func intOrDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
