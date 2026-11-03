// Package main 是博客后端服务的入口。
// 启动顺序：连接数据库（退避重试）→ 执行迁移 → 启动 HTTP 服务。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/freeleebro/personalblog/backend/internal/api"
	"github.com/freeleebro/personalblog/backend/internal/auth"
	"github.com/freeleebro/personalblog/backend/internal/config"
	"github.com/freeleebro/personalblog/backend/internal/middleware"
	"github.com/freeleebro/personalblog/backend/internal/migrations"
	"github.com/freeleebro/personalblog/backend/internal/store"
)

func main() {
	cfg := config.Load()
	log := newLogger(cfg)

	if err := run(cfg, log); err != nil {
		log.Error("服务启动失败", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	// 收到 SIGINT / SIGTERM 时取消 ctx，驱动优雅关闭。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.AutoCreateDatabase {
		// 建库失败不算致命：多数情况是库已存在或权限不足，
		// 真正的连接问题会在 WaitForReady 里报得更清楚。
		createCtx, cancelCreate := context.WithTimeout(ctx, 15*time.Second)
		if err := store.EnsureDatabase(createCtx, cfg.DatabaseURL); err != nil {
			log.Warn("自动建库未完成，继续尝试连接", "error", err)
		}
		cancelCreate()
	}

	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	log.Info("正在连接数据库…")
	readyCtx, cancelReady := context.WithTimeout(ctx, 90*time.Second)
	defer cancelReady()
	if err := st.WaitForReady(readyCtx, 45, 2*time.Second); err != nil {
		return err
	}
	log.Info("数据库连接就绪")

	if cfg.AutoMigrate {
		applied, err := st.Migrate(ctx, migrations.FS)
		if err != nil {
			return err
		}
		if len(applied) > 0 {
			log.Info("数据库迁移完成", "applied", applied)
		} else {
			log.Info("数据库结构已是最新")
		}
	}

	// 首次启动时若还没有任何后台账号，用环境变量里的默认账号创建一个。
	if err := ensureAdminUser(ctx, st, cfg, log); err != nil {
		return err
	}

	// 全新部署（空库）时放一份中性的初始文案，保证「克隆 → 启动 → 打开首页」
	// 不是一个空白页；具体内容由使用者在后台改成自己的。
	if err := seedSiteContent(ctx, st, cfg, log); err != nil {
		return err
	}

	// 可信代理网段决定是否采信 X-Forwarded-For，配错会直接影响限流与日志。
	if err := middleware.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      api.New(st, cfg, log).Handler(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("HTTP 服务已启动", "addr", cfg.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("收到退出信号，开始优雅关闭")
	}

	// 关闭用独立的 context：此时 ctx 已经被取消了。
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Info("服务已退出")
	return nil
}

func newLogger(cfg config.Config) *slog.Logger {
	level := slog.LevelDebug
	if cfg.IsProduction() {
		level = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

// ensureAdminUser 在后台账号表为空时，用环境变量配置创建初始账号。
// 表里已经有账号时不会覆盖密码——避免每次重启都把手工改过的密码冲掉；
// 但如果管理员确实想换密码，可以显式设置 ADMIN_PASSWORD_RESET=1 重启一次。
func ensureAdminUser(ctx context.Context, st *store.Store, cfg config.Config, log *slog.Logger) error {
	n, err := st.CountAdminUsers(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return syncAdminPassword(ctx, st, cfg, log)
	}

	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	if _, err := st.UpsertAdminUser(ctx, cfg.AdminUsername, hash, "管理员"); err != nil {
		return err
	}
	log.Info("已创建初始管理员账号", "username", cfg.AdminUsername)
	return nil
}

// syncAdminPassword 处理「环境变量里的 ADMIN_PASSWORD 和数据库里的哈希对不上」的情况。// bcrypt 没法反推明文，只能拿候选密码去比对：一致就什么都不做；不一致时，
// 只有显式开启 ADMIN_PASSWORD_RESET 才会真正重置，否则只打一条警告。
// 这样既不会悄悄改掉线上密码，也不会让人对着「改了环境变量却登不上」发懵。
func syncAdminPassword(ctx context.Context, st *store.Store, cfg config.Config, log *slog.Logger) error {
	if cfg.AdminPassword == "" {
		return nil
	}
	u, err := st.GetAdminUserByUsername(ctx, cfg.AdminUsername)
	if errors.Is(err, store.ErrNotFound) {
		// 用户名也改过了，没有把握，交给人工处理。
		return nil
	}
	if err != nil {
		return err
	}
	if auth.CheckPassword(u.PasswordHash, cfg.AdminPassword) {
		return nil
	}
	if !cfg.AdminPasswordReset {
		log.Warn("ADMIN_PASSWORD 与数据库中的哈希不一致；如需重置请设置 ADMIN_PASSWORD_RESET=1 并重启一次",
			"username", cfg.AdminUsername)
		return nil
	}

	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	if _, err := st.UpsertAdminUser(ctx, cfg.AdminUsername, hash, u.DisplayName); err != nil {
		return err
	}
	log.Info("已按 ADMIN_PASSWORD 重置管理员密码", "username", cfg.AdminUsername)
	return nil
}

// defaultAboutBody 是全新部署时「关于」页的占位内容。
// 保持中性，不放任何个人叙述——使用者会在后台把它改成自己的。
const defaultAboutBody = `## 你好

这里是「关于」页面。内容存在数据库里，可以在后台的「关于页面」标签里直接编辑，支持 Markdown。

建议写这几件事：

- 你是谁、在做什么
- 这个博客打算记录什么
- 怎么联系你

## 这个站点

用 Next.js、Go 和 PostgreSQL 搭起来，部署在 Docker 里：一个容器跑数据库、API 和前端，前面挂一层 Caddy 负责 HTTPS 与缓存。
`

// seedSiteContent 只在「库里还没有站点设置 / 关于页」时写入初始值。
// 判断依据是记录不存在，而不是内容为空，所以使用者清空文案后不会被反复灌回来。
func seedSiteContent(ctx context.Context, st *store.Store, cfg config.Config, log *slog.Logger) error {
	var existing map[string]any
	err := st.Setting(ctx, "site", &existing)
	switch {
	case errors.Is(err, store.ErrNotFound):
		site := map[string]any{
			"name":        fallback(cfg.SiteName, "我的博客"),
			"tagline":     fallback(cfg.SiteTagline, "记录，然后分享"),
			"description": cfg.SiteDescription,
			"url":         cfg.SiteURL,
			"socials":     []any{},
			"texts":       map[string]string{},
		}
		if err := st.PutSetting(ctx, "site", site); err != nil {
			return err
		}
		log.Info("已写入初始站点设置")
	case err != nil:
		return err
	}

	if _, err := st.GetPage(ctx, "about"); errors.Is(err, store.ErrNotFound) {
		if err := st.UpsertPage(ctx, "about", "关于", defaultAboutBody); err != nil {
			return err
		}
		log.Info("已写入初始「关于」页面")
	} else if err != nil {
		return err
	}

	return nil
}

func fallback(value, def string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return def
}
