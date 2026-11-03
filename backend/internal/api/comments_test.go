package api

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeComment(t *testing.T) {
	t.Run("合法评论", func(t *testing.T) {
		author, email, body, err := normalizeComment(commentInput{
			Author: "  小明 ",
			Email:  " xm@example.com ",
			Body:   "  写得不错  ",
		})
		if err != nil {
			t.Fatalf("不应报错: %v", err)
		}
		if author != "小明" || email != "xm@example.com" || body != "写得不错" {
			t.Fatalf("规范化结果不对: %q %q %q", author, email, body)
		}
	})

	t.Run("昵称必填且限长", func(t *testing.T) {
		if _, _, _, err := normalizeComment(commentInput{Author: "   ", Body: "你好"}); err == nil {
			t.Fatal("空昵称应报错")
		}
		long := strings.Repeat("名", maxCommentAuthorRunes+1)
		if _, _, _, err := normalizeComment(commentInput{Author: long, Body: "你好"}); err == nil {
			t.Fatal("超长昵称应报错")
		}
	})

	t.Run("邮箱可空但格式要对", func(t *testing.T) {
		if _, _, _, err := normalizeComment(commentInput{Author: "a", Body: "你好"}); err != nil {
			t.Fatalf("不填邮箱应允许: %v", err)
		}
		if _, _, _, err := normalizeComment(commentInput{Author: "a", Email: "not-an-email", Body: "你好"}); err == nil {
			t.Fatal("非法邮箱应报错")
		}
	})

	t.Run("正文长度边界", func(t *testing.T) {
		if _, _, _, err := normalizeComment(commentInput{Author: "a", Body: "好"}); err == nil {
			t.Fatal("1 个字应报错")
		}
		if _, _, _, err := normalizeComment(commentInput{Author: "a", Body: "你好"}); err != nil {
			t.Fatalf("2 个字应允许: %v", err)
		}
		tooLong := strings.Repeat("字", maxCommentBodyRunes+1)
		if _, _, _, err := normalizeComment(commentInput{Author: "a", Body: tooLong}); err == nil {
			t.Fatal("超长正文应报错")
		}
	})

	t.Run("统一换行符", func(t *testing.T) {
		_, _, body, err := normalizeComment(commentInput{Author: "a", Body: "第一行\r\n第二行\r第三行"})
		if err != nil {
			t.Fatalf("不应报错: %v", err)
		}
		if strings.Contains(body, "\r") {
			t.Fatalf("换行符应统一为 \\n: %q", body)
		}
	})
}

func TestCommentLimiterWindow(t *testing.T) {
	var l commentLimiter
	now := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)

	for i := 0; i < commentRateLimit; i++ {
		if !l.allow("1.2.3.4", now.Add(time.Duration(i)*time.Second)) {
			t.Fatalf("第 %d 条应放行", i+1)
		}
	}
	if l.allow("1.2.3.4", now.Add(time.Minute)) {
		t.Fatal("超过窗口上限应拒绝")
	}
	if !l.allow("5.6.7.8", now.Add(time.Minute)) {
		t.Fatal("不同 IP 不应受影响")
	}

	// 窗口滑过之后，之前那批记录都应过期。
	later := now.Add(commentRateWindow + time.Minute)
	if !l.allow("1.2.3.4", later) {
		t.Fatal("窗口过期后应重新放行")
	}
}

func TestHashCommentIP(t *testing.T) {
	a := hashCommentIP("203.0.113.9", "secret")
	if len(a) != 16 {
		t.Fatalf("哈希长度应为 16，得到 %d: %q", len(a), a)
	}
	if a != hashCommentIP("203.0.113.9", "secret") {
		t.Fatal("相同输入应得到相同哈希")
	}
	if a == hashCommentIP("203.0.113.9", "other-secret") {
		t.Fatal("换密钥后哈希必须变化")
	}
	if a == hashCommentIP("203.0.113.10", "secret") {
		t.Fatal("换 IP 后哈希必须变化")
	}
	if strings.Contains(a, "203.0.113.9") {
		t.Fatal("哈希里不应含原始 IP")
	}
}
