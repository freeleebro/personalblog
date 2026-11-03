package api

import (
	"testing"
	"time"
)

func TestLoginGuardBlocksAfterThreshold(t *testing.T) {
	var g loginGuard
	key := "203.0.113.9|admin"

	// 前 N-1 次失败还不该锁定
	for i := 0; i < loginMaxFailures-1; i++ {
		g.fail(key)
		if d := g.blockedFor(key); d != 0 {
			t.Fatalf("第 %d 次失败后就被锁定了（%s）", i+1, d)
		}
	}

	// 第 N 次触发锁定
	g.fail(key)
	if d := g.blockedFor(key); d <= 0 {
		t.Fatal("达到阈值后应锁定，实际未锁定")
	} else if d > loginBlockWindow {
		t.Fatalf("锁定时长 %s 超过上限 %s", d, loginBlockWindow)
	}
}

func TestLoginGuardResetClearsBlock(t *testing.T) {
	var g loginGuard
	key := "203.0.113.9|admin"

	for i := 0; i < loginMaxFailures; i++ {
		g.fail(key)
	}
	if g.blockedFor(key) == 0 {
		t.Fatal("应处于锁定状态")
	}

	g.reset(key)
	if d := g.blockedFor(key); d != 0 {
		t.Fatalf("reset 后仍锁定 %s", d)
	}
}

func TestLoginGuardIsolatesKeys(t *testing.T) {
	var g loginGuard
	for i := 0; i < loginMaxFailures; i++ {
		g.fail("203.0.113.9|admin")
	}
	if g.blockedFor("203.0.113.9|admin") == 0 {
		t.Fatal("admin 应被锁定")
	}
	if g.blockedFor("203.0.113.9|someone-else") != 0 {
		t.Fatal("其它用户名不应被连带锁定")
	}
	if g.blockedFor("198.51.100.1|admin") != 0 {
		t.Fatal("其它 IP 不应被连带锁定")
	}
}

func TestLoginGuardExpiresStaleEntries(t *testing.T) {
	var g loginGuard
	key := "203.0.113.9|admin"

	g.mu.Lock()
	g.attempts = map[string]*loginAttempt{
		key: {failures: 1, last: time.Now().Add(-2 * loginAttemptTTL)},
	}
	g.mu.Unlock()

	if d := g.blockedFor(key); d != 0 {
		t.Fatalf("过期记录不应产生锁定，got %s", d)
	}
}
