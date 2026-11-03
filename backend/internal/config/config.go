// Package config 负责从环境变量（以及可选的 .env 文件）读取运行配置。
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // 把 IANA 时区库编进二进制，容器里不必装 tzdata
)

// Config 是进程级配置。
type Config struct {
	Env          string
	Addr         string
	DatabaseURL  string
	AllowedHosts []string

	SiteName        string
	SiteTagline     string
	SiteDescription string
	SiteURL         string

	AutoMigrate bool

	// AutoCreateDatabase 在目标库不存在时自动创建，
	// 让「不用 Docker 也能跑起来」这条路径不需要额外的客户端工具。
	AutoCreateDatabase bool

	// Admin 相关：管理后台的默认账号与 token 签名密钥。
	// AdminUsername / AdminPassword 仅在首次启动（admin_users 表为空）时用于
	// 创建初始账号，之后可忽略。AdminSecret 用于 HMAC 签名登录 token。
	AdminUsername string
	AdminPassword string
	AdminSecret   string

	// AdminPasswordReset 为 true 时，启动阶段会用 ADMIN_PASSWORD 覆盖数据库里的哈希，
	// 用于「忘了后台密码、但有服务器权限」的场景；平时应保持关闭。
	AdminPasswordReset bool

	// AuthorSlug 指向 authors 表里默认展示的作者；没配就退化成 "author"。
	AuthorSlug string

	// SiteLocation 用于生成带日期的 slug（如 post-20261103-ab12cd）等
	// 「按站点本地时区」计算的时间，默认 Asia/Shanghai。
	SiteLocation *time.Location

	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration

	// TrustedProxies 是允许提供 X-Forwarded-For / X-Real-IP 的代理网段。
	// 只有请求确实来自这些网段时才采信转发头，否则一律用 RemoteAddr。
	TrustedProxies []string

	// UploadDir 是文章配图的落盘目录。生产环境挂一个数据卷进来，
	// 这样换镜像不会把用户上传的图片一起换掉。
	UploadDir string

	// CommentsEnabled 控制文章页是否展示评论；CommentAutoApprove 为 true 时
	// 新评论直接公开，否则先进待审核队列（默认，适合公开站点防垃圾评论）。
	CommentsEnabled    bool
	CommentAutoApprove bool
}

// Load 读取配置。查找顺序：真实环境变量 > 当前目录 .env > 上一级目录 .env。
func Load() Config {
	loadDotEnv(".env")
	loadDotEnv(filepath.Join("..", ".env"))

	return Config{
		Env:          getEnv("APP_ENV", "development"),
		Addr:         getEnv("APP_ADDR", ":8080"),
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://blog:blog@127.0.0.1:5433/beichuang?sslmode=disable"),
		AllowedHosts: splitList(getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000")),

		SiteName:        getEnv("SITE_NAME", ""),
		SiteTagline:     getEnv("SITE_TAGLINE", ""),
		SiteDescription: getEnv("SITE_DESCRIPTION", ""),
		SiteURL:         getEnv("SITE_URL", "http://localhost:3000"),

		AutoMigrate:        getBool("AUTO_MIGRATE", true),
		AutoCreateDatabase: getBool("AUTO_CREATE_DATABASE", true),

		AdminUsername:      getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword:      getEnv("ADMIN_PASSWORD", "admin"),
		AdminSecret:        getEnv("ADMIN_SECRET", "change-me-in-production"),
		AdminPasswordReset: getBool("ADMIN_PASSWORD_RESET", false),
		AuthorSlug:         getEnv("AUTHOR_SLUG", "author"),
		SiteLocation:       loadLocation(getEnv("SITE_TIMEZONE", "Asia/Shanghai")),

		ReadTimeout:     getDuration("READ_TIMEOUT", 15*time.Second),
		WriteTimeout:    getDuration("WRITE_TIMEOUT", 30*time.Second),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 15*time.Second),

		// 默认信任回环 + 内网网段：容器里反向代理总是从这些地址连进来。
		// 没有代理时不用改：公网来源不在这些网段内，转发头会被直接忽略。
		TrustedProxies: splitList(getEnv("TRUSTED_PROXIES",
			"127.0.0.0/8,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,fc00::/7")),

		// 本地开发默认落在 backend/data/uploads；容器里由 UPLOAD_DIR 指到数据卷。
		UploadDir: getEnv("UPLOAD_DIR", "./data/uploads"),

		// 评论默认开启，但新评论需要站长在后台点「通过」才公开。
		CommentsEnabled:    getBool("COMMENTS_ENABLED", true),
		CommentAutoApprove: getBool("COMMENT_AUTO_APPROVE", false),
	}
}

// IsProduction 用于决定是否输出更严格的日志与错误细节。
func (c Config) IsProduction() bool { return c.Env == "production" }

// loadLocation 解析 SITE_TIMEZONE。时区数据已通过 time/tzdata 编进二进制，
// 容器里没装 tzdata 也能用；名字非法时退回 UTC，并保持可用的固定偏移。
func loadLocation(name string) *time.Location {
	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}
	if loc, err := time.LoadLocation("UTC"); err == nil {
		return loc
	}
	return time.UTC
}

// Location 返回站点时区；Config 是零值时退回 UTC，避免调用方判断 nil。
func (c Config) Location() *time.Location {
	if c.SiteLocation == nil {
		return time.UTC
	}
	return c.SiteLocation
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return b
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return d
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv 解析一个极简的 .env 文件：KEY=VALUE，支持 # 注释与引号。
// 真实环境变量优先级更高，已存在的变量不会被覆盖。
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}
