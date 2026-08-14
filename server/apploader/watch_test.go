package apploader

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWatchReloadsConfigurationOnFileChange 验证配置文件变更后目标结构体重载并触发回调。
func TestWatchReloadsConfigurationOnFileChange(t *testing.T) {
	configDirectory := t.TempDir()
	configPath := filepath.Join(configDirectory, "config.toml")
	if err := os.WriteFile(configPath, []byte("name = \"v1\"\n"), 0o600); err != nil {
		t.Fatalf("write initial config failed: %v", err)
	}

	var config testConfiguration
	configLoader := NewLoader().SetConfigFileSearcher("config", configDirectory)
	if err := configLoader.LoadToStruct(&config); err != nil {
		t.Fatalf("load initial config failed: %v", err)
	}
	if config.Name != "v1" {
		t.Fatalf("unexpected initial value: %q", config.Name)
	}

	reloaded := make(chan struct{}, 1)
	if err := configLoader.Watch(func() {
		reloaded <- struct{}{}
	}); err != nil {
		t.Fatalf("start watch failed: %v", err)
	}

	// 重写配置文件触发变更事件
	if err := os.WriteFile(configPath, []byte("name = \"v2\"\n"), 0o600); err != nil {
		t.Fatalf("rewrite config failed: %v", err)
	}

	select {
	case <-reloaded:
		if config.Name != "v2" {
			t.Fatalf("config was not reloaded, name=%q", config.Name)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for config change event")
	}
}

// TestWatchRequiresLoadToStruct 验证未先加载目标时 Watch 返回错误。
func TestWatchRequiresLoadToStruct(t *testing.T) {
	if err := NewLoader().Watch(func() {}); err == nil {
		t.Fatal("Watch without LoadToStruct must return an error")
	}
}

// TestWatchRejectsNilHandler 验证 nil 回调被拒绝。
func TestWatchRejectsNilHandler(t *testing.T) {
	configDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDirectory, "config.toml"), []byte("name = \"v1\"\n"), 0o600); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
	var config testConfiguration
	configLoader := NewLoader().SetConfigFileSearcher("config", configDirectory)
	if err := configLoader.LoadToStruct(&config); err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	if err := configLoader.Watch(nil); err == nil {
		t.Fatal("nil handler must be rejected")
	}
}
