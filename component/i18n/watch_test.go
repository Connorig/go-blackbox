package i18n

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestWatchDirReload 验证文件变化触发重载与回调。
func TestWatchDirReload(t *testing.T) {
	dir := t.TempDir()
	zhPath := filepath.Join(dir, "zh-CN.json")
	if err := os.WriteFile(zhPath, []byte(`{"welcome":"你好"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	bundle := NewBundle()
	if err := bundle.LoadDir(dir); err != nil {
		t.Fatalf("initial load failed: %v", err)
	}
	if got := bundle.T("zh-CN", "welcome"); got != "你好" {
		t.Fatalf("unexpected initial: %q", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	changed := 0
	done := make(chan struct{})
	go func() {
		_ = bundle.WatchDir(ctx, dir, 50*time.Millisecond, func() {
			mu.Lock()
			changed++
			mu.Unlock()
		})
		close(done)
	}()
	t.Cleanup(func() { cancel(); <-done })

	// 修改文件内容
	time.Sleep(80 * time.Millisecond) // 确保首次指纹完成
	if err := os.WriteFile(zhPath, []byte(`{"welcome":"您好","new_key":"新值"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// 等待重载生效
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if bundle.T("zh-CN", "new_key") == "新值" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := bundle.T("zh-CN", "new_key"); got != "新值" {
		t.Fatalf("reload did not apply new key: %q", got)
	}
	mu.Lock()
	count := changed
	mu.Unlock()
	if count == 0 {
		t.Fatal("onChange must be called after reload")
	}
}

// TestWatchDirNoChangeNoCallback 无变化不回调。
func TestWatchDirNoChangeNoCallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en-US.json"), []byte(`{"k":"v"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle := NewBundle()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	changed := 0
	done := make(chan struct{})
	go func() {
		_ = bundle.WatchDir(ctx, dir, 40*time.Millisecond, func() {
			mu.Lock()
			changed++
			mu.Unlock()
		})
		close(done)
	}()
	time.Sleep(200 * time.Millisecond)
	cancel()
	<-done
	mu.Lock()
	defer mu.Unlock()
	if changed != 0 {
		t.Fatalf("no change must not trigger callback, got %d", changed)
	}
}

// TestWatchDirErrors 目录错误与 nil 安全。
func TestWatchDirErrors(t *testing.T) {
	var bundle *Bundle
	if err := bundle.WatchDir(context.Background(), "langs", time.Second, nil); err == nil {
		t.Fatal("nil bundle must return error")
	}
	bundle = NewBundle()
	if err := bundle.WatchDir(context.Background(), "", time.Second, nil); err == nil {
		t.Fatal("empty dir must return error")
	}
	if err := bundle.WatchDir(context.Background(), filepath.Join(t.TempDir(), "missing"), time.Second, nil); err == nil {
		t.Fatal("missing dir must return error")
	}
}

// TestWatchDirCancel 取消后立即退出。
func TestWatchDirCancel(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "zh-CN.json"), []byte(`{"k":"v"}`), 0o644)
	bundle := NewBundle()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = bundle.WatchDir(ctx, dir, time.Hour, nil) // 长间隔,仅验证取消
		close(done)
	}()
	cancel()
	select {
	case <-done:
		// 正常退出
	case <-time.After(2 * time.Second):
		t.Fatal("WatchDir must exit on ctx cancel")
	}
}
