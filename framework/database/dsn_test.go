package datasource

import (
	"strings"
	"testing"
)

// TestBuildPostgreSQLDSNPassword 回归:DSN 必须包含真实密码,不是字面星号。
func TestBuildPostgreSQLDSNPassword(t *testing.T) {
	dsn := buildPostgreSQLDSN(Config{
		Host:     "127.0.0.1",
		Port:     5432,
		UserName: "postgres",
		Password: "real-password-123",
		Database: "demo",
		SSLMode:  "disable",
		TimeZone: "Asia/Shanghai",
	}, 5)
	if !strings.Contains(dsn, "password=real-password-123") {
		t.Fatalf("dsn must contain real password: %s", dsn)
	}
	if strings.Contains(dsn, "password=***") {
		t.Fatalf("dsn must not contain literal asterisks: %s", dsn)
	}
	// 其他字段完整性
	for _, want := range []string{
		"host=127.0.0.1", "user=postgres", "dbname=demo", "port=5432",
		"sslmode=disable", "TimeZone=Asia/Shanghai", "connect_timeout=5",
	} {
		if !strings.Contains(dsn, want) {
			t.Errorf("dsn missing %q: %s", want, dsn)
		}
	}
}
