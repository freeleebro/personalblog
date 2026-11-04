package api

import "testing"

func TestNormalizeCollectPath(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"/", "/", true},
		{"/posts/hello-deploy", "/posts/hello-deploy", true},
		{"/posts/hello-deploy/", "/posts/hello-deploy", true},
		{"/?utm=1", "/", true},
		{"/search?q=go#top", "/search", true},
		{"  /about  ", "/about", true},
		{"", "", false},
		{"posts/hello", "", false},
		{"https://example.com/x", "", false},
		{"/admin", "", false},
		{"/admin/posts", "", false},
		{"/api/v1/site", "", false},
		{"/api-proxy/api/v1/site", "", false},
		{"/_next/static/chunk.js", "", false},
		{"/preview/draft", "", false},
		{"/media/2026/11/x.webp", "", false},
	}
	for _, c := range cases {
		got, ok := normalizeCollectPath(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("normalizeCollectPath(%q) = (%q, %v)，期望 (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestNormalizeReferrer(t *testing.T) {
	cases := map[string]string{
		"":                                     "",
		"https://www.google.com/search?q=go":   "www.google.com",
		"http://Example.COM/a":                 "example.com",
		"https://mp.weixin.qq.com/s/abc?x=1#y": "mp.weixin.qq.com",
		"not a url":                            "",
		"/relative":                            "",
	}
	for in, want := range cases {
		if got := normalizeReferrer(in); got != want {
			t.Errorf("normalizeReferrer(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestIsBotUA(t *testing.T) {
	bots := []string{
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"curl/8.5.0",
		"Mozilla/5.0 (compatible; bingbot/2.0)",
		"HeadlessChrome/120",
	}
	for _, ua := range bots {
		if !isBotUA(ua) {
			t.Errorf("isBotUA(%q) = false，期望 true", ua)
		}
	}

	humans := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1",
		"",
	}
	for _, ua := range humans {
		if isBotUA(ua) {
			t.Errorf("isBotUA(%q) = true，期望 false", ua)
		}
	}
}

func TestVisitorHash(t *testing.T) {
	h := visitorHash("1.2.3.4", "ua", "2026-11-04", "secret")
	if len(h) != 16 {
		t.Fatalf("visitorHash 长度 = %d，期望 16", len(h))
	}
	if h != visitorHash("1.2.3.4", "ua", "2026-11-04", "secret") {
		t.Error("相同输入应得到相同哈希")
	}
	if h == visitorHash("1.2.3.5", "ua", "2026-11-04", "secret") {
		t.Error("换 IP 应得到不同哈希")
	}
	if h == visitorHash("1.2.3.4", "ua", "2026-11-05", "secret") {
		t.Error("换日期应得到不同哈希（跨天无法关联）")
	}
	if h == visitorHash("1.2.3.4", "ua", "2026-11-04", "other") {
		t.Error("换密钥应得到不同哈希")
	}
}
