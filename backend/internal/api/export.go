package api

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/freeleebro/personalblog/backend/internal/domain"
	"github.com/freeleebro/personalblog/backend/internal/store"
)

// exportReadme 一起打进压缩包，说明怎么把内容导回去，避免半年后自己都忘了。
const exportReadme = `# 博客内容导出

这个压缩包是后台「导出全部内容」生成的，包含：

- posts/<slug>.md   每篇文章，front matter 里带标题、slug、摘要、状态、发布时间、标签
- pages/<slug>.md   独立页面（关于等）
- media/...         上传过的图片，路径与站内 /media/... 一一对应
- posts.json        所有文章的结构化数据（含正文），方便自己写脚本处理
- site.json         站点信息与自定义文案

## 怎么导回去

1. 后台 → 内容 → 「导入 Markdown 文件」，选 posts/ 里的 .md，逐篇导入后保存；
   front matter 里的 tags 会按标签名自动匹配已有标签。
2. 图片不会自动重新上传：把 media/ 里的文件按原路径放回服务器的上传目录
   （Docker 部署时是 beichuang-media 卷里的 /data/uploads），站内引用即可继续生效。
3. 想换一套博客系统时，直接拿 posts.json 或 .md 文件迁移即可，正文是标准 Markdown。
`

// maxExportMediaBytes 限制单个媒体文件大小，防止误传超大文件把导出拖垮。
const maxExportMediaBytes = 30 << 20

// handleAdminExport 打包全部内容为 zip：文章、页面、媒体文件、结构化 JSON。
// 数据量小的个人博客直接流式写 zip，不需要落临时文件。
func (s *Server) handleAdminExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	posts, err := s.store.ExportPosts(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export_failed", err.Error())
		return
	}
	pages, err := s.store.ExportPages(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export_failed", err.Error())
		return
	}

	var site domain.Site
	if err := s.store.Setting(ctx, "site", &site); err != nil && err != store.ErrNotFound {
		s.log.Warn("导出时读取站点设置失败", "err", err)
	}

	stamp := time.Now().UTC().Format("20060102-150405")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="blog-export-%s.zip"`, stamp))
	w.Header().Set("Cache-Control", "no-store")

	zw := zip.NewWriter(w)
	defer func() {
		if err := zw.Close(); err != nil {
			s.log.Warn("关闭导出压缩包失败", "err", err)
		}
	}()

	if err := writeZipFile(zw, "README.md", []byte(exportReadme)); err != nil {
		s.log.Warn("导出 README 失败", "err", err)
		return
	}

	for _, p := range posts {
		if err := writeZipFile(zw, "posts/"+exportFileName(p.Slug)+".md", []byte(renderPostMarkdown(p))); err != nil {
			s.log.Warn("导出文章失败", "slug", p.Slug, "err", err)
			return
		}
	}
	for _, p := range pages {
		if err := writeZipFile(zw, "pages/"+exportFileName(p.Slug)+".md", []byte(renderPageMarkdown(p))); err != nil {
			s.log.Warn("导出页面失败", "slug", p.Slug, "err", err)
			return
		}
	}

	if raw, err := json.MarshalIndent(posts, "", "  "); err == nil {
		if err := writeZipFile(zw, "posts.json", raw); err != nil {
			s.log.Warn("导出 posts.json 失败", "err", err)
			return
		}
	}
	if raw, err := json.MarshalIndent(site, "", "  "); err == nil {
		if err := writeZipFile(zw, "site.json", raw); err != nil {
			s.log.Warn("导出 site.json 失败", "err", err)
			return
		}
	}

	s.addMediaToZip(zw)
}

// addMediaToZip 把上传目录里的图片按 media/<相对路径> 放进压缩包。
// 媒体目录不存在时静默跳过：新建站点可能还没有任何图片。
func (s *Server) addMediaToZip(zw *zip.Writer) {
	root := s.cfg.UploadDir
	if root == "" {
		return
	}
	if _, err := os.Stat(root); err != nil {
		return
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 单个文件读不了就跳过，不要因为一张图毁掉整次导出
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxExportMediaBytes {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		name := "media/" + filepath.ToSlash(rel)
		dst, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(dst, f)
		return copyErr
	})
	if err != nil {
		s.log.Warn("导出媒体文件时出错", "err", err)
	}
}

func writeZipFile(zw *zip.Writer, name string, data []byte) error {
	dst, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = dst.Write(data)
	return err
}

// exportFileName 把 slug 变成安全的文件名，避免出现 ../ 或路径分隔符。
func exportFileName(slug string) string {
	slug = strings.TrimSpace(slug)
	replacer := strings.NewReplacer("/", "-", "\\", "-", "..", "-", "\x00", "")
	slug = replacer.Replace(slug)
	if slug == "" {
		return "untitled"
	}
	return slug
}

// frontMatterValue 让值能安全放进单行 front matter。
// 后台的 Markdown 导入器按 "key: value" 逐行解析，所以换行必须压平，首尾引号去掉。
func frontMatterValue(v string) string {
	v = strings.ReplaceAll(v, "\r\n", " ")
	v = strings.ReplaceAll(v, "\n", " ")
	v = strings.ReplaceAll(v, "\r", " ")
	v = strings.TrimSpace(v)
	v = strings.Trim(v, `"'`)
	return v
}

func renderPostMarkdown(p domain.Post) string {
	var b strings.Builder

	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %s\n", frontMatterValue(p.Title))
	fmt.Fprintf(&b, "slug: %s\n", frontMatterValue(p.Slug))
	if p.Summary != "" {
		fmt.Fprintf(&b, "summary: %s\n", frontMatterValue(p.Summary))
	}
	fmt.Fprintf(&b, "status: %s\n", frontMatterValue(p.Status))
	if p.PublishedAt != nil {
		fmt.Fprintf(&b, "published_at: %s\n", p.PublishedAt.UTC().Format(time.RFC3339))
	}
	if p.CoverURL != "" {
		fmt.Fprintf(&b, "cover_url: %s\n", frontMatterValue(p.CoverURL))
	}
	if len(p.Tags) > 0 {
		names := make([]string, 0, len(p.Tags))
		for _, t := range p.Tags {
			names = append(names, frontMatterValue(t.Name))
		}
		fmt.Fprintf(&b, "tags: %s\n", strings.Join(names, ", "))
	}
	b.WriteString("---\n\n")

	body := p.BodyMD
	if body == "" {
		body = "# " + p.Title + "\n"
	}
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

func renderPageMarkdown(p domain.Page) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %s\n", frontMatterValue(p.Title))
	fmt.Fprintf(&b, "slug: %s\n", frontMatterValue(p.Slug))
	b.WriteString("---\n\n")
	b.WriteString(p.BodyMD)
	if !strings.HasSuffix(p.BodyMD, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}
