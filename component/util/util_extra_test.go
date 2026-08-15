package util

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestCopyPropertiesNonBlank 空值不覆盖。
func TestCopyPropertiesNonBlank(t *testing.T) {
	type source struct {
		Name  string
		Age   int
		Email string
	}
	type target struct {
		Name  string
		Age   int
		Email string
	}
	src := source{Name: "", Age: 0, Email: "new@example.com"}
	dst := target{Name: "keep", Age: 30, Email: "old@example.com"}

	if err := CopyPropertiesNonBlank(&dst, src); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if dst.Name != "keep" {
		t.Fatalf("blank name must keep dst: %q", dst.Name)
	}
	if dst.Age != 30 {
		t.Fatalf("zero age must keep dst: %d", dst.Age)
	}
	if dst.Email != "new@example.com" {
		t.Fatalf("non-blank must copy: %q", dst.Email)
	}
}

// TestPager 分页计算。
func TestPager(t *testing.T) {
	pager := &Pager{PageNo: 2, PageSize: 10, Count: 25}
	if pager.Offset() != 10 {
		t.Fatalf("offset = %d", pager.Offset())
	}
	if pager.TotalPage() != 3 {
		t.Fatalf("total page = %d", pager.TotalPage())
	}
	if pager.FirstPage() {
		t.Fatal("page 2 must not be first")
	}
	if pager.LastPage() {
		t.Fatal("page 2 of 3 must not be last")
	}
	// 默认值
	empty := &Pager{}
	if empty.Limit() != 10 || empty.PageNo != 1 {
		t.Fatalf("defaults wrong: %+v", empty)
	}
	// 上限截断
	large := &Pager{PageSize: 500}
	if large.Limit() != 100 {
		t.Fatalf("page size cap: %d", large.Limit())
	}
}

// TestFormatTime 零值返回空串。
func TestFormatTime(t *testing.T) {
	if FormatTime(time.Time{}) != "" {
		t.Fatal("zero time must return empty")
	}
	now := time.Date(2026, 8, 16, 1, 2, 3, 0, time.Local)
	if FormatTime(now) != "2026-08-16 01:02:03" {
		t.Fatalf("format = %q", FormatTime(now))
	}
	if FormatTime(now, "2006-01-02") != "2026-08-16" {
		t.Fatalf("custom format = %q", FormatTime(now, "2006-01-02"))
	}
}

// TestParseTimeByStr 容错解析。
func TestParseTimeByStr(t *testing.T) {
	parsed := ParseTimeByStr("2026-08-16 01:02:03")
	if parsed.IsZero() || parsed.Year() != 2026 {
		t.Fatalf("parse failed: %v", parsed)
	}
	// 日期格式
	if ParseTimeByStr("2026-08-16").IsZero() {
		t.Fatal("date only must parse")
	}
	// 空串/非法 → 零值不 panic
	if !ParseTimeByStr("").IsZero() {
		t.Fatal("empty must return zero")
	}
	if !ParseTimeByStr("not-a-date").IsZero() {
		t.Fatal("invalid must return zero")
	}
}

// TestFileHash 流式哈希。
func TestFileHash(t *testing.T) {
	content := bytes.NewReader([]byte("hello"))
	hash := FileHash(content, "sha1")
	// sha1("hello") = aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d
	if hash != "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d" {
		t.Fatalf("sha1 = %q", hash)
	}
	content2 := bytes.NewReader([]byte("hello"))
	if FileHash(content2, "md5") != "5d41402abc4b2a76b9719d911017c592" {
		t.Fatalf("md5 = %q", FileHash(content2, "md5"))
	}
	if FileHash(nil, "md5") != "" {
		t.Fatal("nil reader must return empty")
	}
}

// TestCopyPropertiesNonBlankPointerSrc 指针源。
func TestCopyPropertiesNonBlankPointerSrc(t *testing.T) {
	type source struct{ Name string }
	type target struct {
		Name string
		Age  int
	}
	src := &source{Name: "x"}
	dst := target{Age: 1}
	if err := CopyPropertiesNonBlank(&dst, src); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if dst.Name != "x" || dst.Age != 1 {
		t.Fatalf("copy wrong: %+v", dst)
	}
	// 类型不匹配字段跳过不报错
	if err := CopyPropertiesNonBlank(&dst, "not struct"); err != nil {
		t.Fatalf("non-struct src must be ignored: %v", err)
	}
}

// TestPagerResult 分页结果组装。
func TestPagerResult(t *testing.T) {
	pager := &Pager{PageNo: 1, PageSize: 20, Count: 45}
	result := PagerResult{
		List:      []string{"a", "b"},
		PageNo:    pager.PageNo,
		PageSize:  pager.Limit(),
		Total:     pager.Count,
		TotalPage: pager.TotalPage(),
		FirstPage: pager.FirstPage(),
		LastPage:  pager.LastPage(),
	}
	if result.TotalPage != 3 || !result.FirstPage || result.LastPage {
		t.Fatalf("result wrong: %+v", result)
	}
	_ = strings.Join // keep import
}
