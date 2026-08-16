package datasource

import (
	"strings"
	"testing"
)

// TestPostgreSQLDSNPassword 回归:DSN 必须包含真实密码,不是字面 ***。
func TestPostgreSQLDSNPassword(t *testing.T) {
	dialector, err := newPostgreSQLDialector(Config{
		Host:     "127.0.0.1",
		Port:     5432,
		UserName: "postgres",
		Password: "real-password-123",
		Database: "demo",
		SSLMode:  "disable",
	})
	if err != nil {
		t.Fatalf("new dialector failed: %v", err)
	}
	dsn := dialector.Name()
	if !strings.Contains(dsn, "password=real-password-123") {
		t.Fatalf("dsn must contain real password: %s", dsn)
	}
	if strings.Contains(dsn, "password=***") {
		t.Fatalf("dsn must not contain literal ***: %s", dsn)
	}
}
