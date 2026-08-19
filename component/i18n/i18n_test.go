package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRegisterAndT 验证注册与翻译(含回退)。
func TestRegisterAndT(t *testing.T) {
	bundle := NewBundle()
	bundle.Register("zh-CN", map[string]string{
		"order.created": "订单创建成功,编号 {{order_no}}",
		"hello":         "你好,{{ name }}",
	})
	bundle.Register("en-US", map[string]string{
		"order.created": "Order created, id {{order_no}}",
	})

	if got := bundle.T("zh-CN", "hello", map[string]interface{}{"name": "张三"}); got != "你好,张三" {
		t.Fatalf("unexpected zh: %q", got)
	}
	if got := bundle.T("en-US", "order.created", map[string]interface{}{"order_no": 1001}); got != "Order created, id 1001" {
		t.Fatalf("unexpected en: %q", got)
	}
	// 未注册语言回退默认(zh-CN)
	if got := bundle.T("fr-FR", "hello", map[string]interface{}{"name": "李四"}); got != "你好,李四" {
		t.Fatalf("fallback failed: %q", got)
	}
	// 缺失 key 返回 key
	if got := bundle.T("zh-CN", "no.such.key"); got != "no.such.key" {
		t.Fatalf("missing key must return key, got %q", got)
	}
}

// TestLoadDir 验证从目录加载语言文件。
func TestLoadDir(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "zh-CN.json"), []byte(`{"welcome":"欢迎"}`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "en-US.json"), []byte(`{"welcome":"Welcome"}`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "README.txt"), []byte("ignore"), 0o644)

	bundle := NewBundle()
	if err := bundle.LoadDir(dir); err != nil {
		t.Fatalf("load dir failed: %v", err)
	}
	if got := bundle.T("en-US", "welcome"); got != "Welcome" {
		t.Fatalf("unexpected en: %q", got)
	}
	langs := bundle.Langs()
	if len(langs) != 2 || langs[0] != "en-US" || langs[1] != "zh-CN" {
		t.Fatalf("unexpected langs: %v", langs)
	}
	if !bundle.Has("zh-CN") || bundle.Has("fr-FR") {
		t.Fatal("Has must reflect loaded langs")
	}
}

// TestLoadDirErrors 验证目录错误。
func TestLoadDirErrors(t *testing.T) {
	bundle := NewBundle()
	if err := bundle.LoadDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing dir must return error")
	}
	empty := t.TempDir()
	if err := bundle.LoadDir(empty); err == nil {
		t.Fatal("dir without language files must return error")
	}
	bad := t.TempDir()
	_ = os.WriteFile(filepath.Join(bad, "zh-CN.json"), []byte("{invalid"), 0o644)
	if err := bundle.LoadDir(bad); err == nil {
		t.Fatal("invalid json must return error")
	}
}

// TestDetectLanguage 验证 Accept-Language 解析。
func TestDetectLanguage(t *testing.T) {
	bundle := NewBundle()
	bundle.Register("zh-CN", map[string]string{"k": "v"})
	bundle.Register("en-US", map[string]string{"k": "v"})

	cases := map[string]string{
		"":                            "zh-CN", // 空 → 回退
		"zh-CN,zh;q=0.9":              "zh-CN",
		"zh-cn":                       "zh-CN", // 大小写归一
		"en-US,en;q=0.9":              "en-US",
		"fr-FR,en-US;q=0.8":           "en-US", // 首个未注册跳过,取下一个
		"fr-FR;q=0.9,de-DE":           "zh-CN", // 都未注册 → 回退
		"EN-us":                       "en-US",
	}
	for header, expected := range cases {
		if got := bundle.DetectLanguage(header); got != expected {
			t.Fatalf("DetectLanguage(%q) = %q, want %q", header, got, expected)
		}
	}
}

// TestTf 验证 fmt 风格格式化。
func TestTf(t *testing.T) {
	bundle := NewBundle()
	bundle.Register("zh-CN", map[string]string{"greet": "你好, %s!你有 %d 条消息"})
	if got := bundle.Tf("zh-CN", "greet", "张三", 3); got != "你好, 张三!你有 3 条消息" {
		t.Fatalf("unexpected tf: %q", got)
	}
	if got := bundle.Tf("zh-CN", "no.such"); got != "no.such" {
		t.Fatalf("missing key tf must return key: %q", got)
	}
}

// TestNilAndEdge 验证 nil 安全与边界。
func TestNilAndEdge(t *testing.T) {
	var bundle *Bundle
	if bundle.T("zh-CN", "key") != "key" {
		t.Fatal("nil bundle must return key")
	}
	if bundle.DetectLanguage("zh-CN") != DefaultLang {
		t.Fatal("nil bundle must return default lang")
	}
	bundle = NewBundle()
	bundle.SetFallback("en-US")
	bundle.Register("en-US", map[string]string{"k": "fallback"})
	if got := bundle.T("de-DE", "k"); got != "fallback" {
		t.Fatalf("custom fallback failed: %q", got)
	}
	bundle.Register("", map[string]string{"x": "y"}) // 空语言忽略
	if got := bundle.T("", "x"); got != "x" {
		t.Fatal("empty lang must not register")
	}
}
