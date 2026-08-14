package apploader

import "testing"

// TestRedactMasksSensitiveFields 验证内置配置脱敏输出不含明文密码。
func TestRedactMasksSensitiveFields(t *testing.T) {
	var config Configuration
	config.Db.Password = "super-secret"
	config.Redis.Password = "redis-secret"
	config.Db.User = "postgres"

	redacted, ok := Redact(config).(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected redacted type: %T", Redact(config))
	}
	dbSection, ok := redacted["db"].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected db section type: %T", redacted["db"])
	}
	if dbSection["password"] != "***" {
		t.Fatalf("db password must be masked, got: %v", dbSection["password"])
	}
	if dbSection["user"] != "postgres" {
		t.Fatalf("non-sensitive field must be preserved: %v", dbSection["user"])
	}
	if containsSecret(redacted, "super-secret") || containsSecret(redacted, "redis-secret") {
		t.Fatal("redacted snapshot must not contain plaintext passwords")
	}
}

// TestRedactFallbackForNonRedactor 验证未实现 Redactor 的配置返回占位。
func TestRedactFallbackForNonRedactor(t *testing.T) {
	if value := Redact("plain string"); value == nil {
		t.Fatal("non-redactor config must return a placeholder")
	}
	if value := Redact(nil); value != nil {
		t.Fatalf("nil config must return nil, got %v", value)
	}
}

// TestEnvFileConvention 验证环境文件命名约定。
func TestEnvFileConvention(t *testing.T) {
	if EnvFile("config", "") != "config" {
		t.Fatalf("empty env must keep base name: %s", EnvFile("config", ""))
	}
	if EnvFile("config", "prod") != "config.prod" {
		t.Fatalf("unexpected env file name: %s", EnvFile("config", "prod"))
	}
	if EnvFile("config", "  dev  ") != "config.dev" {
		t.Fatalf("env name must be trimmed: %s", EnvFile("config", "  dev  "))
	}
}

// containsSecret 递归检查 map 值中是否出现敏感字符串。
func containsSecret(value interface{}, secret string) bool {
	switch typed := value.(type) {
	case map[string]interface{}:
		for _, item := range typed {
			if containsSecret(item, secret) {
				return true
			}
		}
	case string:
		return typed == secret
	}
	return false
}
