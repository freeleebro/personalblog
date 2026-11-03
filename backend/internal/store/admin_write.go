package store

import (
	"context"
	"fmt"
)

// DeletePost 删除文章（post_tags 会因外键 ON DELETE CASCADE 一并清除）。
func (s *Store) DeletePost(ctx context.Context, slug string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM posts WHERE slug = $1`, slug)
	if err != nil {
		return fmt.Errorf("删除文章 %q 失败: %w", slug, err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteTag 删除标签（关联会级联清除）。
func (s *Store) DeleteTag(ctx context.Context, slug string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM tags WHERE slug = $1`, slug)
	if err != nil {
		return fmt.Errorf("删除标签 %q 失败: %w", slug, err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeletePage 删除独立页面。
func (s *Store) DeletePage(ctx context.Context, slug string) error {
	ct, err := s.pool.Exec(ctx, `DELETE FROM pages WHERE slug = $1`, slug)
	if err != nil {
		return fmt.Errorf("删除页面 %q 失败: %w", slug, err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
