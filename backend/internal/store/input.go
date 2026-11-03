package store

import "time"

// 写入路径的入参类型，由管理后台使用。

// AuthorInput 是作者写入入参。
type AuthorInput struct {
	Slug      string
	Name      string
	AvatarURL string
	Bio       string
}

// TagInput 是标签写入入参。
type TagInput struct {
	Slug        string
	Name        string
	Color       string
	Description string
}

// PostInput 是文章写入入参。TagIDs 会整体覆盖该文章现有的标签关联。
type PostInput struct {
	Slug           string
	Title          string
	Summary        string
	BodyMD         string
	CoverColor     string
	CoverURL       string
	Status         string
	Featured       bool
	ReadingMinutes int
	AuthorID       *int64
	TagIDs         []int64
	PublishedAt    *time.Time
}
