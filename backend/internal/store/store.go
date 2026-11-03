// Package store 封装 PostgreSQL 访问：连接池、迁移器与全部查询。
package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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

// migrationChecksum 返回迁移文件内容的 SHA-256 十六进制摘要。
// 用来发现「已经应用过的迁移文件后来被改动」——这种改动不会重跑，
// 数据库结构与代码预期会悄悄分叉。
func migrationChecksum(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// Migrate 按文件名升序执行 embed 的 SQL 迁移，已执行过的会跳过。
// 每个文件在独立事务中执行，失败即整体回滚。
// 已应用的迁移会记录校验和：如果文件在应用后被修改，启动时直接报错，
// 避免「本地库是旧结构、代码按新结构查询」这类难查的问题。
func (s *Store) Migrate(ctx context.Context, files fs.FS) ([]string, error) {
	if _, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT        PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return nil, fmt.Errorf("创建 schema_migrations 失败: %w", err)
	}
	// 老库里的 schema_migrations 只有 version/applied_at，补上校验和列。
	if _, err := s.pool.Exec(ctx,
		`ALTER TABLE schema_migrations ADD COLUMN IF NOT EXISTS checksum TEXT NOT NULL DEFAULT ''`,
	); err != nil {
		return nil, fmt.Errorf("升级 schema_migrations 失败: %w", err)
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
		body, err := fs.ReadFile(files, name)
		if err != nil {
			return applied, fmt.Errorf("读取迁移 %s 失败: %w", name, err)
		}
		checksum := migrationChecksum(body)

		var recorded string
		err = s.pool.QueryRow(ctx,
			`SELECT checksum FROM schema_migrations WHERE version = $1`, name,
		).Scan(&recorded)
		switch {
		case err == nil:
			if recorded == "" {
				// 引入校验和之前应用过的迁移：补记当前内容，不重跑。
				if _, uerr := s.pool.Exec(ctx,
					`UPDATE schema_migrations SET checksum = $2 WHERE version = $1`,
					name, checksum,
				); uerr != nil {
					return applied, fmt.Errorf("补记迁移校验和 %s 失败: %w", name, uerr)
				}
				continue
			}
			if recorded != checksum {
				return applied, fmt.Errorf(
					"迁移文件 %s 在应用后被修改（记录 %s，当前 %s）；请新增一个迁移文件而不是改历史文件",
					name, recorded[:12], checksum[:12],
				)
			}
			continue
		case errors.Is(err, pgx.ErrNoRows):
			// 未应用，继续走下面的执行流程。
		default:
			return applied, fmt.Errorf("查询迁移状态 %s 失败: %w", name, err)
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
			`INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)`, name, checksum,
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
