package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/freeleebro/personalblog/backend/internal/domain"
)

// postSelect 是所有文章查询共用的投影。
// 标签用 LATERAL + json_agg 一次性聚合成 JSON，避免 N+1 查询。
const postSelect = `
SELECT
    p.id, p.slug, p.title, p.summary, p.body_md, p.cover_color, p.cover_url, p.status,
    p.featured, p.reading_minutes, p.views, p.published_at, p.created_at, p.updated_at,
    a.id, a.slug, a.name, a.avatar_url, a.bio,
    COALESCE(agg.tags, '[]'::json) AS tags
FROM posts p
LEFT JOIN authors a ON a.id = p.author_id
LEFT JOIN LATERAL (
    SELECT json_agg(
               json_build_object(
                   'id',          t.id,
                   'name',        t.name,
                   'slug',        t.slug,
                   'color',       t.color,
                   'description', t.description
               ) ORDER BY t.name
           ) AS tags
    FROM post_tags pt
    JOIN tags t ON t.id = pt.tag_id
    WHERE pt.post_id = p.id
) agg ON TRUE
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPost(row rowScanner) (*domain.Post, error) {
	var (
		p        domain.Post
		aID      *int64
		aSlug    *string
		aName    *string
		aAvatar  *string
		aBio     *string
		tagsJSON []byte
	)

	if err := row.Scan(
		&p.ID, &p.Slug, &p.Title, &p.Summary, &p.BodyMD, &p.CoverColor, &p.CoverURL, &p.Status,
		&p.Featured, &p.ReadingMinutes, &p.Views, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
		&aID, &aSlug, &aName, &aAvatar, &aBio,
		&tagsJSON,
	); err != nil {
		return nil, err
	}

	if aID != nil {
		p.Author = &domain.Author{
			ID:        *aID,
			Slug:      deref(aSlug),
			Name:      deref(aName),
			AvatarURL: deref(aAvatar),
			Bio:       deref(aBio),
		}
	}
	if len(tagsJSON) > 0 {
		if err := json.Unmarshal(tagsJSON, &p.Tags); err != nil {
			return nil, fmt.Errorf("解析 tags JSON 失败: %w", err)
		}
	}
	if p.Tags == nil {
		p.Tags = []domain.Tag{}
	}
	return &p, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// PostFilter 描述列表查询的过滤条件。零值表示「不过滤」。
type PostFilter struct {
	TagSlug  string
	Query    string
	Year     int
	Featured *bool
	Status   string // 空字符串等价于 published
	Page     int
	PerPage  int
}

// PostPage 是分页结果。
type PostPage struct {
	Items   []domain.Post
	Total   int
	Page    int
	PerPage int
}

// TotalPages 返回总页数，至少为 1。
func (p PostPage) TotalPages() int {
	if p.PerPage <= 0 {
		return 1
	}
	pages := (p.Total + p.PerPage - 1) / p.PerPage
	if pages < 1 {
		return 1
	}
	return pages
}

// escapeLike 转义 LIKE 的通配符，避免用户输入 "%" 变成全表匹配。
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func buildPostWhere(f PostFilter) (string, []any) {
	args := make([]any, 0, 6)
	conds := make([]string, 0, 6)

	status := f.Status
	if status == "" {
		status = "published"
	}
	args = append(args, status)
	conds = append(conds, fmt.Sprintf("p.status = $%d", len(args)))

	// 只暴露「已到发布时间」的文章。
	// 有了这一条，后台填未来时间就等于定时发布；管理员列表走
	// ListPostsAnyStatus，不受影响。
	conds = append(conds, "p.published_at IS NOT NULL AND p.published_at <= now()")

	if f.TagSlug != "" {
		args = append(args, f.TagSlug)
		conds = append(conds, fmt.Sprintf(`EXISTS (
            SELECT 1 FROM post_tags pt_f
            JOIN tags t_f ON t_f.id = pt_f.tag_id
            WHERE pt_f.post_id = p.id AND t_f.slug = $%d)`, len(args)))
	}

	if f.Query != "" {
		args = append(args, "%"+escapeLike(f.Query)+"%")
		// 表达式必须与 0004 里的 gin 索引完全一致，否则用不上索引。
		conds = append(conds, fmt.Sprintf(
			`(coalesce(p.title, '') || ' ' || coalesce(p.summary, '') || ' ' || coalesce(p.body_md, ''))
             ILIKE $%[1]d ESCAPE '\'`, len(args)))
	}

	if f.Year > 0 {
		args = append(args, f.Year)
		conds = append(conds, fmt.Sprintf("EXTRACT(YEAR FROM p.published_at) = $%d", len(args)))
	}

	if f.Featured != nil {
		args = append(args, *f.Featured)
		conds = append(conds, fmt.Sprintf("p.featured = $%d", len(args)))
	}

	return strings.Join(conds, " AND "), args
}

func clampInt(v, min, max, fallback int) int {
	if v == 0 {
		v = fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ListPosts 返回分页后的文章列表，附带总数。
func (s *Store) ListPosts(ctx context.Context, f PostFilter) (*PostPage, error) {
	where, args := buildPostWhere(f)
	perPage := clampInt(f.PerPage, 1, 50, 10)
	page := clampInt(f.Page, 1, 1_000_000, 1)

	var total int
	countSQL := "SELECT count(*) FROM posts p WHERE " + where
	if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("统计文章总数失败: %w", err)
	}

	listArgs := append(append([]any{}, args...), perPage, (page-1)*perPage)
	sql := postSelect +
		" WHERE " + where +
		fmt.Sprintf(" ORDER BY p.published_at DESC NULLS LAST, p.id DESC LIMIT $%d OFFSET $%d",
			len(args)+1, len(args)+2)

	rows, err := s.pool.Query(ctx, sql, listArgs...)
	if err != nil {
		return nil, fmt.Errorf("查询文章列表失败: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Post, 0, perPage)
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, fmt.Errorf("解析文章失败: %w", err)
		}
		items = append(items, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历文章失败: %w", err)
	}

	return &PostPage{Items: items, Total: total, Page: page, PerPage: perPage}, nil
}

// GetPostBySlug 返回单篇文章（含正文 Markdown），仅限已发布的公开文章。
// 草稿和「定时到未来」的文章不会出现在这里，后台请用 GetPostBySlugAnyStatus。
func (s *Store) GetPostBySlug(ctx context.Context, slug string) (*domain.Post, error) {
	row := s.pool.QueryRow(ctx, postSelect+`
        WHERE p.slug = $1
          AND p.status = 'published'
          AND p.published_at IS NOT NULL
          AND p.published_at <= now()`, slug)
	p, err := scanPost(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询文章 %q 失败: %w", slug, err)
	}
	return p, nil
}

// GetPostBySlugAnyStatus 供管理后台使用：不限制状态与发布时间。
func (s *Store) GetPostBySlugAnyStatus(ctx context.Context, slug string) (*domain.Post, error) {
	row := s.pool.QueryRow(ctx, postSelect+" WHERE p.slug = $1", slug)
	p, err := scanPost(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询文章 %q 失败: %w", slug, err)
	}
	return p, nil
}

// RelatedPosts 找出与该文章共享标签的其他文章，按发布时间倒序。
func (s *Store) RelatedPosts(ctx context.Context, postID int64, limit int) ([]domain.Post, error) {
	if limit <= 0 {
		limit = 3
	}
	sql := postSelect + `
WHERE p.status = 'published' AND p.id <> $1
  AND p.published_at IS NOT NULL AND p.published_at <= now()
  AND EXISTS (
      SELECT 1
      FROM post_tags pt_self
      WHERE pt_self.post_id = p.id
        AND pt_self.tag_id IN (SELECT tag_id FROM post_tags WHERE post_id = $1)
  )
ORDER BY p.published_at DESC NULLS LAST
LIMIT $2`

	rows, err := s.pool.Query(ctx, sql, postID, limit)
	if err != nil {
		return nil, fmt.Errorf("查询相关文章失败: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Post, 0, limit)
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, fmt.Errorf("解析相关文章失败: %w", err)
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// IncrementViews 原子自增阅读量并返回最新值。
func (s *Store) IncrementViews(ctx context.Context, slug string) (int, error) {
	var views int
	err := s.pool.QueryRow(ctx,
		`UPDATE posts SET views = views + 1
         WHERE slug = $1 AND status = 'published'
           AND published_at IS NOT NULL AND published_at <= now()
         RETURNING views`,
		slug,
	).Scan(&views)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("更新阅读量失败: %w", err)
	}
	return views, nil
}

// Neighbours 返回按发布时间排序的上一篇（更早）/ 下一篇（更晚）。
// 注意方向：prev 取更早的文章，next 取更晚的文章，与阅读顺序一致。
func (s *Store) Neighbours(ctx context.Context, publishedAt time.Time, postID int64) (prev, next *domain.Post, err error) {
	prevRow := s.pool.QueryRow(ctx, postSelect+`
        WHERE p.status = 'published'
          AND p.published_at IS NOT NULL AND p.published_at <= now()
          AND (p.published_at, p.id) < ($1, $2)
        ORDER BY p.published_at DESC, p.id DESC
        LIMIT 1`, publishedAt, postID)
	if prev, err = scanPost(prevRow); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, fmt.Errorf("查询上一篇失败: %w", err)
	} else if errors.Is(err, pgx.ErrNoRows) {
		prev = nil
	}

	nextRow := s.pool.QueryRow(ctx, postSelect+`
        WHERE p.status = 'published'
          AND p.published_at IS NOT NULL AND p.published_at <= now()
          AND (p.published_at, p.id) > ($1, $2)
        ORDER BY p.published_at ASC, p.id ASC
        LIMIT 1`, publishedAt, postID)
	if next, err = scanPost(nextRow); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, fmt.Errorf("查询下一篇失败: %w", err)
	} else if errors.Is(err, pgx.ErrNoRows) {
		next = nil
	}

	return prev, next, nil
}

// PostCountByYear 返回某年的文章数，供归档页使用。
func (s *Store) PostCountByYear(ctx context.Context) ([]domain.ArchiveYear, error) {
	rows, err := s.pool.Query(ctx, `
        SELECT EXTRACT(YEAR FROM published_at)::int  AS year,
               EXTRACT(MONTH FROM published_at)::int AS month,
               count(*)::int                         AS count
        FROM posts
        WHERE status = 'published' AND published_at IS NOT NULL AND published_at <= now()
        GROUP BY 1, 2
        ORDER BY 1 DESC, 2 DESC`)
	if err != nil {
		return nil, fmt.Errorf("查询归档失败: %w", err)
	}
	defer rows.Close()

	years := make([]domain.ArchiveYear, 0, 8)
	index := make(map[int]int)
	for rows.Next() {
		var y, m, c int
		if err := rows.Scan(&y, &m, &c); err != nil {
			return nil, fmt.Errorf("解析归档失败: %w", err)
		}
		pos, ok := index[y]
		if !ok {
			years = append(years, domain.ArchiveYear{Year: y})
			pos = len(years) - 1
			index[y] = pos
		}
		years[pos].Months = append(years[pos].Months, domain.MonthCount{Year: y, Month: m, Count: c})
		years[pos].Count += c
	}
	return years, rows.Err()
}

// Stats 返回站点聚合统计。
func (s *Store) Stats(ctx context.Context) (domain.Stats, error) {
	var st domain.Stats
	err := s.pool.QueryRow(ctx,
		`SELECT post_count, tag_count, total_chars, last_published_at FROM site_overview`,
	).Scan(&st.Posts, &st.Tags, &st.Chars, &st.LastPostAt)
	if err != nil {
		return st, fmt.Errorf("查询站点统计失败: %w", err)
	}
	return st, nil
}

// ListPostsAnyStatus 供管理后台使用：返回任意状态的文章。
// filter.Status == "*" 表示不过滤状态，否则按指定状态过滤。
func (s *Store) ListPostsAnyStatus(ctx context.Context, f PostFilter) (*PostPage, error) {
	perPage := clampInt(f.PerPage, 1, 100, 100)
	page := clampInt(f.Page, 1, 1_000_000, 1)

	where := ""
	args := []any{}
	if f.Status != "*" && f.Status != "" {
		args = append(args, f.Status)
		where = " WHERE p.status = $1"
	}

	var total int
	countSQL := "SELECT count(*) FROM posts p" + where
	if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("统计文章总数失败: %w", err)
	}

	listArgs := append(append([]any{}, args...), perPage, (page-1)*perPage)
	sql := postSelect + where +
		" ORDER BY p.updated_at DESC, p.id DESC" +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)

	rows, err := s.pool.Query(ctx, sql, listArgs...)
	if err != nil {
		return nil, fmt.Errorf("查询文章列表失败: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Post, 0, perPage)
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, fmt.Errorf("解析文章失败: %w", err)
		}
		items = append(items, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历文章失败: %w", err)
	}
	return &PostPage{Items: items, Total: total, Page: page, PerPage: perPage}, nil
}
