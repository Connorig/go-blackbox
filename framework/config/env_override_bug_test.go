package apploader

import (
	"os"
	"path/filepath"
	"testing"
)

// bug7Config 复现直播项目 Bug 7 的配置结构:
// 业务自定义结构,内嵌字段带 mapstructure 标签,前缀 LIVE。
type bug7Config struct {
	Live bug7Live `mapstructure:"live"`
}

type bug7Live struct {
	WebAddr string `mapstructure:"webAddr"`
	APIBase string `mapstructure:"apiBase"`
}

// TestEnvOverrideAfterLayeredMerge 复现 Bug 7:
// 环境变量必须覆盖 config.toml 的同名键(默认约定 + 驼峰拆分约定)。
func TestEnvOverrideAfterLayeredMerge(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	content := "[live]\nwebAddr = \":8080\"\napiBase = \"http://127.0.0.1:1985\"\n"
	if err := os.WriteFile(tomlPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// webAddr 用默认约定(全小写字段 ToUpper);apiBase 用拆分约定(直觉写法)。
	t.Setenv("LIVE_LIVE_WEBADDR", ":9530")
	t.Setenv("LIVE_LIVE_API_BASE", "http://10.0.0.9:1985")

	loader := NewLoader()
	loader.SetConfigFileSearcher("config", dir)
	loader.EnableEnvSearcher("LIVE")

	var config bug7Config
	if err := loader.LoadToStruct(&config); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if config.Live.WebAddr != ":9530" {
		t.Fatalf("env must override file: got %q, want %q", config.Live.WebAddr, ":9530")
	}
	if config.Live.APIBase != "http://10.0.0.9:1985" {
		t.Fatalf("split env must override file: got %q, want %q", config.Live.APIBase, "http://10.0.0.9:1985")
	}
}

// TestEnvOverrideSplitCamelPrecedence 拆分约定优先级高于默认约定。
func TestEnvOverrideSplitCamelPrecedence(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(tomlPath, []byte("[live]\napiBase = \"http://127.0.0.1:1985\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIVE_LIVE_APIBASE", "http://from-default-convention")
	t.Setenv("LIVE_LIVE_API_BASE", "http://from-split-convention")

	loader := NewLoader()
	loader.SetConfigFileSearcher("config", dir)
	loader.EnableEnvSearcher("LIVE")

	var config bug7Config
	if err := loader.LoadToStruct(&config); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if config.Live.APIBase != "http://from-split-convention" {
		t.Fatalf("split convention must win: got %q", config.Live.APIBase)
	}
}

// TestEnvOverrideSplitForWebAddr 全小写字段也支持拆分写法(WEB_ADDR)。
func TestEnvOverrideSplitForWebAddr(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(tomlPath, []byte("[live]\nwebAddr = \":8080\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIVE_LIVE_WEB_ADDR", ":9530")

	loader := NewLoader()
	loader.SetConfigFileSearcher("config", dir)
	loader.EnableEnvSearcher("LIVE")

	var config bug7Config
	if err := loader.LoadToStruct(&config); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if config.Live.WebAddr != ":9530" {
		t.Fatalf("split env must apply: got %q, want %q", config.Live.WebAddr, ":9530")
	}
}

// TestEnvOverrideWithoutEnvFallsBackToFile 验证无环境变量时文件值生效。
func TestEnvOverrideWithoutEnvFallsBackToFile(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	content := "[live]\nwebAddr = \":8080\"\napiBase = \"http://127.0.0.1:1985\"\n"
	if err := os.WriteFile(tomlPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIVE_LIVE_WEBADDR", "")
	t.Setenv("LIVE_LIVE_API_BASE", "")

	loader := NewLoader()
	loader.SetConfigFileSearcher("config", dir)
	loader.EnableEnvSearcher("LIVE")

	var config bug7Config
	if err := loader.LoadToStruct(&config); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if config.Live.WebAddr != ":8080" {
		t.Fatalf("file value must apply when env empty: got %q", config.Live.WebAddr)
	}
	if config.Live.APIBase != "http://127.0.0.1:1985" {
		t.Fatalf("file value must apply when env empty: got %q", config.Live.APIBase)
	}
}

// TestSplitCamelUnit 拆分算法单测。
func TestSplitCamelUnit(t *testing.T) {
	cases := map[string]string{
		"apiBase":   "api_base",
		"APIBase":   "api_base",
		"webAddr":   "web_addr",
		"APIServer": "api_server",
		"live":      "live",
		"order":     "order",
		"":          "",
		"URLPath":   "url_path",
	}
	for input, expected := range cases {
		if got := splitCamel(input); got != expected {
			t.Fatalf("splitCamel(%q) = %q, want %q", input, got, expected)
		}
	}
}
