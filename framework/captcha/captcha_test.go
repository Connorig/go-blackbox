package captcha

import (
	"strings"
	"testing"
	"time"
)

// TestGenerateVerify 生成 → 校验(大小写不敏感)→ 一次性消费。
func TestGenerateVerify(t *testing.T) {
	generator := NewGenerator()
	id, base64PNG, err := generator.Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if id == "" || !strings.HasPrefix(base64PNG, "data:image/png;base64,") {
		t.Fatalf("id=%q png prefix missing", id)
	}
	// 无法预知答案:验证 id 存在性 + 消费语义
	if !generator.Exists(id) {
		t.Fatal("captcha must exist after generate")
	}
	// 错误答案消费后失效(防暴力破解)
	if generator.Verify(id, "wrong-answer") {
		t.Fatal("wrong answer must fail")
	}
	if generator.Exists(id) {
		t.Fatal("captcha must be consumed after verify attempt")
	}
}

// TestVerifyCaseInsensitive 大小写不敏感(通过内存 store 直接测)。
func TestVerifyCaseInsensitive(t *testing.T) {
	store := newMemoryStore()
	_ = store.Set("id1", "ABC", time.Minute)
	generator := NewGenerator().WithStore(store)
	if !generator.Verify("id1", "abc") {
		t.Fatal("case-insensitive verify must pass")
	}
}

// TestStoreExpiry TTL 过期失效。
func TestStoreExpiry(t *testing.T) {
	store := newMemoryStore()
	_ = store.Set("id1", "abc", 50*time.Millisecond)
	time.Sleep(80 * time.Millisecond)
	if store.Exists("id1") {
		t.Fatal("expired captcha must not exist")
	}
	if _, err := store.Get("id1"); err == nil {
		t.Fatal("expired get must fail")
	}
}

// TestMemoryStoreConsumeOnce 一次性消费。
func TestMemoryStoreConsumeOnce(t *testing.T) {
	store := newMemoryStore()
	_ = store.Set("id1", "abc", time.Minute)
	if answer, err := store.Get("id1"); err != nil || answer != "abc" {
		t.Fatalf("first get: %q %v", answer, err)
	}
	if _, err := store.Get("id1"); err == nil {
		t.Fatal("second get must fail (consumed)")
	}
}

// TestNilSafe nil 安全。
func TestNilSafe(t *testing.T) {
	var generator *Generator
	if _, _, err := generator.Generate(); err == nil {
		t.Fatal("nil generate must fail")
	}
	if generator.Verify("id", "ans") || generator.Exists("id") {
		t.Fatal("nil verify/exists must be false")
	}
}
