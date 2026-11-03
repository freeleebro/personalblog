package config

import (
	"testing"
	"time"
)

func TestGetEnvTrimsAndFallsBack(t *testing.T) {
	t.Setenv("DSH_TEST_VALUE", "  hello  ")
	if got := getEnv("DSH_TEST_VALUE", "fallback"); got != "hello" {
		t.Errorf("应裁掉首尾空白，得到 %q", got)
	}

	t.Setenv("DSH_TEST_VALUE", "   ")
	if got := getEnv("DSH_TEST_VALUE", "fallback"); got != "fallback" {
		t.Errorf("全空白应按未设置处理，得到 %q", got)
	}

	t.Setenv("DSH_TEST_EMPTY", "")
	if got := getEnv("DSH_TEST_EMPTY", "fallback"); got != "fallback" {
		t.Errorf("空值应回退，得到 %q", got)
	}
}

func TestGetBool(t *testing.T) {
	cases := []struct {
		value    string
		fallback bool
		want     bool
	}{
		{"1", false, true},
		{"true", false, true},
		{"TRUE", false, true},
		{"0", true, false},
		{"false", true, false},
		{"yes", true, true}, // 非法值回退
		{"", true, true},    // 空串回退
	}

	for _, tc := range cases {
		t.Setenv("DSH_TEST_BOOL", tc.value)
		if got := getBool("DSH_TEST_BOOL", tc.fallback); got != tc.want {
			t.Errorf("getBool(%q, %v)=%v，期望 %v", tc.value, tc.fallback, got, tc.want)
		}
	}
}

func TestGetDuration(t *testing.T) {
	t.Setenv("DSH_TEST_DUR", "42s")
	if got := getDuration("DSH_TEST_DUR", time.Second); got != 42*time.Second {
		t.Errorf("得到 %v，期望 42s", got)
	}

	t.Setenv("DSH_TEST_DUR", "不是时长")
	if got := getDuration("DSH_TEST_DUR", 7*time.Second); got != 7*time.Second {
		t.Errorf("非法时长应回退，得到 %v", got)
	}
}

func TestSplitList(t *testing.T) {
	got := splitList(" a, b ,,c ")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("得到 %v，期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 项得到 %q，期望 %q", i, got[i], want[i])
		}
	}

	if got := splitList(""); len(got) != 0 {
		t.Fatalf("空串应得到空列表，得到 %v", got)
	}
}

func TestLoadDefaults(t *testing.T) {
	// 清掉可能影响默认值的变量（t.Setenv 会在测试结束后自动恢复）。
	for _, key := range []string{"APP_ENV", "APP_ADDR", "ADMIN_USERNAME", "ADMIN_SECRET", "UPLOAD_DIR", "TRUSTED_PROXIES"} {
		t.Setenv(key, "")
	}

	cfg := Load()
	if cfg.Env != "development" {
		t.Errorf("默认环境应为 development，得到 %q", cfg.Env)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("默认监听地址应为 :8080，得到 %q", cfg.Addr)
	}
	if cfg.AdminUsername != "admin" {
		t.Errorf("默认管理员用户名应为 admin，得到 %q", cfg.AdminUsername)
	}
	if cfg.UploadDir != "./data/uploads" {
		t.Errorf("默认上传目录应为 ./data/uploads，得到 %q", cfg.UploadDir)
	}
	if len(cfg.TrustedProxies) == 0 {
		t.Error("默认应信任回环与内网网段")
	}
	if cfg.IsProduction() {
		t.Error("development 环境不应被判为生产")
	}
}

func TestIsProduction(t *testing.T) {
	if !(Config{Env: "production"}).IsProduction() {
		t.Error("production 应判为生产环境")
	}
	if (Config{Env: "Production"}).IsProduction() {
		t.Error("大小写不同不应判为生产环境")
	}
}

func TestLocation(t *testing.T) {
	// 零值 Config 不应 panic，退回 UTC。
	if got := (Config{}).Location(); got != time.UTC {
		t.Errorf("零值 Config 的 Location 应为 UTC，得到 %v", got)
	}

	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("当前环境没有时区数据：%v", err)
	}
	cfg := Config{SiteLocation: shanghai}
	// 2026-11-03 17:00Z 是北京时间 2026-11-04 01:00——自动 slug 的日期应按站点时区。
	if got := time.Date(2026, 11, 3, 17, 0, 0, 0, time.UTC).In(cfg.Location()).Format("20060102"); got != "20261104" {
		t.Errorf("Asia/Shanghai 下应格式化为 20261104，得到 %s", got)
	}

	// 非法时区名应退回 UTC 而不是 panic。
	if got := loadLocation("Not/AZone"); got == nil {
		t.Error("非法时区名不应返回 nil")
	}
}
