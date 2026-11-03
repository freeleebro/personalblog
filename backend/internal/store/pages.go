package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/freeleebro/personalblog/backend/internal/domain"
)

// GetPage 按 slug 读取独立页面（例如 about）。
func (s *Store) GetPage(ctx context.Context, slug string) (*domain.Page, error) {
	var p domain.Page
	err := s.pool.QueryRow(ctx,
		`SELECT slug, title, body_md, updated_at FROM pages WHERE slug = $1`, slug,
	).Scan(&p.Slug, &p.Title, &p.BodyMD, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询页面 %q 失败: %w", slug, err)
	}
	return &p, nil
}

// Setting 读取一条 JSONB 配置并反序列化到 dest。
// 配置缺失时返回 ErrNotFound，调用方自行决定回退值。
func (s *Store) Setting(ctx context.Context, key string, dest any) error {
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("读取设置 %q 失败: %w", key, err)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("解析设置 %q 失败: %w", key, err)
	}
	return nil
}

// GetAuthorBySlug 读取作者资料。
func (s *Store) GetAuthorBySlug(ctx context.Context, slug string) (*domain.Author, error) {
	var a domain.Author
	err := s.pool.QueryRow(ctx,
		`SELECT id, slug, name, avatar_url, bio FROM authors WHERE slug = $1`, slug,
	).Scan(&a.ID, &a.Slug, &a.Name, &a.AvatarURL, &a.Bio)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询作者 %q 失败: %w", slug, err)
	}
	return &a, nil
}
