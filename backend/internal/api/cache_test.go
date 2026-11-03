package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCacheableRequest(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		header map[string]string
		want   bool
	}{
		{"公开列表", http.MethodGet, "/api/v1/posts", nil, true},
		{"首页聚合", http.MethodGet, "/api/v1/site", nil, true},
		{"后台接口不缓存", http.MethodGet, "/api/v1/admin/posts", nil, false},
		{"健康检查不缓存", http.MethodGet, "/api/v1/health", nil, false},
		{"草稿预览不缓存", http.MethodGet, "/api/v1/preview/abc", nil, false},
		{"写操作不缓存", http.MethodPost, "/api/v1/posts", nil, false},
		{"带 Authorization 不缓存", http.MethodGet, "/api/v1/posts", map[string]string{"Authorization": "Bearer x"}, false},
		{"客户端要求 no-store 不缓存", http.MethodGet, "/api/v1/posts", map[string]string{"Cache-Control": "no-store"}, false},
		{"非 api 路径不缓存", http.MethodGet, "/media/a.png", nil, false},
		{"媒体接口不缓存", http.MethodHead, "/api/v1/posts", nil, false},
	}

	for _, tc := range cases {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		for k, v := range tc.header {
			r.Header.Set(k, v)
		}
		if got := cacheableRequest(r); got != tc.want {
			t.Errorf("%s（%s %s）：得到 %v，期望 %v", tc.name, tc.method, tc.path, got, tc.want)
		}
	}
}

func TestIsAdminMutation(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodPost, "/api/v1/admin/posts", true},
		{http.MethodPut, "/api/v1/admin/site", true},
		{http.MethodDelete, "/api/v1/admin/tags/x", true},
		{http.MethodPost, "/api/v1/posts/hello/views", true},
		{http.MethodGet, "/api/v1/admin/posts", false},
		// 登录也会命中 admin 前缀从而清一次缓存：代价是下一次读多打一次库，
		// 换来「登录后立刻看到最新内容」，可以接受。
		{http.MethodPost, "/api/v1/admin/login", true},
	}

	for _, tc := range cases {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		if got := isAdminMutation(r); got != tc.want {
			t.Errorf("%s %s：得到 %v，期望 %v", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestResponseCacheExpires(t *testing.T) {
	var c responseCache
	now := time.Now()
	c.set("k", cacheEntry{status: 200, body: []byte("x"), expires: now.Add(time.Second)})

	if _, ok := c.get("k", now); !ok {
		t.Fatal("未过期的条目应该能命中")
	}
	if _, ok := c.get("k", now.Add(2*time.Second)); ok {
		t.Fatal("过期条目不应命中")
	}
	c.clear()
	if _, ok := c.get("k", now); ok {
		t.Fatal("clear 之后不应命中")
	}
}

func TestCacheMiddlewareHitsAndClears(t *testing.T) {
	s := &Server{}
	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":"ok"}`))
	})
	h := s.cacheMiddleware(next)

	get := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/posts", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	first := get()
	if got := first.Header().Get("X-Cache"); got != "MISS" {
		t.Fatalf("第一次请求应是 MISS，得到 %q", got)
	}
	second := get()
	if got := second.Header().Get("X-Cache"); got != "HIT" {
		t.Fatalf("第二次请求应是 HIT，得到 %q", got)
	}
	if calls != 1 {
		t.Fatalf("命中缓存时不该再调用下游，实际调用 %d 次", calls)
	}
	if second.Body.String() != first.Body.String() {
		t.Fatalf("缓存响应体应与首次一致：%q vs %q", second.Body.String(), first.Body.String())
	}

	// 后台写操作后缓存应整体失效。
	post := httptest.NewRequest(http.MethodPost, "/api/v1/admin/posts", nil)
	h.ServeHTTP(httptest.NewRecorder(), post)
	if got := get().Header().Get("X-Cache"); got != "MISS" {
		t.Fatalf("写操作后应重新回源，得到 %q", got)
	}
}
