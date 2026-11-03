package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/freeleebro/personalblog/backend/internal/domain"
)

const tagSelect = `
SELECT t.id, t.slug, t.name, t.color, t.description,
       count(p.id)::int AS post_count
FROM tags t
LEFT JOIN post_tags pt ON pt.tag_id = t.id
LEFT JOIN posts p ON p.id = pt.post_id AND p.status = 'published'
`

// ListTags 返回全部标签及其已发布文章数，按热度倒序。
func (s *Store) ListTags(ctx context.Context) ([]domain.Tag, error) {
	rows, err := s.pool.Query(ctx, tagSelect+`
        GROUP BY t.id
        ORDER BY post_count DESC, t.name ASC`)
	if err != nil {
		return nil, fmt.Errorf("查询标签列表失败: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Tag, 0, 16)
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Color, &t.Description, &t.PostCount); err != nil {
			return nil, fmt.Errorf("解析标签失败: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTagBySlug 返回单个标签。
func (s *Store) GetTagBySlug(ctx context.Context, slug string) (*domain.Tag, error) {
	var t domain.Tag
	err := s.pool.QueryRow(ctx, tagSelect+`
        WHERE t.slug = $1
        GROUP BY t.id`, slug,
	).Scan(&t.ID, &t.Slug, &t.Name, &t.Color, &t.Description, &t.PostCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询标签 %q 失败: %w", slug, err)
	}
	return &t, nil
}
