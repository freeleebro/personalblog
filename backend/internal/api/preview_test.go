package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/freeleebro/personalblog/backend/internal/config"
)

func newPreviewServer() *Server {
	return &Server{cfg: config.Config{AdminSecret: "test-secret"}}
}

func TestSignPreviewIsStableAndSecretBound(t *testing.T) {
	s := newPreviewServer()
	exp := time.Now().Add(time.Hour).Unix()

	sig := s.signPreview("hello", exp)
	if sig != s.signPreview("hello", exp) {
		t.Fatal("同样的输入应得到同样的签名")
	}
	if sig == s.signPreview("hello", exp+1) {
		t.Fatal("过期时间变了，签名应随之变化")
	}
	if sig == s.signPreview("other", exp) {
		t.Fatal("slug 变了，签名应随之变化")
	}

	other := &Server{cfg: config.Config{AdminSecret: "another-secret"}}
	if sig == other.signPreview("hello", exp) {
		t.Fatal("换 secret 后签名必须失效")
	}
}

func TestHandlePreviewRejectsBadSignatures(t *testing.T) {
	s := newPreviewServer()
	exp := time.Now().Add(time.Hour).Unix()
	good := s.signPreview("hello", exp)
	expStr := strconv.FormatInt(exp, 10)

	cases := []struct {
		name string
		slug string
		url  string
		want int
	}{
		{"缺 exp", "hello", "/preview/hello?t=" + good, http.StatusNotFound},
		{"签名被篡改", "hello", "/preview/hello?t=deadbeef&exp=" + expStr, http.StatusNotFound},
		{"slug 被替换", "other", "/preview/other?t=" + good + "&exp=" + expStr, http.StatusNotFound},
		{"exp 不是数字", "hello", "/preview/hello?t=" + good + "&exp=abc", http.StatusNotFound},
	}

	for _, tc := range cases {
		r := httptest.NewRequest(http.MethodGet, tc.url, nil)
		r.SetPathValue("slug", tc.slug)
		w := httptest.NewRecorder()
		s.handlePreview(w, r)
		if w.Code != tc.want {
			t.Errorf("%s：得到 %d，期望 %d", tc.name, w.Code, tc.want)
		}
	}
}

func TestHandlePreviewGoneAfterExpiry(t *testing.T) {
	s := newPreviewServer()
	past := time.Now().Add(-time.Minute).Unix()
	sig := s.signPreview("hello", past)

	url := "/preview/hello?exp=" + strconv.FormatInt(past, 10) + "&t=" + sig
	r := httptest.NewRequest(http.MethodGet, url, nil)
	r.SetPathValue("slug", "hello")
	w := httptest.NewRecorder()
	s.handlePreview(w, r)

	if w.Code != http.StatusGone {
		t.Fatalf("过期链接应返回 410，得到 %d：%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("过期响应也不该被中间层缓存，得到 %q", got)
	}
	if !strings.Contains(w.Body.String(), "preview_expired") {
		t.Fatalf("错误码应为 preview_expired：%s", w.Body.String())
	}
}
