package api

import (
	"strings"
	"testing"
	"time"

	"github.com/freeleebro/personalblog/backend/internal/domain"
)

func TestExportFileName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello-world", "hello-world"},
		{"个人博客", "个人博客"},
		{"", "untitled"},
		{"  ", "untitled"},
		{"a/b", "a-b"},
		{"../etc", "--etc"},
		{"a\\b", "a-b"},
	}
	for _, c := range cases {
		if got := exportFileName(c.in); got != c.want {
			t.Errorf("exportFileName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFrontMatterValue(t *testing.T) {
	cases := []struct{ in, want string }{
		{"普通标题", "普通标题"},
		{"  多余空格  ", "多余空格"},
		{"第一行\n第二行", "第一行 第二行"},
		{"第一行\r\n第二行", "第一行 第二行"},
		{`"带引号"`, "带引号"},
	}
	for _, c := range cases {
		if got := frontMatterValue(c.in); got != c.want {
			t.Errorf("frontMatterValue(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	// 换行必须被压平，否则导入器按行解析会把正文误当成 front matter。
	if got := frontMatterValue("a\nb\nc"); strings.Contains(got, "\n") {
		t.Errorf("frontMatterValue 仍含换行: %q", got)
	}
}

func TestRenderPostMarkdown(t *testing.T) {
	published := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	p := domain.Post{
		Slug:        "hello-world",
		Title:       "你好，世界",
		Summary:     "第一篇文章",
		BodyMD:      "## 小标题\n\n正文。",
		Status:      "published",
		PublishedAt: &published,
		CoverURL:    "/media/2026/11/a.webp",
		Tags:        []domain.Tag{{Name: "随笔"}, {Name: "生活"}},
	}

	md := renderPostMarkdown(p)

	for _, want := range []string{
		"---\n",
		"title: 你好，世界\n",
		"slug: hello-world\n",
		"summary: 第一篇文章\n",
		"status: published\n",
		"published_at: 2026-11-03T09:00:00Z\n",
		"cover_url: /media/2026/11/a.webp\n",
		"tags: 随笔, 生活\n",
		"## 小标题\n\n正文。\n",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("导出内容缺少 %q\n---\n%s", want, md)
		}
	}

	// front matter 只应出现一次开头的 --- 与一次结束的 ---，正文里的分隔线不受影响。
	if strings.Count(md, "\n---\n") < 1 {
		t.Errorf("front matter 分隔符缺失:\n%s", md)
	}
}

func TestRenderPostMarkdownFallsBackToTitle(t *testing.T) {
	md := renderPostMarkdown(domain.Post{Slug: "empty", Title: "空文章", Status: "draft"})
	if !strings.Contains(md, "# 空文章") {
		t.Errorf("正文为空时应回退成标题，实际:\n%s", md)
	}
	if strings.Contains(md, "published_at:") {
		t.Errorf("草稿不应写出 published_at:\n%s", md)
	}
}

func TestRenderPageMarkdown(t *testing.T) {
	md := renderPageMarkdown(domain.Page{Slug: "about", Title: "关于", BodyMD: "这里是我。"})
	for _, want := range []string{"title: 关于\n", "slug: about\n", "这里是我。\n"} {
		if !strings.Contains(md, want) {
			t.Errorf("页面导出缺少 %q:\n%s", want, md)
		}
	}
}
