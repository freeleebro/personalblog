package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// changePasswordCase 只覆盖「还没碰到数据库就该被拒」的输入校验路径，
// 所以可以拿一个没有 store 的 Server 直接调 handler。
func TestChangePasswordValidation(t *testing.T) {
	s := &Server{}

	cases := []struct {
		name     string
		body     string
		wantCode int
		wantErr  string
	}{
		{"空请求体", `{}`, http.StatusBadRequest, "bad_request"},
		{"缺当前密码", `{"new_password":"newpass123"}`, http.StatusBadRequest, "bad_request"},
		{"缺新密码", `{"current_password":"oldpass123"}`, http.StatusBadRequest, "bad_request"},
		{"新密码太短", `{"current_password":"oldpass123","new_password":"short"}`, http.StatusBadRequest, "weak_password"},
		{"新密码过长", `{"current_password":"oldpass123","new_password":"` + strings.Repeat("a", 73) + `"}`, http.StatusBadRequest, "password_too_long"},
		{"新旧相同", `{"current_password":"samepass123","new_password":"samepass123"}`, http.StatusBadRequest, "same_password"},
		{"JSON 非法", `{`, http.StatusBadRequest, "bad_request"},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/password", strings.NewReader(tc.body))
		req = req.WithContext(context.WithValue(req.Context(), ctxAdminUser{}, "admin"))
		w := httptest.NewRecorder()

		s.handleAdminChangePassword(w, req)

		if w.Code != tc.wantCode {
			t.Errorf("%s：得到 %d，期望 %d（%s）", tc.name, w.Code, tc.wantCode, w.Body.String())
			continue
		}
		if !strings.Contains(w.Body.String(), tc.wantErr) {
			t.Errorf("%s：错误码应包含 %q，实际 %s", tc.name, tc.wantErr, w.Body.String())
		}
	}
}

// 8 个字符是下限，7 个字符必须被拒、8 个字符要通过长度检查（走到数据库那一步才失败）。
func TestChangePasswordLengthBoundary(t *testing.T) {
	s := &Server{}

	short := `{"current_password":"oldpass123","new_password":"1234567"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/password", strings.NewReader(short))
	req = req.WithContext(context.WithValue(req.Context(), ctxAdminUser{}, "admin"))
	w := httptest.NewRecorder()
	s.handleAdminChangePassword(w, req)
	if !strings.Contains(w.Body.String(), "weak_password") {
		t.Fatalf("7 个字符应被拒，实际 %s", w.Body.String())
	}

	// 恰好 8 个字符：长度校验通过。store 为 nil 会 panic，所以只断言不返回 400。
	ok := `{"current_password":"oldpass123","new_password":"12345678"}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/password", strings.NewReader(ok))
	req2 = req2.WithContext(context.WithValue(req2.Context(), ctxAdminUser{}, "admin"))
	w2 := httptest.NewRecorder()

	defer func() {
		// 走到 store 时会 panic，这恰好证明长度校验放行了。
		if r := recover(); r == nil {
			t.Fatalf("期望在访问 store 时失败，但请求正常结束：%s", w2.Body.String())
		}
	}()
	s.handleAdminChangePassword(w2, req2)
}
