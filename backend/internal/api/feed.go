package api

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/freeleebro/personalblog/backend/internal/store"
)

// ---------------------------------------------------------------- RSS 2.0

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Atom    string     `xml:"xmlns:atom,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Language    string    `xml:"language"`
	LastBuild   string    `xml:"lastBuildDate"`
	AtomLink    atomLink  `xml:"atom:link"`
	Items       []rssItem `xml:"item"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type rssItem struct {
	Title   string   `xml:"title"`
	Link    string   `xml:"link"`
	GUID    string   `xml:"guid"`
	PubDate string   `xml:"pubDate"`
	Desc    string   `xml:"description"`
	Cats    []string `xml:"category"`
}

func (s *Server) handleFeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	base := s.siteBaseURL(ctx)
	name, _, description := s.siteMeta(ctx)

	page, err := s.store.ListPosts(ctx, store.PostFilter{PerPage: 20})
	if err != nil {
		s.fail(w, "生成 RSS 失败", err)
		return
	}

	feed := rssFeed{
		Version: "2.0",
		Atom:    "http://www.w3.org/2005/Atom",
		Channel: rssChannel{
			Title:       name,
			Link:        base,
			Description: description,
			Language:    "zh-CN",
			LastBuild:   time.Now().UTC().Format(time.RFC1123Z),
			AtomLink: atomLink{
				// 对外地址是 /feed.xml（由 web/app/feed.xml/route.ts 转发），
				// 不能把内部的 /api/v1/ 前缀暴露给阅读器。
				Href: base + "/feed.xml",
				Rel:  "self",
				Type: "application/rss+xml",
			},
		},
	}

	for _, p := range page.Items {
		published := p.CreatedAt
		if p.PublishedAt != nil {
			published = *p.PublishedAt
		}
		cats := make([]string, 0, len(p.Tags))
		for _, t := range p.Tags {
			cats = append(cats, t.Name)
		}
		feed.Channel.Items = append(feed.Channel.Items, rssItem{
			Title:   p.Title,
			Link:    fmt.Sprintf("%s/posts/%s", base, p.Slug),
			GUID:    fmt.Sprintf("%s/posts/%s", base, p.Slug),
			PubDate: published.Format(time.RFC1123Z),
			Desc:    p.Summary,
			Cats:    cats,
		})
	}

	body, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		s.fail(w, "序列化 RSS 失败", err)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=600")
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(body)
}

// ---------------------------------------------------------------- sitemap.xml

type urlSet struct {
	XMLName xml.Name   `xml:"urlset"`
	Xmlns   string     `xml:"xmlns,attr"`
	URLs    []urlEntry `xml:"url"`
}

type urlEntry struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	base := s.siteBaseURL(ctx)

	entries := []urlEntry{
		{Loc: base + "/"},
		{Loc: base + "/posts"},
		{Loc: base + "/tags"},
		{Loc: base + "/archive"},
		{Loc: base + "/about"},
	}

	page, err := s.store.ListPosts(ctx, store.PostFilter{PerPage: 50})
	if err != nil {
		s.fail(w, "生成 sitemap 失败", err)
		return
	}
	for _, p := range page.Items {
		entries = append(entries, urlEntry{
			Loc:     fmt.Sprintf("%s/posts/%s", base, p.Slug),
			LastMod: p.UpdatedAt.UTC().Format("2006-01-02"),
		})
	}

	tags, err := s.store.ListTags(ctx)
	if err != nil {
		s.fail(w, "生成 sitemap 失败", err)
		return
	}
	for _, t := range tags {
		entries = append(entries, urlEntry{Loc: fmt.Sprintf("%s/tags/%s", base, t.Slug)})
	}

	body, err := xml.MarshalIndent(urlSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  entries,
	}, "", "  ")
	if err != nil {
		s.fail(w, "序列化 sitemap 失败", err)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=600")
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(body)
}

// siteBaseURL 优先使用数据库里的站点地址，回退到环境变量。
func (s *Server) siteBaseURL(ctx context.Context) string {
	base := s.cfg.SiteURL
	var cfg struct {
		URL string `json:"url"`
	}
	if err := s.store.Setting(ctx, "site", &cfg); err != nil && !errors.Is(err, store.ErrNotFound) {
		s.log.Warn("读取站点地址失败，回退到环境变量", "error", err)
	}
	if strings.TrimSpace(cfg.URL) != "" {
		base = strings.TrimSpace(cfg.URL)
	}
	return strings.TrimRight(base, "/")
}

// siteMeta 读取后台里配置的站点名称 / 副标题 / 描述，读不到再回退到环境变量。
// RSS 这类对外产物应该跟后台看到的一致，而不是只认环境变量。
func (s *Server) siteMeta(ctx context.Context) (name, tagline, description string) {
	name, tagline, description = s.cfg.SiteName, s.cfg.SiteTagline, s.cfg.SiteDescription
	var cfg struct {
		Name        string `json:"name"`
		Tagline     string `json:"tagline"`
		Description string `json:"description"`
	}
	if err := s.store.Setting(ctx, "site", &cfg); err != nil && !errors.Is(err, store.ErrNotFound) {
		s.log.Warn("读取站点信息失败，回退到环境变量", "error", err)
	}
	if v := strings.TrimSpace(cfg.Name); v != "" {
		name = v
	}
	if v := strings.TrimSpace(cfg.Tagline); v != "" {
		tagline = v
	}
	if v := strings.TrimSpace(cfg.Description); v != "" {
		description = v
	}
	return name, tagline, description
}
