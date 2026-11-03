// Package auth 提供管理后台的密码哈希与登录 token。
//
// 密码用 bcrypt 哈希存储；token 采用 HMAC-SHA256 签名，格式为
// base64(payload).base64(signature)，payload 是 JSON 编码的 username + 过期时间。
// 全程无状态、零外部依赖，适合个人博客这种单机部署场景。
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword 用 bcrypt 生成密码哈希。
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("生成密码哈希失败: %w", err)
	}
	return string(b), nil
}

// CheckPassword 校验明文密码与哈希是否匹配。
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

type claims struct {
	Username string `json:"username"`
	Expires  int64  `json:"exp"` // Unix 秒
}

// SignToken 签发一个有效期 ttl 的登录 token。
func SignToken(secret, username string, ttl time.Duration) (string, error) {
	payload := claims{Username: username, Expires: time.Now().Add(ttl).Unix()}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("编码 token 失败: %w", err)
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	sig := sign(secret, body)
	return body + "." + sig, nil
}

// VerifyToken 校验 token 的签名与有效期，返回其中的用户名。
func VerifyToken(secret, token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", errors.New("token 格式非法")
	}
	body, sig := parts[0], parts[1]
	if !hmac.Equal([]byte(sig), []byte(sign(secret, body))) {
		return "", errors.New("token 签名无效")
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return "", errors.New("token 载荷非法")
	}
	var c claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return "", errors.New("token 载荷非法")
	}
	if c.Expires < time.Now().Unix() {
		return "", errors.New("token 已过期")
	}
	if c.Username == "" {
		return "", errors.New("token 缺少用户名")
	}
	return c.Username, nil
}

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
