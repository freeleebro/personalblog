package store

import (
	"context"
	"fmt"

	"github.com/freeleebro/personalblog/backend/internal/domain"
)

// commentSelect 是评论查询共用的投影。
const commentSelect = `
SELECT c.id, c.post_slug, c.author, c.email, c.body, c.status, c.created_at
FROM comments c
`

func scanComment(row rowScanner) (*domain.Comment, error) {
	var c domain.Comment
	if err := row.Scan(&c.ID, &c.PostSlug, &c.Author, &c.Email, &c.Body, &c.Status, &c.CreatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}

// ListPublishedComments 返回某篇文章下已通过的评论，按时间正序。
func (s *Store) ListPublishedComments(ctx context.Context, slug string, limit int) ([]domain.Comment, error) {
	rows, err := s.pool.Query(ctx, commentSelect+`
		WHERE c.post_slug = $1 AND c.status = $2
		ORDER BY c.created_at ASC, c.id ASC
		LIMIT $3`, slug, domain.CommentPublished, limit)
	if err != nil {
		return nil, fmt.Errorf("查询评论失败: %w", err)
	}
	defer rows.Close()

	list := make([]domain.Comment, 0, 8)
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, fmt.Errorf("读取评论失败: %w", err)
		}
		list = append(list, *c)
	}
	return list, rows.Err()
}

// CountPublishedComments 返回某篇文章已通过的评论数。
func (s *Store) CountPublishedComments(ctx context.Context, slug string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM comments WHERE post_slug = $1 AND status = $2`,
		slug, domain.CommentPublished,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("统计评论失败: %w", err)
	}
	return n, nil
}

// CreateComment 写入一条评论，状态由调用方决定。
func (s *Store) CreateComment(ctx context.Context, c *domain.Comment, ipHash string) error {
	return s.pool.QueryRow(ctx, `
		INSERT INTO comments (post_slug, author, email, body, status, ip_hash)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`,
		c.PostSlug, c.Author, c.Email, c.Body, c.Status, ipHash,
	).Scan(&c.ID, &c.CreatedAt)
}

// AdminListComments 按状态列出评论，status 为空表示全部。
func (s *Store) AdminListComments(ctx context.Context, status string, limit int) ([]domain.Comment, error) {
	query := commentSelect
	args := []any{}
	if status != "" {
		query += ` WHERE c.status = $1`
		args = append(args, status)
	}
	query += fmt.Sprintf(` ORDER BY c.created_at DESC, c.id DESC LIMIT $%d`, len(args)+1)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询评论失败: %w", err)
	}
	defer rows.Close()

	list := make([]domain.Comment, 0, 16)
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, fmt.Errorf("读取评论失败: %w", err)
		}
		list = append(list, *c)
	}
	return list, rows.Err()
}

// CommentCounts 返回各状态的评论数量，用于后台角标。
func (s *Store) CommentCounts(ctx context.Context) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT status, count(*) FROM comments GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("统计评论失败: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{
		domain.CommentPending:   0,
		domain.CommentPublished: 0,
		domain.CommentHidden:    0,
	}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, fmt.Errorf("读取评论统计失败: %w", err)
		}
		counts[status] = n
	}
	return counts, rows.Err()
}

// SetCommentStatus 修改评论状态。返回 false 表示评论不存在。
func (s *Store) SetCommentStatus(ctx context.Context, id int64, status string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE comments SET status = $2 WHERE id = $1`, id, status)
	if err != nil {
		return false, fmt.Errorf("更新评论状态失败: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// DeleteComment 删除评论。返回 false 表示评论不存在。
func (s *Store) DeleteComment(ctx context.Context, id int64) (bool, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM comments WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("删除评论失败: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
