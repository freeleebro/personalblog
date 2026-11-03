package api

import (
	"strconv"
	"strings"
	"testing"
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

func TestSlugify(t *testing.T) {
	// 这里锁定的是当前行为：ASCII 转小写、少数分隔符变连字符、中文原样保留。
	// 如果以后给中文加拼音转换（T2.9），这几条断言会一起需要更新。
	cases := map[string]string{
		"Hello World": "hello-world",
		"a/b?c":       "a-b-c",
		"C++ 入门":      "c-入门",
		"个人博客":        "个人博客",
		"":            "untitled",
		// 连字符本身不在允许集合里，会被丢掉，于是整串变成空。
		"---": "untitled",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q)=%q，期望 %q", in, got, want)
		}
	}
}
