package apploader

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeToml 写测试配置文件。
func writeToml(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// TestLayeredOverride 分层覆盖:内置默认 < 全局 < 项目 < 环境变量。
func TestLayeredOverride(t *testing.T) {
	dir := t.TempDir()
	writeToml(t, dir, "gbx.toml", `
[web]
port = "9090"
level = "warn"

[db]
maxIdleConns = 5
`)
	writeToml(t, dir, "config.toml", `
[web]
port = "8080"

[db]
maxIdleConns = 8
ssl = "require"
`)

	type config struct {
		Web struct {
			Port  string `mapstructure:"port"`
			Level string `mapstructure:"level"`
		} `mapstructure:"web"`
		Db struct {
			MaxIdleConns int    `mapstructure:"maxIdleConns"`
			Ssl          string `mapstructure:"ssl"`
		} `mapstructure:"db"`
	}
	var cfg config

	t.Setenv("TESTAPP_WEB_PORT", "7070")
	loader := NewLoader()
	loader.SetGlobalConfigFile("gbx", dir)      // 父级
	loader.SetConfigFileSearcher("config", dir) // 子级
	loader.EnableEnvSearcher("TESTAPP")
	if err := loader.LoadToStruct(&cfg); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	// 环境变量最高
	if cfg.Web.Port != "7070" {
		t.Fatalf("env must win: %q", cfg.Web.Port)
	}
	// 项目覆盖全局
	if cfg.Db.MaxIdleConns != 8 {
		t.Fatalf("project must override global: %d", cfg.Db.MaxIdleConns)
	}
	// 全局独有键保留(键级合并,非文件级替换)
	if cfg.Web.Level != "warn" {
		t.Fatalf("global-only key must be kept: %q", cfg.Web.Level)
	}
	// 项目独有键
	if cfg.Db.Ssl != "require" {
		t.Fatalf("project-only key missing: %q", cfg.Db.Ssl)
	}
}

// TestLayeredKeyMerge 嵌套键级合并(父级子键保留)。
func TestLayeredKeyMerge(t *testing.T) {
	dir := t.TempDir()
	writeToml(t, dir, "gbx.toml", `
[modules]
[modules.web]
enabled = true
port = "8080"

[modules.monitor]
enabled = true
`)
	writeToml(t, dir, "config.toml", `
[modules]
[modules.web]
port = "9090"   # 只覆盖子键
`)

	type cfg struct {
		Modules struct {
			Web struct {
				Enabled bool   `mapstructure:"enabled"`
				Port    string `mapstructure:"port"`
			} `mapstructure:"web"`
			Monitor struct {
				Enabled bool `mapstructure:"enabled"`
			} `mapstructure:"monitor"`
		} `mapstructure:"modules"`
	}
	var c cfg
	loader := NewLoader()
	loader.SetGlobalConfigFile("gbx", dir)
	loader.SetConfigFileSearcher("config", dir)
	if err := loader.LoadToStruct(&c); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if !c.Modules.Web.Enabled {
		t.Fatal("parent enabled must be inherited")
	}
	if c.Modules.Web.Port != "9090" {
		t.Fatalf("child override failed: %q", c.Modules.Web.Port)
	}
	if !c.Modules.Monitor.Enabled {
		t.Fatal("parent-only key must be kept")
	}
}

// TestLayeredGlobalOptional 全局文件缺失不阻塞。
func TestLayeredGlobalOptional(t *testing.T) {
	dir := t.TempDir()
	writeToml(t, dir, "config.toml", `
[web]
port = "8080"
`)
	type cfg struct {
		Web struct {
			Port string `mapstructure:"port"`
		} `mapstructure:"web"`
	}
	var c cfg
	loader := NewLoader()
	loader.SetGlobalConfigFile("missing-gbx", dir)
	loader.SetConfigFileSearcher("config", dir)
	if err := loader.LoadToStruct(&c); err != nil {
		t.Fatalf("missing global must not block: %v", err)
	}
	if c.Web.Port != "8080" {
		t.Fatalf("project config lost: %q", c.Web.Port)
	}
}

// TestModulesDefaults 模块结构默认值与 TOML 解析。
func TestModulesDefaults(t *testing.T) {
	dir := t.TempDir()
	writeToml(t, dir, "config.toml", `
[modules]
[modules.web]
enabled = true
port = ":9528"

[modules.database]
enabled = true
driver = "sqlite"
dsn = "./app.db"

[modules.openapi]
enabled = true
[modules.openapi.apps]
partner-001 = { secret = "s3cret", algorithm = "HMAC-SHA256", enabled = true }
`)
	type cfg struct {
		Modules Modules `mapstructure:"modules"`
	}
	var c cfg
	loader := NewLoader()
	loader.SetConfigFileSearcher("config", dir)
	if err := loader.LoadToStruct(&c); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if !c.Modules.Web.Enabled || c.Modules.Web.Port != ":9528" {
		t.Fatalf("web module wrong: %+v", c.Modules.Web)
	}
	if !c.Modules.Database.Enabled || c.Modules.Database.Driver != "sqlite" {
		t.Fatalf("database module wrong: %+v", c.Modules.Database)
	}
	if !c.Modules.OpenAPI.Enabled {
		t.Fatal("openapi must be enabled")
	}
	app, ok := c.Modules.OpenAPI.Apps["partner-001"]
	if !ok || app.Secret != "s3cret" || !app.Enabled {
		t.Fatalf("openapi app wrong: %+v", c.Modules.OpenAPI.Apps)
	}
	// 未配置的模块默认关闭
	if c.Modules.Monitor.Enabled || c.Modules.Admin.Enabled || c.Modules.Cache.Enabled {
		t.Fatal("unconfigured modules must stay disabled")
	}
}

// TestModulesEnvOverride 模块配置支持环境变量覆盖。
func TestModulesEnvOverride(t *testing.T) {
	dir := t.TempDir()
	writeToml(t, dir, "config.toml", `
[modules]
[modules.admin]
enabled = true
listen = ":6060"
`)
	type cfg struct {
		Modules Modules `mapstructure:"modules"`
	}
	var c cfg
	t.Setenv("TESTAPP_MODULES_ADMIN_LISTEN", ":7070")
	loader := NewLoader()
	loader.SetConfigFileSearcher("config", dir)
	loader.EnableEnvSearcher("TESTAPP")
	if err := loader.LoadToStruct(&c); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if c.Modules.Admin.Listen != ":7070" {
		t.Fatalf("env override failed: %q", c.Modules.Admin.Listen)
	}
}

// TestDurationField TOML 时长字段解析。
func TestDurationField(t *testing.T) {
	dir := t.TempDir()
	writeToml(t, dir, "config.toml", `
[modules]
[modules.auth]
enabled = true
access_ttl = "1h"
refresh_ttl = "72h"
issuer = "demo"
`)
	type cfg struct {
		Modules Modules `mapstructure:"modules"`
	}
	var c cfg
	loader := NewLoader()
	loader.SetConfigFileSearcher("config", dir)
	if err := loader.LoadToStruct(&c); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if c.Modules.Auth.AccessTTL != time.Hour {
		t.Fatalf("access_ttl = %s", c.Modules.Auth.AccessTTL)
	}
	if c.Modules.Auth.RefreshTTL != 72*time.Hour {
		t.Fatalf("refresh_ttl = %s", c.Modules.Auth.RefreshTTL)
	}
	if c.Modules.Auth.Issuer != "demo" {
		t.Fatalf("issuer = %q", c.Modules.Auth.Issuer)
	}
}
