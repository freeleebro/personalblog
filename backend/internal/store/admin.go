package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// AdminUser 是管理后台账号。
type AdminUser struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	DisplayName  string `json:"display_name"`
}

// GetAdminUserByUsername 按用户名查询后台账号。
func (s *Store) GetAdminUserByUsername(ctx context.Context, username string) (*AdminUser, error) {
	var u AdminUser
	err := s.pool.QueryRow(ctx,
		`SELECT id, username, password_hash, display_name FROM admin_users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询管理员 %q 失败: %w", username, err)
	}
	return &u, nil
}

// CountAdminUsers 返回后台账号数量，用于判断是否需要创建初始账号。
func (s *Store) CountAdminUsers(ctx context.Context) (int, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM admin_users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("统计管理员数量失败: %w", err)
	}
	return n, nil
}

// UpsertAdminUser 创建或更新后台账号（密码为空则不更新哈希）。
func (s *Store) UpsertAdminUser(ctx context.Context, username, passwordHash, displayName string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
        INSERT INTO admin_users (username, password_hash, display_name)
        VALUES ($1, $2, $3)
        ON CONFLICT (username) DO UPDATE
            SET password_hash = CASE WHEN $2 = '' THEN admin_users.password_hash ELSE EXCLUDED.password_hash END,
                display_name  = EXCLUDED.display_name
        RETURNING id`,
		username, passwordHash, displayName,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("写入管理员 %q 失败: %w", username, err)
	}
	return id, nil
}
