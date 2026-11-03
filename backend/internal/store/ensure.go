package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// EnsureDatabase 在目标数据库不存在时创建它。
//
// 用 Docker 启动时 PostgreSQL 镜像会依据 POSTGRES_DB 自动建库，这一步是空操作；
// 而在「本机跑一个便携版 PostgreSQL」的场景里没有 psql/createdb 可用，
// 由服务自己把库建出来能省掉一整套客户端工具依赖。
func EnsureDatabase(ctx context.Context, dsn string) error {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("解析 DATABASE_URL 失败: %w", err)
	}
	target := cfg.Database
	if target == "" {
		return nil
	}

	// 连到维护库 postgres 上执行检查与创建
	adminCfg := cfg.Copy()
	adminCfg.Database = "postgres"

	conn, err := pgx.ConnectConfig(ctx, adminCfg)
	if err != nil {
		return fmt.Errorf("连接维护库 postgres 失败: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	var exists bool
	if err := conn.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, target,
	).Scan(&exists); err != nil {
		return fmt.Errorf("检查数据库是否存在失败: %w", err)
	}
	if exists {
		return nil
	}

	// 数据库名无法作为参数绑定，只能拼进 SQL，因此必须手动转义标识符。
	quoted := `"` + strings.ReplaceAll(target, `"`, `""`) + `"`
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		return fmt.Errorf("创建数据库 %s 失败: %w", target, err)
	}
	return nil
}
