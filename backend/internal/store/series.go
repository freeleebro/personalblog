package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/freeleebro/personalblog/backend/internal/domain"
)

// seriesSelect 是系列的基础投影。
//
// post_count 只统计「已发布且已到发布时间」的文章：草稿和定时到未来的文章
// 不能出现在公开列表的计数里，否则读者点进去会发现少了几篇。
const seriesSelect = `
SELECT
    s.id, s.slug, s.title, s.description, s.created_at, s.updated_at,
    COALESCE(c.n, 0) AS post_count
FROM series s
LEFT JOIN LATERAL (
    SELECT count(*)::int AS n
    FROM posts p
    WHERE p.series_id = s.id
      AND p.status = 'published'
      AND p.published_at IS NOT NULL
      AND p.published_at <= now()
) c ON TRUE
`

// ListSeries 返回系列列表。publicOnly 为 true 时只返回至少有一篇已发布文章的系列，
// 避免前台出现点进去空无一物的合集；后台需要看到全部（含刚建好的空系列）。
func (s *Store) ListSeries(ctx context.Context, publicOnly bool) ([]domain.Series, error) {
	sql := seriesSelect
	if publicOnly {
		sql += " WHERE c.n > 0"
	}
	sql += " ORDER BY s.created_at DESC, s.id DESC"

	rows, err := s.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("查询系列列表失败: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Series, 0, 8)
	for rows.Next() {
		var item domain.Series
		if err := rows.Scan(
			&item.ID, &item.Slug, &item.Title, &item.Description,
			&item.CreatedAt, &item.UpdatedAt, &item.PostCount,
		); err != nil {
			return nil, fmt.Errorf("解析系列失败: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// GetSeriesBySlug 返回系列详情与它的文章列表。
//
// 列表按 series_order 升序，序号相同的按发布时间升序——这样站长只填一部分
// 序号时，没填的那些也不会乱序，而是退回「按时间先后」的默认阅读顺序。
func (s *Store) GetSeriesBySlug(ctx context.Context, slug string) (*domain.Series, []domain.Post, error) {
	var item domain.Series
	err := s.pool.QueryRow(ctx, seriesSelect+" WHERE s.slug = $1", slug).Scan(
		&item.ID, &item.Slug, &item.Title, &item.Description,
		&item.CreatedAt, &item.UpdatedAt, &item.PostCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("查询系列 %q 失败: %w", slug, err)
	}

	rows, err := s.pool.Query(ctx, postSelect+`
        WHERE p.series_id = $1
          AND p.status = 'published'
          AND p.published_at IS NOT NULL
          AND p.published_at <= now()
        ORDER BY p.series_order ASC, p.published_at ASC, p.id ASC`, item.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("查询系列文章失败: %w", err)
	}
	defer rows.Close()

	posts := make([]domain.Post, 0, item.PostCount)
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("解析系列文章失败: %w", err)
		}
		p.BodyMD = ""
		posts = append(posts, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return &item, posts, nil
}

// UpsertSeries 按 slug 幂等写入系列。改标题不会改 slug——已经有文章指向它了。
func (s *Store) UpsertSeries(ctx context.Context, in SeriesInput) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
        INSERT INTO series (slug, title, description)
        VALUES ($1, $2, $3)
        ON CONFLICT (slug) DO UPDATE
            SET title       = EXCLUDED.title,
                description = EXCLUDED.description,
                updated_at  = now()
        RETURNING id`,
		in.Slug, in.Title, in.Description,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("写入系列 %q 失败: %w", in.Slug, err)
	}
	return id, nil
}

// DeleteSeries 删除系列。成员文章的 series_id 由外键的 ON DELETE SET NULL 清空，
// 文章本身不受影响。
func (s *Store) DeleteSeries(ctx context.Context, slug string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM series WHERE slug = $1`, slug)
	if err != nil {
		return fmt.Errorf("删除系列 %q 失败: %w", slug, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
