package store

import (
	"context"
	"fmt"
	"time"
)

// PageView 是一条待写入的访问记录。
type PageView struct {
	Path         string
	ReferrerHost string
	Visitor      string
	IsBot        bool
}

// StatsPoint 是某一天的 PV / UV。
type StatsPoint struct {
	Date string `json:"date"`
	PV   int    `json:"pv"`
	UV   int    `json:"uv"`
}

// TopItem 是排行榜里的一行。
type TopItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// StatsSummary 是后台「统计」页需要的全部数字。
type StatsSummary struct {
	Days         int          `json:"days"`
	PV           int          `json:"pv"`
	UV           int          `json:"uv"`
	Bots         int          `json:"bots"`
	Daily        []StatsPoint `json:"daily"`
	TopPages     []TopItem    `json:"top_pages"`
	TopReferrers []TopItem    `json:"top_referrers"`
}

// RecordPageView 写入一条访问记录。
func (s *Store) RecordPageView(ctx context.Context, v PageView) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO page_views (path, referrer_host, visitor, is_bot)
		VALUES ($1, $2, $3, $4)`, v.Path, v.ReferrerHost, v.Visitor, v.IsBot)
	if err != nil {
		return fmt.Errorf("写入访问记录失败: %w", err)
	}
	return nil
}

// StatsSummary 汇总最近 days 天的 PV/UV、热门页面和来源。
// tz 用于按站点时区切分自然日，例如 "Asia/Shanghai"。
func (s *Store) StatsSummary(ctx context.Context, days int, tz string) (*StatsSummary, error) {
	if days < 1 {
		days = 7
	}
	since := time.Now().AddDate(0, 0, -days)

	summary := &StatsSummary{Days: days, Daily: []StatsPoint{}, TopPages: []TopItem{}, TopReferrers: []TopItem{}}

	if err := s.pool.QueryRow(ctx, `
		SELECT count(*), count(DISTINCT visitor)
		FROM page_views
		WHERE created_at >= $1 AND NOT is_bot`, since).Scan(&summary.PV, &summary.UV); err != nil {
		return nil, fmt.Errorf("统计访问量失败: %w", err)
	}

	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM page_views
		WHERE created_at >= $1 AND is_bot`, since).Scan(&summary.Bots); err != nil {
		return nil, fmt.Errorf("统计爬虫访问失败: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT to_char(created_at AT TIME ZONE $2, 'YYYY-MM-DD') AS day,
		       count(*), count(DISTINCT visitor)
		FROM page_views
		WHERE created_at >= $1 AND NOT is_bot
		GROUP BY day
		ORDER BY day`, since, tz)
	if err != nil {
		return nil, fmt.Errorf("按天统计失败: %w", err)
	}
	defer rows.Close()

	byDay := make(map[string]StatsPoint, days)
	for rows.Next() {
		var p StatsPoint
		if err := rows.Scan(&p.Date, &p.PV, &p.UV); err != nil {
			return nil, fmt.Errorf("读取按天统计失败: %w", err)
		}
		byDay[p.Date] = p
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取按天统计失败: %w", err)
	}

	// 补齐没有访问的日期，前端画柱状图时不会缺柱子。
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	today := time.Now().In(loc)
	for i := days - 1; i >= 0; i-- {
		day := today.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := byDay[day]; ok {
			summary.Daily = append(summary.Daily, p)
		} else {
			summary.Daily = append(summary.Daily, StatsPoint{Date: day})
		}
	}

	pages, err := s.topItems(ctx, `
		SELECT path, count(*) AS c FROM page_views
		WHERE created_at >= $1 AND NOT is_bot
		GROUP BY path ORDER BY c DESC, path ASC LIMIT 8`, since)
	if err != nil {
		return nil, err
	}
	summary.TopPages = pages

	refs, err := s.topItems(ctx, `
		SELECT referrer_host, count(*) AS c FROM page_views
		WHERE created_at >= $1 AND NOT is_bot AND referrer_host <> ''
		GROUP BY referrer_host ORDER BY c DESC, referrer_host ASC LIMIT 8`, since)
	if err != nil {
		return nil, err
	}
	summary.TopReferrers = refs

	return summary, nil
}

func (s *Store) topItems(ctx context.Context, query string, args ...any) ([]TopItem, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("统计排行失败: %w", err)
	}
	defer rows.Close()

	items := make([]TopItem, 0, 8)
	for rows.Next() {
		var it TopItem
		if err := rows.Scan(&it.Name, &it.Count); err != nil {
			return nil, fmt.Errorf("读取排行失败: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取排行失败: %w", err)
	}
	return items, nil
}

// PurgePageViews 删除 olderThan 之前的记录，返回删除条数。
func (s *Store) PurgePageViews(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM page_views WHERE created_at < $1`, time.Now().Add(-olderThan))
	if err != nil {
		return 0, fmt.Errorf("清理访问记录失败: %w", err)
	}
	return tag.RowsAffected(), nil
}
