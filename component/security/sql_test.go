package security

import (
	"testing"
)

// TestIsSQLInjectionPositive 典型注入载荷必须命中。
func TestIsSQLInjectionPositive(t *testing.T) {
	payloads := []string{
		"1 UNION SELECT username, password FROM users",
		"' OR '1'='1",
		`" OR "1"="1`,
		"1 OR 1=1",
		"admin'--",
		"x'; DROP TABLE users;--",
		"id=1 /* comment */ AND 1=1",
		"1 AND 1=1",
		"SELECT * FROM information_schema.tables",
		"1; SELECT * FROM users",
		"1' AND SLEEP(5)--",
		"1 AND UPDATEXML(1,CONCAT(0x7e,VERSION()),1)",
		"LOAD_FILE('/etc/passwd')",
		"INTO OUTFILE '/tmp/x'",
		"DROP TABLE orders",
		"TRUNCATE TABLE logs",
		"1' OR '1'='1' -- -",
	}
	for _, payload := range payloads {
		if !IsSQLInjection(payload) {
			t.Errorf("must detect: %q", payload)
		}
	}
}

// TestIsSQLInjectionNegative 正常业务输入必须放行。
func TestIsSQLInjectionNegative(t *testing.T) {
	values := []string{
		"",
		"hello world",
		"张三",
		"NO-20260815-001",
		"2026-08-15 14:00:00",
		"abc@example.com",
		"13800138000",
		"select_status",     // 字段名包含 select 但不构成注入
		"union_high_school", // 单词前缀不是关键字注入
		"{\"name\":\"demo\"}",
		"order_id=100",
		"1+1=2", // 无关键字
		"a-b_c.d",
	}
	for _, value := range values {
		if IsSQLInjection(value) {
			t.Errorf("must NOT detect: %q", value)
		}
	}
}

// TestFindInjection 命中时返回模式,未命中返回空。
func TestFindInjection(t *testing.T) {
	if pattern := FindInjection("normal value"); pattern != "" {
		t.Fatalf("unexpected pattern: %q", pattern)
	}
	if pattern := FindInjection("1 UNION SELECT 2"); pattern == "" {
		t.Fatal("must find pattern")
	}
}

// TestCheckValues 批量检测返回命中项。
func TestCheckValues(t *testing.T) {
	hits := CheckValues("safe", "1' OR '1'='1", "also safe", "x; DROP TABLE t")
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2: %v", len(hits), hits)
	}
}

// TestContainsControlChars 控制字符检测(日志注入防护)。
func TestContainsControlChars(t *testing.T) {
	if !ContainsControlChars("a\x00b") {
		t.Fatal("must detect NUL")
	}
	if !ContainsControlChars("a\x1bb") {
		t.Fatal("must detect ESC")
	}
	if ContainsControlChars("a\tb\nc") {
		t.Fatal("tab/newline are allowed")
	}
}

// TestSanitizeLog 控制字符被替换。
func TestSanitizeLog(t *testing.T) {
	clean := SanitizeLog("user\x00\x1battack")
	for _, r := range clean {
		if r < 0x20 || r == 0x7f {
			t.Fatalf("control char leaked: %q", clean)
		}
	}
}
