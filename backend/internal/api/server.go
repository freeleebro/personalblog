// Package api 实现博客的 HTTP 接口层。
package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/freeleebro/personalblog/backend/internal/config"
	"github.com/freeleebro/personalblog/backend/internal/middleware"
	"github.com/freeleebro/personalblog/backend/internal/store"
)

// Server 聚合依赖，所有 handler 都挂在它上面。
type Server struct {
	store *store.Store
	cfg   config.Config
	log   *slog.Logger

	// login 记录登录失败次数，用于账号/IP 维度的锁定。
	login loginGuard
	// comments 是访客评论的滑动窗口限流器（同一 IP 10 分钟内最多 5 条）。
	comments commentLimiter
	// cache 是读接口的短 TTL 响应缓存，写操作后整体失效。
	cache responseCache
}

// New 构造一个 Server。
func New(st *store.Store, cfg config.Config, log *slog.Logger) *Server {
	return &Server{store: st, cfg: cfg, log: log}
}

// Handler 组装路由与中间件链。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/site", s.handleSite)

	mux.HandleFunc("GET /api/v1/posts", s.handleListPosts)
	mux.HandleFunc("GET /api/v1/posts/{slug}", s.handleGetPost)
	mux.HandleFunc("GET /api/v1/posts/{slug}/related", s.handleRelatedPosts)
	mux.Handle("POST /api/v1/posts/{slug}/views",
		middleware.RateLimit(30, time.Minute)(http.HandlerFunc(s.handleIncrementViews)))

	// ---- 评论：读公开，写对访客开放（自带限流 + 审核队列）----
	mux.HandleFunc("GET /api/v1/posts/{slug}/comments", s.handleListComments)
	mux.HandleFunc("POST /api/v1/posts/{slug}/comments", s.handleCreateComment)

	mux.HandleFunc("GET /api/v1/tags", s.handleListTags)
	mux.HandleFunc("GET /api/v1/tags/{slug}", s.handleGetTag)

	mux.HandleFunc("GET /api/v1/archive", s.handleArchive)
	mux.HandleFunc("GET /api/v1/pages/{slug}", s.handleGetPage)

	mux.HandleFunc("GET /api/v1/feed.xml", s.handleFeed)
	mux.HandleFunc("GET /api/v1/sitemap.xml", s.handleSitemap)

	// ---- 配图：写要鉴权，读是公开的静态文件 ----
	mux.HandleFunc("GET /media/{path...}", s.handleMedia)

	// ---- 草稿预览：先校验签名，通过后返回任意状态的文章 ----
	mux.HandleFunc("GET /api/v1/preview/{slug}", s.handlePreview)

	// ---- 管理后台：登录（无需鉴权，但强限流 + 失败锁定）----
	mux.Handle("POST /api/v1/admin/login",
		middleware.RateLimit(10, time.Minute)(http.HandlerFunc(s.handleAdminLogin)))

	// ---- 管理后台：受保护接口 ----
	auth := s.requireAuth
	mux.HandleFunc("GET /api/v1/admin/whoami", auth(s.handleAdminWhoami))

	// 改密码：校验旧密码，另外限流，防止拿到 token 的人在这里慢慢试旧密码。
	mux.Handle("POST /api/v1/admin/password",
		middleware.RateLimit(10, time.Minute)(auth(s.handleAdminChangePassword)))

	mux.HandleFunc("GET /api/v1/admin/posts", auth(s.handleAdminListPosts))
	mux.HandleFunc("GET /api/v1/admin/posts/{slug}", auth(s.handleAdminGetPost))
	mux.HandleFunc("POST /api/v1/admin/posts", auth(s.handleAdminSavePost))
	mux.HandleFunc("PUT /api/v1/admin/posts/{slug}", auth(s.handleAdminSavePost))
	mux.HandleFunc("DELETE /api/v1/admin/posts/{slug}", auth(s.handleAdminDeletePost))

	mux.HandleFunc("POST /api/v1/admin/uploads", auth(s.handleAdminUpload))
	mux.HandleFunc("GET /api/v1/admin/preview/{slug}", auth(s.handleAdminPreviewLink))
	mux.HandleFunc("GET /api/v1/admin/export", auth(s.handleAdminExport))

	// 评论审核：列表、通过/隐藏、删除。
	mux.HandleFunc("GET /api/v1/admin/comments", auth(s.handleAdminListComments))
	mux.HandleFunc("PUT /api/v1/admin/comments/{id}", auth(s.handleAdminUpdateComment))
	mux.HandleFunc("DELETE /api/v1/admin/comments/{id}", auth(s.handleAdminDeleteComment))

	mux.HandleFunc("POST /api/v1/admin/tags", auth(s.handleAdminSaveTag))
	mux.HandleFunc("PUT /api/v1/admin/tags/{slug}", auth(s.handleAdminSaveTag))
	mux.HandleFunc("DELETE /api/v1/admin/tags/{slug}", auth(s.handleAdminDeleteTag))

	mux.HandleFunc("POST /api/v1/admin/pages", auth(s.handleAdminSavePage))
	mux.HandleFunc("DELETE /api/v1/admin/pages/{slug}", auth(s.handleAdminDeletePage))

	mux.HandleFunc("PUT /api/v1/admin/site", auth(s.handleAdminSaveSite))

	mux.HandleFunc("/", s.handleNotFound)

	return middleware.Chain(mux,
		middleware.RequestID,
		middleware.Recoverer(s.log),
		middleware.AccessLog(s.log),
		middleware.CORS(s.cfg.AllowedHosts, !s.cfg.IsProduction()),
		s.cacheMiddleware,
	)
}

func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
		return
	}
	http.Error(w, "404 page not found", http.StatusNotFound)
}

// fail 记录内部错误，对外只返回通用信息，避免泄露实现细节。
func (s *Server) fail(w http.ResponseWriter, message string, err error) {
	s.log.Error(message, "error", err)
	writeError(w, http.StatusInternalServerError, "internal", "服务内部错误")
}
