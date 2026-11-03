// Package migrations 以 embed 方式携带 SQL 迁移脚本，保证二进制自包含。
package migrations

import "embed"

// FS 内含全部 .sql 迁移文件，按文件名升序执行。
//
//go:embed *.sql
var FS embed.FS
