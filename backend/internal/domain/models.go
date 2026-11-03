// Package domain 定义领域模型，是数据库行与 HTTP 响应之间的唯一契约。
package domain

import "time"

// Author 是文章作者。
type Author struct {
	ID        int64  `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
}

// Tag 是文章标签，PostCount 仅在列表查询时填充。
type Tag struct {
	ID          int64  `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description,omitempty"`
	PostCount   int    `json:"post_count"`
}

// Post 是一篇博客文章。BodyMD 仅在详情接口返回，列表接口会省略以减小载荷。
type Post struct {
	ID             int64      `json:"id"`
	Slug           string     `json:"slug"`
	Title          string     `json:"title"`
	Summary        string     `json:"summary"`
	BodyMD         string     `json:"body_md,omitempty"`
	CoverColor     string     `json:"cover_color"`
	CoverURL       string     `json:"cover_url"`
	Status         string     `json:"status"`
	Featured       bool       `json:"featured"`
	ReadingMinutes int        `json:"reading_minutes"`
	Views          int        `json:"views"`
	Author         *Author    `json:"author,omitempty"`
	Tags           []Tag      `json:"tags"`
	PublishedAt    *time.Time `json:"published_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Page 是独立页面，例如「关于」。
type Page struct {
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	BodyMD    string    `json:"body_md"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Stats 是站点维度的聚合统计。
type Stats struct {
	Posts      int        `json:"posts"`
	Tags       int        `json:"tags"`
	Chars      int        `json:"chars"`
	LastPostAt *time.Time `json:"last_published_at"`
}

// Social 是站点对外链接。
type Social struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// Site 是前端首屏需要的全部站点级信息，一次请求取回。
type Site struct {
	Name        string   `json:"name"`
	Tagline     string   `json:"tagline"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Author      *Author  `json:"author"`
	Socials     []Social `json:"socials"`
	// Texts 是后台可编辑的短文案（首页大图标题、图集标题等）。
	Texts    map[string]string `json:"texts"`
	Stats    Stats             `json:"stats"`
	Tags     []Tag             `json:"tags"`
	Featured *Post             `json:"featured,omitempty"`
	Recent   []Post            `json:"recent"`
}

// SettingAdminMeta 是 settings 表里存放后台运维元数据的键。
const SettingAdminMeta = "admin_meta"

// AdminMeta 记录后台账号的运维元数据。
//
// 目前只有 PasswordUpdatedAt 一个字段：在后台改过密码之后，启动时就不再
// 拿环境变量里的 ADMIN_PASSWORD 去和数据库比对，免得每次重启都打一条
// 「密码不一致」的误导性警告（环境变量里留着旧值本来就很正常）。
type AdminMeta struct {
	PasswordUpdatedAt string `json:"password_updated_at,omitempty"`
}

// Comment 是文章下的一条读者评论。
//
// Email 只出现在后台接口，公开接口永远不会带上它；IPHash 干脆不进 JSON，
// 只用于后台判断是不是同一来源在刷屏。
type Comment struct {
	ID        int64     `json:"id"`
	PostSlug  string    `json:"post_slug,omitempty"`
	Author    string    `json:"author"`
	Email     string    `json:"email,omitempty"`
	Body      string    `json:"body"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// 评论状态。pending 等待站长审核，published 公开，hidden 是站长隐藏的。
const (
	CommentPending   = "pending"
	CommentPublished = "published"
	CommentHidden    = "hidden"
)

// MonthCount 用于归档页的按年月聚合。
type MonthCount struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Count int `json:"count"`
}

// ArchiveYear 是按年份分组后的归档。
type ArchiveYear struct {
	Year   int          `json:"year"`
	Count  int          `json:"count"`
	Months []MonthCount `json:"months"`
}
