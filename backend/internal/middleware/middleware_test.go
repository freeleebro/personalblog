package middleware

import (
	"net/http"
	"testing"
)

func mustTrusted(t *testing.T, proxies []string) {
	t.Helper()
	if err := SetTrustedProxies(proxies); err != nil {
		t.Fatalf("SetTrustedProxies: %v", err)
	}
}

func requestFrom(remote string, headers map[string]string) *http.Request {
	r := &http.Request{RemoteAddr: remote, Header: make(http.Header)}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

// 公网直连时，任何人都能自带 X-Forwarded-For；必须忽略它。
func TestClientIPIgnoresForwardedHeadersFromUntrustedPeer(t *testing.T) {
	mustTrusted(t, DefaultTrustedProxies())

	r := requestFrom("203.0.113.9:54321", map[string]string{
		"X-Forwarded-For": "1.2.3.4",
		"X-Real-IP":       "5.6.7.8",
	})
	if got := ClientIP(r); got != "203.0.113.9" {
		t.Fatalf("ClientIP = %q, want 203.0.113.9（不应采信伪造的转发头）", got)
	}
}

// 经过可信代理时，取 XFF 里从右往左第一个不可信地址。
func TestClientIPTakesRightmostUntrustedHop(t *testing.T) {
	mustTrusted(t, DefaultTrustedProxies())

	r := requestFrom("172.18.0.2:41234", map[string]string{
		// 客户端自己伪造了 1.1.1.1，代理在右侧追加了真实来源
		"X-Forwarded-For": "1.1.1.1, 203.0.113.7",
	})
	if got := ClientIP(r); got != "203.0.113.7" {
		t.Fatalf("ClientIP = %q, want 203.0.113.7", got)
	}
}

// 整条链路都是可信代理时，回退到直连地址。
func TestClientIPFallsBackWhenAllHopsTrusted(t *testing.T) {
	mustTrusted(t, DefaultTrustedProxies())

	r := requestFrom("172.18.0.2:41234", map[string]string{
		"X-Forwarded-For": "10.1.2.3, 192.168.1.5",
	})
	if got := ClientIP(r); got != "172.18.0.2" {
		t.Fatalf("ClientIP = %q, want 172.18.0.2", got)
	}
}

// 未配置可信代理（显式只信任一个无关网段）时，任何转发头都不采信。
func TestClientIPWithoutTrustedProxy(t *testing.T) {
	mustTrusted(t, []string{"198.51.100.0/24"})

	r := requestFrom("127.0.0.1:50000", map[string]string{
		"X-Forwarded-For": "203.0.113.7",
	})
	if got := ClientIP(r); got != "127.0.0.1" {
		t.Fatalf("ClientIP = %q, want 127.0.0.1", got)
	}

	mustTrusted(t, DefaultTrustedProxies()) // 还原，避免影响其它测试
}

func TestSetTrustedProxiesRejectsGarbage(t *testing.T) {
	if err := SetTrustedProxies([]string{"not-an-ip"}); err == nil {
		t.Fatal("期望非法网段返回错误")
	}
	mustTrusted(t, DefaultTrustedProxies())
}
