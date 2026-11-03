package store

import (
	"strings"
	"testing"
)

func TestMigrationChecksumStable(t *testing.T) {
	body := []byte("CREATE TABLE t (id INT);\n")
	first := migrationChecksum(body)
	if len(first) != 64 {
		t.Fatalf("校验和应为 64 位十六进制，得到 %d 位: %q", len(first), first)
	}
	if strings.ContainsFunc(first, func(r rune) bool {
		return !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f'))
	}) {
		t.Fatalf("校验和只能包含小写十六进制字符: %q", first)
	}
	if again := migrationChecksum(body); again != first {
		t.Fatalf("相同内容应得到相同校验和: %q != %q", first, again)
	}
}

func TestMigrationChecksumDetectsEdits(t *testing.T) {
	original := migrationChecksum([]byte("ALTER TABLE posts ADD COLUMN cover_url TEXT;"))
	edited := migrationChecksum([]byte("ALTER TABLE posts ADD COLUMN cover_url TEXT NOT NULL;"))
	if original == edited {
		t.Fatal("内容变化后校验和必须变化，否则无法发现历史迁移被改写")
	}
	// 只差一个空白字符也必须能发现。
	if migrationChecksum([]byte("SELECT 1;")) == migrationChecksum([]byte("SELECT 1; ")) {
		t.Fatal("末尾空白差异也应改变校验和")
	}
}
