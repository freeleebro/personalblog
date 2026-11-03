// Package store 封装 PostgreSQL 访问：连接池、迁移器与全部查询。
package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound 表示目标记录不存在，由 API 层翻译成 404。
var ErrNotFound = errors.New("record not found")

// Store 持有 pgx 连接池。
type Store struct {
	pool *pgxpool.Pool
}

// Open 建立连接池。注意这里不做连通性校验，交给 WaitForReady。
func Open(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("解析 DATABASE_URL 失败: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 10 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("创建连接池失败: %w", err)
	}
	return &Store{pool: pool}, nil
}

// WaitForReady 轮询数据库直到可用。容器编排里 PostgreSQL 的 healthy 与
// 「真的能接受查询」之间仍有毫秒级窗口，因此服务启动时做一次退避重试。
func (s *Store) WaitForReady(ctx context.Context, attempts int, interval time.Duration) error {
	var lastErr error
	for i := 1; i <= attempts; i++ {
		if err := s.pool.Ping(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return fmt.Errorf("数据库在 %d 次重试后仍不可用: %w", attempts, lastErr)
}

// Ping 用于健康检查。
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Close 释放连接池。
func (s *Store) Close() { s.pool.Close() }

// Migrate 按文件名升序执行 embed 的 SQL 迁移，已执行过的会跳过。
// 每个文件在独立事务中执行，失败即整体回滚。
func (s *Store) Migrate(ctx context.Context, files fs.FS) ([]string, error) {
	if _, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT        PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return nil, fmt.Errorf("创建 schema_migrations 失败: %w", err)
	}

	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("读取迁移目录失败: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	applied := make([]string, 0, len(names))
	for _, name := range names {
		var done bool
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name,
		).Scan(&done); err != nil {
			return applied, fmt.Errorf("查询迁移状态 %s 失败: %w", name, err)
		}
		if done {
			continue
		}

		body, err := fs.ReadFile(files, name)
		if err != nil {
			return applied, fmt.Errorf("读取迁移 %s 失败: %w", name, err)
		}

		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return applied, fmt.Errorf("开启事务失败: %w", err)
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("执行迁移 %s 失败: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`, name,
		); err != nil {
			_ = tx.Rollback(ctx)
			return applied, fmt.Errorf("记录迁移 %s 失败: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return applied, fmt.Errorf("提交迁移 %s 失败: %w", name, err)
		}
		applied = append(applied, name)
	}
	return applied, nil
}
