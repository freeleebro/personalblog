package api

import (
	"bytes"
	"net/http"
	"strings"
	"sync"
	"time"
)

// responseCacheTTL 是读接口的进程内缓存时长。
//
// 为什么需要它：Next.js 侧对页面做了 force-dynamic（构建镜像时后端并不存在，
// 静态化会把「后端不可用」烤进产物），所以每次访问都会回源到 Go。
// 首页一次要打 7 个查询、文章页 4 个，人一多就在数据库上排队。
// 这里把 GET /api/v1/* 的响应缓存 15 秒，写操作后立即整体失效，
// 于是「后台改完立刻生效」和「正常访问不打库」可以同时成立。
const responseCacheTTL = 15 * time.Second

// responseCacheMaxBody 限制可缓存的响应体大小，避免把大响应也塞进内存。
const responseCacheMaxBody = 2 << 20 // 2 MiB

type cacheEntry struct {
	status  int
	header  http.Header
	body    []byte
	expires time.Time
}

// responseCache 是一个带 TTL 的极简响应缓存。
type responseCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

func (c *responseCache) get(key string, now time.Time) (cacheEntry, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || now.After(e.expires) {
		return cacheEntry{}, false
	}
	return e, true
}

func (c *responseCache) set(key string, e cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]cacheEntry)
	}
	// 简单的容量保护：条目过多时整体清空，个人博客量级不会频繁触发。
	if len(c.entries) > 2_000 {
		c.entries = make(map[string]cacheEntry)
	}
	c.entries[key] = e
}

func (c *responseCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = nil
}

// cacheWriter 记录 handler 写出的状态码、响应头与响应体。
type cacheWriter struct {
	header http.Header
	status int
	buf    bytes.Buffer
}

func (w *cacheWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *cacheWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
}

func (w *cacheWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.buf.Write(p)
}

// cacheableRequest 判断请求是否走缓存：
// 只缓存匿名 GET，后台接口与健康检查永远直连。
func cacheableRequest(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	if r.Header.Get("Authorization") != "" {
		return false
	}
	if cc := r.Header.Get("Cache-Control"); strings.Contains(cc, "no-cache") || strings.Contains(cc, "no-store") {
		return false
	}
	p := r.URL.Path
	if !strings.HasPrefix(p, "/api/v1/") {
		return false
	}
	if strings.HasPrefix(p, "/api/v1/admin/") || p == "/api/v1/health" {
		return false
	}
	// 草稿预览是「刚保存就想看效果」的场景，缓存 15 秒只会让人怀疑没保存上。
	if strings.HasPrefix(p, "/api/v1/preview/") {
		return false
	}
	return true
}

// isAdminMutation 判断是否为后台写操作，用于写后清空缓存。
func isAdminMutation(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return strings.HasPrefix(r.URL.Path, "/api/v1/admin/") ||
			strings.HasSuffix(r.URL.Path, "/views")
	}
	return false
}

func (s *Server) cacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 写操作先放行，成功后整体失效——这样后台保存完刷新页面就是新内容。
		if isAdminMutation(r) {
			next.ServeHTTP(w, r)
			s.cache.clear()
			return
		}
		if !cacheableRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Method + " " + r.URL.RequestURI()
		now := time.Now()
		if e, ok := s.cache.get(key, now); ok {
			copyHeader(w.Header(), e.header)
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(e.status)
			_, _ = w.Write(e.body)
			return
		}

		rec := &cacheWriter{}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}

		copyHeader(w.Header(), rec.header)
		w.Header().Set("X-Cache", "MISS")
		w.WriteHeader(rec.status)
		body := rec.buf.Bytes()
		_, _ = w.Write(body)

		if rec.status == http.StatusOK && len(body) <= responseCacheMaxBody {
			stored := make(http.Header, len(rec.header))
			copyHeader(stored, rec.header)
			stored.Del("X-Cache")
			s.cache.set(key, cacheEntry{
				status:  rec.status,
				header:  stored,
				body:    append([]byte(nil), body...),
				expires: now.Add(responseCacheTTL),
			})
		}
	})
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
