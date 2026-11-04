package api

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSanitizeTexts(t *testing.T) {
	long := strings.Repeat("字", maxSiteTextLen+50)
	in := map[string]string{
		"home.title": "  标题  ",
		"Bad Key":    "含非法字符的键",
		"UPPER":      "大写键",
		"":           "空键",
		"blank":      "   ",
		"long_value": long,
		"ok-key.2_3": "正常值",
	}

	out := sanitizeTexts(in)

	if got := out["home.title"]; got != "标题" {
		t.Errorf("值应去掉首尾空白，得到 %q", got)
	}
	if _, ok := out["Bad Key"]; ok {
		t.Error("含空格的键应被丢弃")
	}
	if _, ok := out["UPPER"]; ok {
		t.Error("大写键应被丢弃（前端约定全小写）")
	}
	if _, ok := out[""]; ok {
		t.Error("空键应被丢弃")
	}
	if _, ok := out["blank"]; ok {
		t.Error("全空白的值应被丢弃")
	}
	if got := len([]rune(out["long_value"])); got != maxSiteTextLen {
		t.Errorf("超长值应截断到 %d 个字符，得到 %d", maxSiteTextLen, got)
	}
	if got := out["ok-key.2_3"]; got != "正常值" {
		t.Errorf("合法键值应原样保留，得到 %q", got)
	}
	if len(out) != 3 {
		t.Errorf("应只剩 3 条合法文案，得到 %d：%v", len(out), out)
	}
}

func TestSanitizeTextsCapsCount(t *testing.T) {
	in := make(map[string]string, maxSiteTexts+10)
	for i := 0; i < maxSiteTexts+10; i++ {
		in["k"+strconv.Itoa(i)] = "v"
	}
	if got := len(sanitizeTexts(in)); got > maxSiteTexts {
		t.Fatalf("文案条数应封顶在 %d，得到 %d", maxSiteTexts, got)
	}
}

func TestPostSlug(t *testing.T) {
	// ASCII 标题走可读 slug；含非 ASCII 字符时退回 post-日期-哈希。
	fixed := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	cases := map[string]string{
		"Hello World":  "hello-world",
		"a/b?c":        "a-b-c",
		"Hello  World": "hello-world", // 连续分隔符折叠成一个
		"  Hello  ":    "hello",
		"v1.2.3":       "v1-2-3",
	}
	for in, want := range cases {
		if got := postSlug(in, fixed); got != want {
			t.Errorf("postSlug(%q)=%q，期望 %q", in, got, want)
		}
	}

	// 中文 / 混合标题：前缀固定，后缀是 6 位十六进制。
	for _, in := range []string{"C++ 入门", "个人博客", "", "---", "测试文章"} {
		got := postSlug(in, fixed)
		if !strings.HasPrefix(got, "post-20261103-") {
			t.Errorf("postSlug(%q)=%q，期望 post-20261103- 前缀", in, got)
			continue
		}
		suffix := strings.TrimPrefix(got, "post-20261103-")
		if len(suffix) != 6 {
			t.Errorf("postSlug(%q) 的哈希后缀应为 6 位，得到 %q", in, suffix)
		}
		if _, err := strconv.ParseUint(suffix, 16, 32); err != nil {
			t.Errorf("postSlug(%q) 的后缀不是十六进制：%q", in, suffix)
		}
	}

	// 同一个标题在同一天必须稳定，不能每次保存都换 URL。
	if a, b := postSlug("中文标题", fixed), postSlug("中文标题", fixed); a != b {
		t.Errorf("同一标题生成的 slug 不稳定：%q vs %q", a, b)
	}
	if a, b := postSlug("中文标题", fixed), postSlug("另一个标题", fixed); a == b {
		t.Errorf("不同标题生成了相同 slug：%q", a)
	}

	// 超长 ASCII 标题要截断到 maxSlugLen。
	long := strings.Repeat("abcdefghij", 10)
	if got := postSlug(long, fixed); len([]rune(got)) > maxSlugLen {
		t.Errorf("超长标题未截断：%d 字符", len([]rune(got)))
	}
}

func TestTagSlugKeepsChinese(t *testing.T) {
	// 标签沿用旧行为：中文保留，便于可读的 /tags/中文。
	cases := map[string]string{
		"Go":          "go",
		"个人博客":        "个人博客",
		"Hello World": "hello-world",
		"":            "untitled",
	}
	for in, want := range cases {
		if got := tagSlug(in); got != want {
			t.Errorf("tagSlug(%q)=%q，期望 %q", in, got, want)
		}
	}
}

func TestPostInputPublishedAtDefaults(t *testing.T) {
	// published 但没给时间：必须补成当前时间，否则前台按发布时间过滤会看不到。
	in := adminPostInput{Title: "x", Slug: "x", Status: "published"}
	out, err := in.toStore(nil)
	if err != nil {
		t.Fatalf("toStore 报错：%v", err)
	}
	if out.PublishedAt == nil {
		t.Fatal("published 状态没有补 published_at")
	}

	// draft 不该自动带发布时间。
	in = adminPostInput{Title: "x", Slug: "x", Status: "draft"}
	out, err = in.toStore(nil)
	if err != nil {
		t.Fatalf("toStore 报错：%v", err)
	}
	if out.PublishedAt != nil {
		t.Fatal("draft 不应该有 published_at")
	}

	// 显式时间要原样保留（定时发布靠它）。
	at := "2027-01-01T09:00:00+08:00"
	in = adminPostInput{Title: "x", Slug: "x", Status: "published", Published: &at}
	out, err = in.toStore(nil)
	if err != nil {
		t.Fatalf("toStore 报错：%v", err)
	}
	if out.PublishedAt == nil || out.PublishedAt.Format(time.RFC3339) != "2027-01-01T09:00:00+08:00" {
		t.Fatalf("显式发布时间被改写：%v", out.PublishedAt)
	}
}
