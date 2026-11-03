package api

import (
	"sync"
	"time"
)

// loginGuard 按「客户端 IP + 用户名」记录登录失败次数，连续失败到阈值后
// 锁定一段时间。单实例进程内实现即可——博客只有一个后端进程，
// 引入 Redis 换来的只是多一个需要运维的组件。
type loginGuard struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt
}

type loginAttempt struct {
	failures int
	last     time.Time
	blocked  time.Time
}

const (
	// loginMaxFailures 是触发锁定的连续失败次数。
	loginMaxFailures = 5
	// loginBlockWindow 是锁定时长。
	loginBlockWindow = 15 * time.Minute
	// loginAttemptTTL 之后清掉无用的记录，避免 map 无限增长。
	loginAttemptTTL = time.Hour
)

func (g *loginGuard) lookup(key string, now time.Time) (*loginAttempt, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	a, ok := g.attempts[key]
	if !ok {
		return nil, false
	}
	if now.Sub(a.last) > loginAttemptTTL && now.After(a.blocked) {
		delete(g.attempts, key)
		return nil, false
	}
	return a, true
}

// blockedFor 返回剩余的锁定时长；未锁定返回 0。
func (g *loginGuard) blockedFor(key string) time.Duration {
	now := time.Now()
	a, ok := g.lookup(key, now)
	if !ok || a.blocked.IsZero() {
		return 0
	}
	if remaining := a.blocked.Sub(now); remaining > 0 {
		return remaining
	}
	return 0
}

// fail 记录一次失败，达到阈值后开始锁定。
func (g *loginGuard) fail(key string) {
	now := time.Now()

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.attempts == nil {
		g.attempts = make(map[string]*loginAttempt)
	}
	a, ok := g.attempts[key]
	if !ok {
		a = &loginAttempt{}
		g.attempts[key] = a
	}
	a.failures++
	a.last = now
	if a.failures >= loginMaxFailures {
		a.blocked = now.Add(loginBlockWindow)
		a.failures = 0
	}
	if len(g.attempts) > 10_000 {
		for k, v := range g.attempts {
			if now.Sub(v.last) > loginAttemptTTL && now.After(v.blocked) {
				delete(g.attempts, k)
			}
		}
	}
}

// reset 在登录成功后清空该 key 的失败记录。
func (g *loginGuard) reset(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.attempts, key)
}
