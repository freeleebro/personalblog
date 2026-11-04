package api

import "testing"

func TestSplitTagQuery(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		query string
		tag   string
	}{
		{"空", "", "", ""},
		{"只有关键词", "缓存", "缓存", ""},
		{"标签加关键词", "tag:go 缓存", "缓存", "go"},
		{"标签在前带空格", "  tag:postgres   索引  ", "索引", "postgres"},
		{"只有标签", "tag:go", "", "go"},
		{"空标签忽略", "tag: 缓存", "缓存", ""},
		{"多个标签取第一个", "tag:go tag:db 缓存", "缓存", "go"},
		{"大小写保留", "tag:Go 并发", "并发", "Go"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			query, tag := splitTagQuery(c.raw)
			if query != c.query || tag != c.tag {
				t.Fatalf("splitTagQuery(%q) = (%q, %q)，期望 (%q, %q)", c.raw, query, tag, c.query, c.tag)
			}
		})
	}
}
