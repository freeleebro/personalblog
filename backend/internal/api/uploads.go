package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"image"
	// 注册解码器，供 image.DecodeConfig 读取宽高。webp 不在标准库里，
	// 前端上传时已统一转成 jpeg/webp，读不到宽高时返回 0 即可。
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	// maxUploadBytes 是单张配图的上限。前端会先压到长边 1600px，
	// 正常都在 300KB 以内，5MB 只是兜底。
	maxUploadBytes = 5 << 20

	// mediaURLPrefix 是配图对外的访问前缀，Go 与 Next 都认这个路径。
	mediaURLPrefix = "/media/"
)

// allowedImageTypes 把嗅探出来的真实 MIME 映射到落盘扩展名。
// 只认内容不认文件名，避免把 .php 改名成 .jpg 传上来。
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// handleAdminUpload 接收一张配图，落盘到 UPLOAD_DIR/YYYY/MM/<随机名>。
func (s *Server) handleAdminUpload(w http.ResponseWriter, r *http.Request) {
	// 先卡请求体大小，避免把超大文件读进内存。
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+1<<20)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "图片不能超过 5MB")
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", "请用 multipart/form-data 上传，字段名 file")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "缺少 file 字段")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		s.fail(w, "读取上传文件失败", err)
		return
	}
	if len(data) > maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "图片不能超过 5MB")
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "文件是空的")
		return
	}

	ext, ok := allowedImageTypes[http.DetectContentType(data)]
	if !ok {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_type", "只支持 JPEG / PNG / WebP / GIF 图片")
		return
	}

	now := time.Now()
	rel := path.Join(now.Format("2006"), now.Format("01"), randomName()+ext)
	full := filepath.Join(s.cfg.UploadDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		s.fail(w, "创建上传目录失败", err)
		return
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		s.fail(w, "写入上传文件失败", err)
		return
	}

	width, height := 0, 0
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		width, height = cfg.Width, cfg.Height
	}

	w.Header().Set("Cache-Control", "no-store")
	writeData(w, map[string]any{
		"url":    mediaURLPrefix + path.Join(rel),
		"width":  width,
		"height": height,
		"size":   len(data),
		"name":   header.Filename,
	}, nil)
}

// handleMedia 提供配图的只读访问。
//
// 文件名是随机串，内容永不变化，所以可以给一年强缓存；
// 只放行图片扩展名，且路径经过 Clean，避免读到 UPLOAD_DIR 之外的文件。
func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	rel := path.Clean("/" + r.PathValue("path"))
	if rel == "/" {
		http.NotFound(w, r)
		return
	}
	switch strings.ToLower(path.Ext(rel)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
	default:
		http.NotFound(w, r)
		return
	}

	full := filepath.Join(s.cfg.UploadDir, filepath.FromSlash(strings.TrimPrefix(rel, "/")))
	f, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func randomName() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand 失败属于系统级异常，用时间戳兜底也比崩溃好。
		return hex.EncodeToString([]byte(time.Now().Format("20060102150405")))
	}
	return hex.EncodeToString(buf)
}
