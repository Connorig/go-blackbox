package apploader

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEnvOverridesNestedConfig 环境变量覆盖嵌套配置:
// 前缀 + 嵌套键大写下划线(如 LIVE_LIVE_WEBADDR)必须覆盖文件值。
// 回归 Bug 7:v1.52.0 后环境变量覆盖失效的排障锁死。
func TestEnvOverridesNestedConfig(t *testing.T) {
	configDirectory := t.TempDir()
	configContent := []byte("name = \"demo\"\n[web]\nlisten = \"127.0.0.1:9528\"\n")
	if err := os.WriteFile(filepath.Join(configDirectory, "config.toml"), configContent, 0o600); err != nil {
		t.Fatal(err)
	}

	const envKey = "DEMO_WEB_LISTEN"
	const envValue = ":9998"
	oldValue, hadOld := os.LookupEnv(envKey)
	if err := os.Setenv(envKey, envValue); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if hadOld {
			_ = os.Setenv(envKey, oldValue)
		} else {
			_ = os.Unsetenv(envKey)
		}
	}()

	var config testConfiguration
	loader := NewLoader()
	loader.SetConfigFileSearcher("config", configDirectory).
		EnableEnvSearcher("DEMO")
	if err := loader.LoadToStruct(&config); err != nil {
		t.Fatalf("load: %v", err)
	}
	if config.Web.Listen != envValue {
		t.Fatalf("env override failed: got %q, want %q", config.Web.Listen, envValue)
	}
}

// TestEnvMissingFallsBackToFile 无环境变量时回退文件值。
func TestEnvMissingFallsBackToFile(t *testing.T) {
	configDirectory := t.TempDir()
	configContent := []byte("[web]\nlisten = \"127.0.0.1:9528\"\n")
	if err := os.WriteFile(filepath.Join(configDirectory, "config.toml"), configContent, 0o600); err != nil {
		t.Fatal(err)
	}
	const envKey = "FALLBACK_WEB_LISTEN"
	_ = os.Unsetenv(envKey)
	defer os.Unsetenv(envKey)

	var config testConfiguration
	loader := NewLoader()
	loader.SetConfigFileSearcher("config", configDirectory).
		EnableEnvSearcher("FALLBACK")
	if err := loader.LoadToStruct(&config); err != nil {
		t.Fatalf("load: %v", err)
	}
	if config.Web.Listen != "127.0.0.1:9528" {
		t.Fatalf("fallback failed: got %q", config.Web.Listen)
	}
}
