// Package middleware 提供一组零依赖的 HTTP 中间件。
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// Chain 按声明顺序把中间件套在 handler 外层（第一个最先执行）。
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// RequestID 为每个请求生成或透传 X-Request-Id，便于跨服务排查。
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = randomID()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// RequestIDFrom 从 context 取出请求 ID。
func RequestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

func randomID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// AccessLog 以结构化日志记录每个请求。
func AccessLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			log.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"query", r.URL.RawQuery,
				"status", rec.status,
				"bytes", rec.bytes,
				"duration_ms", time.Since(start).Milliseconds(),
				"ip", ClientIP(r),
				"request_id", RequestIDFrom(r.Context()),
			)
		})
	}
}

// Recoverer 兜住 panic，避免单个请求打挂整个进程。
func Recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered",
						"error", rec,
						"path", r.URL.Path,
						"request_id", RequestIDFrom(r.Context()),
					)
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"服务内部错误"}}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// CORS 按白名单放行跨域请求。allowAll 为 true 时回显任意来源（仅建议开发环境使用）。
func CORS(allowed []string, allowAll bool) func(http.Handler) http.Handler {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		allowedSet[strings.ToLower(strings.TrimSpace(o))] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				_, ok := allowedSet[strings.ToLower(origin)]
				switch {
				case allowAll:
					w.Header().Set("Access-Control-Allow-Origin", "*")
				case ok:
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if allowAll || ok {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id")
					w.Header().Set("Access-Control-Max-Age", "600")
				}
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit 是按客户端 IP 的令牌桶限流，用来保护写接口。
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	if limit <= 0 || window <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	rate := float64(limit) / window.Seconds()

	type bucket struct {
		tokens float64
		last   time.Time
	}

	var (
		mu      sync.Mutex
		buckets = make(map[string]*bucket)
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIP(r)
			now := time.Now()

			mu.Lock()
			b, ok := buckets[ip]
			if !ok {
				b = &bucket{tokens: float64(limit), last: now}
				buckets[ip] = b
			}
			elapsed := now.Sub(b.last).Seconds()
			b.tokens = min(float64(limit), b.tokens+elapsed*rate)
			b.last = now

			allowed := b.tokens >= 1
			if allowed {
				b.tokens--
			}
			if len(buckets) > 10_000 {
				for k, v := range buckets {
					if now.Sub(v.last) > 10*window {
						delete(buckets, k)
					}
				}
			}
			mu.Unlock()

			if !allowed {
				w.Header().Set("Retry-After", "60")
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"请求过于频繁，请稍后再试"}}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP 返回请求的真实来源 IP。
//
// X-Forwarded-For / X-Real-IP 只有在本请求确实来自可信代理（默认是回环与
// 内网网段，可用 TRUSTED_PROXIES 覆盖）时才会被采信，否则任何人都能伪造
// 一个头绕过限流、污染日志。
func ClientIP(r *http.Request) string {
	host := remoteHost(r)
	ip := net.ParseIP(host)
	if ip == nil || !isTrustedProxy(ip) {
		return host
	}

	// 从右往左找第一个「不可信」地址：右端是代理追加的真实来源，
	// 左端可能是客户端自己伪造的。
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			p := net.ParseIP(strings.TrimSpace(parts[i]))
			if p == nil {
				continue
			}
			if !isTrustedProxy(p) {
				return p.String()
			}
		}
	}
	if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); real != "" {
		if p := net.ParseIP(real); p != nil && !isTrustedProxy(p) {
			return p.String()
		}
	}
	return host
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// trustedProxies 保存允许提供转发头的代理网段。
var trustedProxies atomic.Pointer[[]*net.IPNet]

// DefaultTrustedProxies 是默认的可信代理网段：回环 + RFC1918 内网 + IPv6 ULA。
// 单容器 / compose 部署时，反向代理总是从这些网段连进来。
func DefaultTrustedProxies() []string {
	return []string{
		"127.0.0.0/8",
		"::1/128",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7",
	}
}

// SetTrustedProxies 设置可信代理网段。传空切片表示使用默认值。
// 支持 CIDR 与裸 IP 两种写法。
func SetTrustedProxies(cidrs []string) error {
	if len(cidrs) == 0 {
		cidrs = DefaultTrustedProxies()
	}
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, raw := range cidrs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.Contains(raw, "/") {
			ip := net.ParseIP(raw)
			if ip == nil {
				return fmt.Errorf("TRUSTED_PROXIES 含无效地址 %q", raw)
			}
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			raw = fmt.Sprintf("%s/%d", ip.String(), bits)
		}
		_, n, err := net.ParseCIDR(raw)
		if err != nil {
			return fmt.Errorf("TRUSTED_PROXIES 含无效网段 %q: %w", raw, err)
		}
		nets = append(nets, n)
	}
	trustedProxies.Store(&nets)
	return nil
}

func isTrustedProxy(ip net.IP) bool {
	nets := trustedProxies.Load()
	if nets == nil {
		return false
	}
	for _, n := range *nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
