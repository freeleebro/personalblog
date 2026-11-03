package store

import (
	"context"
	"encoding/json"
	"fmt"
)

// UpsertAuthor 按 slug 幂等写入作者。
func (s *Store) UpsertAuthor(ctx context.Context, a AuthorInput) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
        INSERT INTO authors (slug, name, avatar_url, bio)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (slug) DO UPDATE
            SET name = EXCLUDED.name,
                avatar_url = EXCLUDED.avatar_url,
                bio = EXCLUDED.bio
        RETURNING id`,
		a.Slug, a.Name, a.AvatarURL, a.Bio,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("写入作者 %q 失败: %w", a.Slug, err)
	}
	return id, nil
}

// UpsertTag 按 slug 幂等写入标签。
func (s *Store) UpsertTag(ctx context.Context, t TagInput) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
        INSERT INTO tags (slug, name, color, description)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (slug) DO UPDATE
            SET name = EXCLUDED.name,
                color = EXCLUDED.color,
                description = EXCLUDED.description
        RETURNING id`,
		t.Slug, t.Name, t.Color, t.Description,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("写入标签 %q 失败: %w", t.Slug, err)
	}
	return id, nil
}

// UpsertPost 按 slug 幂等写入文章，并整体重置其标签关联。
func (s *Store) UpsertPost(ctx context.Context, p PostInput) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	err = tx.QueryRow(ctx, `
        INSERT INTO posts (
            slug, title, summary, body_md, cover_color, cover_url, status, featured,
            reading_minutes, author_id, published_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        ON CONFLICT (slug) DO UPDATE
            SET title           = EXCLUDED.title,
                summary         = EXCLUDED.summary,
                body_md         = EXCLUDED.body_md,
                cover_color     = EXCLUDED.cover_color,
                cover_url       = EXCLUDED.cover_url,
                status          = EXCLUDED.status,
                featured        = EXCLUDED.featured,
                reading_minutes = EXCLUDED.reading_minutes,
                author_id       = EXCLUDED.author_id,
                published_at    = EXCLUDED.published_at
        RETURNING id`,
		p.Slug, p.Title, p.Summary, p.BodyMD, p.CoverColor, p.CoverURL, p.Status, p.Featured,
		p.ReadingMinutes, p.AuthorID, p.PublishedAt,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("写入文章 %q 失败: %w", p.Slug, err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM post_tags WHERE post_id = $1`, id); err != nil {
		return 0, fmt.Errorf("清理文章标签失败: %w", err)
	}
	for _, tagID := range p.TagIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO post_tags (post_id, tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			id, tagID,
		); err != nil {
			return 0, fmt.Errorf("关联标签失败: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("提交文章 %q 失败: %w", p.Slug, err)
	}
	return id, nil
}

// UpsertPage 按 slug 幂等写入独立页面。
func (s *Store) UpsertPage(ctx context.Context, slug, title, bodyMD string) error {
	_, err := s.pool.Exec(ctx, `
        INSERT INTO pages (slug, title, body_md)
        VALUES ($1, $2, $3)
        ON CONFLICT (slug) DO UPDATE
            SET title = EXCLUDED.title,
                body_md = EXCLUDED.body_md`,
		slug, title, bodyMD)
	if err != nil {
		return fmt.Errorf("写入页面 %q 失败: %w", slug, err)
	}
	return nil
}

// PutSetting 以 JSONB 形式写入一条配置。
func (s *Store) PutSetting(ctx context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化设置 %q 失败: %w", key, err)
	}
	_, err = s.pool.Exec(ctx, `
        INSERT INTO settings (key, value)
        VALUES ($1, $2::jsonb)
        ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, string(raw))
	if err != nil {
		return fmt.Errorf("写入设置 %q 失败: %w", key, err)
	}
	return nil
}
