package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/freeleebro/personalblog/backend/internal/auth"
	"github.com/freeleebro/personalblog/backend/internal/domain"
	"github.com/freeleebro/personalblog/backend/internal/store"
)

const (
	// 密码长度下限。bcrypt 对强度没有要求，只能自己定一个不至于太离谱的值。
	minPasswordLen = 8
	// bcrypt 只用到前 72 字节，再长会被静默截断，所以直接拒绝。
	maxPasswordBytes = 72
)

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// handleAdminChangePassword 让已登录的管理员自己改密码。
//
// 之所以需要它：以前换密码只能改 .env.server 里的 ADMIN_PASSWORD 再设
// ADMIN_PASSWORD_RESET=1 重启容器，对个人博客来说太重了。
//
// 注意：token 是无状态 HMAC（见 auth 包），改密码不会让已经登录的设备掉线。
// 真要把所有设备踢下线，只能轮换 ADMIN_SECRET——这个取舍写在了 README 里。
func (s *Server) handleAdminChangePassword(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value(ctxAdminUser{}).(string)

	// 请求体很短，限制一下，避免有人拿大 body 打这个接口。
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "请求体非法")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "当前密码和新密码都不能为空")
		return
	}
	if utf8.RuneCountInString(req.NewPassword) < minPasswordLen {
		writeError(w, http.StatusBadRequest, "weak_password",
			fmt.Sprintf("新密码至少 %d 个字符", minPasswordLen))
		return
	}
	if len(req.NewPassword) > maxPasswordBytes {
		writeError(w, http.StatusBadRequest, "password_too_long", "新密码不能超过 72 字节")
		return
	}
	if req.NewPassword == req.CurrentPassword {
		writeError(w, http.StatusBadRequest, "same_password", "新密码不能和当前密码相同")
		return
	}

	user, err := s.store.GetAdminUserByUsername(r.Context(), username)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "invalid_token", "账号不存在")
		return
	}
	if err != nil {
		s.fail(w, "查询管理员失败", err)
		return
	}

	if !auth.CheckPassword(user.PasswordHash, req.CurrentPassword) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "当前密码不正确")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		s.fail(w, "生成密码哈希失败", err)
		return
	}
	if _, err := s.store.UpsertAdminUser(r.Context(), user.Username, hash, user.DisplayName); err != nil {
		s.fail(w, "写入新密码失败", err)
		return
	}

	// 记下修改时间：启动时的环境变量比对会看这个标记，避免误报。
	// 这里失败不影响密码本身已生效，所以只告警不返回错误。
	changedAt := time.Now().UTC().Format(time.RFC3339)
	meta := domain.AdminMeta{PasswordUpdatedAt: changedAt}
	if err := s.store.PutSetting(r.Context(), domain.SettingAdminMeta, meta); err != nil {
		s.log.Warn("记录密码修改时间失败", "error", err)
	}

	s.log.Info("管理员密码已修改", "username", user.Username)

	w.Header().Set("Cache-Control", "no-store")
	writeData(w, map[string]any{
		"changed_at": changedAt,
		// 前端据此提示「其他设备仍然登录着」。
		"sessions_revoked": false,
	}, nil)
}
