package store

import (
	"context"
	"fmt"

	"github.com/freeleebro/personalblog/backend/internal/domain"
)

// ExportPosts 返回全部文章（含正文），按创建时间升序，方便导出后按原顺序阅读。
// 只用于后台「导出全部」，不做分页。
func (s *Store) ExportPosts(ctx context.Context) ([]domain.Post, error) {
	rows, err := s.pool.Query(ctx, postSelect+" ORDER BY p.created_at ASC, p.id ASC")
	if err != nil {
		return nil, fmt.Errorf("导出查询文章失败: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Post, 0, 64)
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, fmt.Errorf("导出解析文章失败: %w", err)
		}
		items = append(items, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("导出遍历文章失败: %w", err)
	}
	return items, nil
}

// ExportPages 返回全部独立页面（关于等）。
func (s *Store) ExportPages(ctx context.Context) ([]domain.Page, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT slug, title, body_md, updated_at FROM pages ORDER BY slug`)
	if err != nil {
		return nil, fmt.Errorf("导出查询页面失败: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Page, 0, 8)
	for rows.Next() {
		var p domain.Page
		if err := rows.Scan(&p.Slug, &p.Title, &p.BodyMD, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("导出解析页面失败: %w", err)
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("导出遍历页面失败: %w", err)
	}
	return items, nil
}
