package markdown

import (
	"strings"
	"testing"
)

func TestPlainTextStripsMarkdownSyntax(t *testing.T) {
	src := "# 标题\n\n正文**加粗**和`code`，[链接](https://example.com) 与 ![图](/a.png)。\n\n```go\nfmt.Println(1)\n```\n"
	got := PlainText(src)

	for _, banned := range []string{"**", "```", "[", "]", "fmt.Println", "#"} {
		if strings.Contains(got, banned) {
			t.Fatalf("PlainText 输出仍包含 %q：%q", banned, got)
		}
	}
	if !strings.Contains(got, "正文") || !strings.Contains(got, "链接") {
		t.Fatalf("PlainText 丢失正文内容：%q", got)
	}
}

func TestExcerptDropsLeadingHeading(t *testing.T) {
	src := "# 这是一篇很长很长的文章标题\n\n这是正文的第一段，讲了一件具体的事。"
	got := Excerpt(src, 40)

	if strings.HasPrefix(got, "这是一篇很长很长的文章标题") {
		t.Fatalf("Excerpt 没有去掉开头的同名标题：%q", got)
	}
	if !strings.Contains(got, "这是正文") {
		t.Fatalf("Excerpt 丢失正文：%q", got)
	}
}

func TestExcerptTruncatesWithEllipsis(t *testing.T) {
	src := strings.Repeat("字", 200)
	got := Excerpt(src, 20)

	if !strings.HasSuffix(got, "……") {
		t.Fatalf("超长摘要应以省略号结尾：%q", got)
	}
	if n := len([]rune(got)); n > 24 {
		t.Fatalf("摘要长度没有收敛，得到 %d 个字符：%q", n, got)
	}
}

func TestExcerptKeepsShortTextIntact(t *testing.T) {
	if got := Excerpt("一句话。", 40); got != "一句话。" {
		t.Fatalf("短文本不该被改动，得到 %q", got)
	}
}

func TestReadingMinutes(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"空文本", "", 1},
		{"很短", "你好", 1},
		{"400 汉字约一分钟", strings.Repeat("字", 400), 1},
		{"1200 汉字约三分钟", strings.Repeat("字", 1200), 3},
	}
	for _, tc := range cases {
		if got := ReadingMinutes(tc.src); got != tc.want {
			t.Errorf("%s：ReadingMinutes=%d，期望 %d", tc.name, got, tc.want)
		}
	}
}

func TestWordCountCountsCJKAndLatin(t *testing.T) {
	if got := WordCount("你好 world"); got < 3 {
		t.Fatalf("WordCount 应同时统计中文与西文词，得到 %d", got)
	}
	// 代码块不参与字数统计。
	if got := WordCount("正文\n\n```\nvar a = 1\n```\n"); got > 4 {
		t.Fatalf("代码块被算进字数了：%d", got)
	}
}
