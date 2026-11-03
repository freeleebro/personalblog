// Package markdown 提供纯文本抽取、摘要生成与阅读时长估算。
// 渲染交给前端的 react-markdown，这里只做「服务端需要知道的文本统计」。
package markdown

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	reFence      = regexp.MustCompile("(?s)```.*?```")
	reInlineCode = regexp.MustCompile("`[^`]*`")
	reImage      = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reLink       = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reHeading    = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s*`)
	reFirstHead  = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s+.*\n`)
	reQuote      = regexp.MustCompile(`(?m)^\s{0,3}>\s?`)
	reListMark   = regexp.MustCompile(`(?m)^\s*([-*+]|\d+[.)])\s+`)
	reTableSep   = regexp.MustCompile(`(?m)^\s*\|?[\s:|-]{3,}\|?\s*$`)
	reEmphasis   = regexp.MustCompile(`(\*\*|__|\*|_|~~)`)
	reHTMLTag    = regexp.MustCompile(`<[^>]+>`)
	reBlankRun   = regexp.MustCompile(`\n{3,}`)
	reSpaceRun   = regexp.MustCompile(`[ \t]{2,}`)
	reCJK        = regexp.MustCompile(`[\p{Han}\p{Hiragana}\p{Katakana}\p{Hangul}]`)
	reTrailing   = regexp.MustCompile(`[\s，,、；;：:]+$`)
)

// PlainText 把 Markdown 粗加工成可读纯文本。
// 它不追求完美还原，只需要在摘要、搜索与字数统计场景下与原文语义一致。
func PlainText(src string) string {
	s := src
	s = reFence.ReplaceAllString(s, " ")
	s = reInlineCode.ReplaceAllString(s, " ")
	s = reImage.ReplaceAllString(s, " ")
	s = reLink.ReplaceAllString(s, "$1")
	s = reHeading.ReplaceAllString(s, "")
	s = reQuote.ReplaceAllString(s, "")
	s = reListMark.ReplaceAllString(s, "")
	s = reTableSep.ReplaceAllString(s, "")
	s = reEmphasis.ReplaceAllString(s, "")
	s = reHTMLTag.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "|", " ")
	s = reSpaceRun.ReplaceAllString(s, " ")
	s = reBlankRun.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// Excerpt 截取摘要。正文常以同名标题开头，这里会先把它去掉。
func Excerpt(src string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 120
	}
	if m := reFirstHead.FindString(src); m != "" {
		src = strings.Replace(src, m, "", 1)
	}
	text := strings.Join(strings.Fields(PlainText(src)), " ")

	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	cut := string(runes[:maxRunes])
	if i := strings.LastIndexAny(cut, "。！？.!?；;，,、 "); i > maxRunes/2 {
		cut = cut[:i]
	}
	return reTrailing.ReplaceAllString(cut, "") + "……"
}

// ReadingMinutes 估算阅读时长：中文约 400 字/分钟，西文约 220 词/分钟，最少 1 分钟。
func ReadingMinutes(src string) int {
	text := PlainText(src)
	cjk := len(reCJK.FindAllString(text, -1))

	latin := 0
	for _, w := range strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if !reCJK.MatchString(w) {
			latin++
		}
	}

	minutes := float64(cjk)/400 + float64(latin)/220
	if minutes < 1 {
		return 1
	}
	return int(minutes + 0.5)
}

// WordCount 返回 CJK 字符数与西文词数的加权统计，用于站点「总字数」展示。
func WordCount(src string) int {
	text := PlainText(src)
	return len(reCJK.FindAllString(text, -1)) + len(strings.Fields(text))
}
