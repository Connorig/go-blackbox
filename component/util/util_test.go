package util

import (
	"strings"
	"testing"
)

// ===== CopyProperties =====

type copySrc struct {
	Name    string
	Age     int
	Score   float64
	Active  bool
	extra   string // 未导出,忽略
	Ignored string
}

type copyDst struct {
	Name   string
	Age    int64 // 类型不同(int→int64 自动转换)
	Score  float64
	Active bool
	Only   string // src 没有,保持零值
}

type copyOther struct {
	Name string
	Age  string // 类型不同(string←int 自动转换)
}

// TestCopyProperties 基本拷贝 + 数值类型转换。
func TestCopyProperties(t *testing.T) {
	src := copySrc{Name: "connor", Age: 30, Score: 99.5, Active: true, extra: "hidden", Ignored: "x"}
	var dst copyDst
	if err := CopyProperties(&dst, src); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if dst.Name != "connor" || dst.Age != 30 || dst.Score != 99.5 || !dst.Active {
		t.Fatalf("copy mismatch: %+v", dst)
	}
	if dst.Only != "" {
		t.Fatalf("field absent in src must stay zero: %+v", dst)
	}
}

// TestCopyPropertiesPointerSrc 指针源。
func TestCopyPropertiesPointerSrc(t *testing.T) {
	src := &copySrc{Name: "ptr", Age: 1}
	var dst copyDst
	if err := CopyProperties(&dst, src); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if dst.Name != "ptr" {
		t.Fatalf("pointer src copy failed: %+v", dst)
	}
}

// TestCopyPropertiesStringNumber 字符串与数值互转。
func TestCopyPropertiesStringNumber(t *testing.T) {
	src := copySrc{Name: "n", Age: 42}
	var dst copyOther
	if err := CopyProperties(&dst, src); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if dst.Age != "42" {
		t.Fatalf("int -> string conversion failed: %q", dst.Age)
	}
}

// TestCopyPropertiesErrors 非法参数。
func TestCopyPropertiesErrors(t *testing.T) {
	if err := CopyProperties(nil, copySrc{}); err == nil {
		t.Fatal("nil dst must fail")
	}
	if err := CopyProperties(copySrc{}, copySrc{}); err == nil {
		t.Fatal("non-pointer dst must fail")
	}
	var dst copyDst
	if err := CopyProperties(&dst, "not struct"); err == nil {
		t.Fatal("non-struct src must fail")
	}
}

// ===== DeepCopy =====

type deepInner struct {
	Items []string
	Meta  map[string]int
}

type deepOuter struct {
	ID    int64
	Inner *deepInner
	Tags  []string
	Extra map[string]*deepInner
}

// TestDeepCopyStruct 深拷贝:修改副本不影响原值。
func TestDeepCopyStruct(t *testing.T) {
	original := deepOuter{
		ID:    1,
		Inner: &deepInner{Items: []string{"a", "b"}, Meta: map[string]int{"x": 1}},
		Tags:  []string{"t1"},
		Extra: map[string]*deepInner{"k": {Items: []string{"z"}}},
	}
	copied, err := DeepCopy(original)
	if err != nil {
		t.Fatalf("deep copy failed: %v", err)
	}
	clone := copied.(deepOuter)
	// 修改副本
	clone.Inner.Items[0] = "CHANGED"
	clone.Inner.Meta["x"] = 999
	clone.Tags[0] = "CHANGED"
	clone.Extra["k"].Items[0] = "CHANGED"
	// 原值不受影响
	if original.Inner.Items[0] != "a" {
		t.Fatalf("inner slice not deep copied: %v", original.Inner.Items)
	}
	if original.Inner.Meta["x"] != 1 {
		t.Fatalf("inner map not deep copied: %v", original.Inner.Meta)
	}
	if original.Tags[0] != "t1" {
		t.Fatalf("slice not deep copied: %v", original.Tags)
	}
	if original.Extra["k"].Items[0] != "z" {
		t.Fatalf("map of pointers not deep copied: %v", original.Extra)
	}
}

// TestDeepCopyBasics 基本类型与 nil 指针。
func TestDeepCopyBasics(t *testing.T) {
	// 基本类型
	if v, _ := DeepCopy(42); v.(int) != 42 {
		t.Fatal("int copy failed")
	}
	if v, _ := DeepCopy("str"); v.(string) != "str" {
		t.Fatal("string copy failed")
	}
	// nil 指针
	type T struct{ X int }
	var p *T
	v, err := DeepCopy(p)
	if err != nil {
		t.Fatalf("nil ptr copy failed: %v", err)
	}
	if v != nil && v.(*T) != nil {
		t.Fatalf("nil ptr must stay nil: %#v", v)
	}
	// nil 接口
	if v, err := DeepCopy(nil); err != nil || v != nil {
		t.Fatalf("nil copy failed: %v %v", v, err)
	}
}

// ===== FieldValue / SetFieldValue =====

// TestFieldValue 读取与写入(含嵌套)。
func TestFieldValue(t *testing.T) {
	obj := &deepOuter{ID: 7, Inner: &deepInner{Items: []string{"a"}}}
	value, err := FieldValue(obj, "ID")
	if err != nil || value.(int64) != 7 {
		t.Fatalf("FieldValue ID failed: %v %v", value, err)
	}
	value, err = FieldValue(obj, "Inner.Items")
	if err != nil || len(value.([]string)) != 1 {
		t.Fatalf("FieldValue nested failed: %v %v", value, err)
	}
	if err := SetFieldValue(obj, "ID", int64(99)); err != nil {
		t.Fatalf("SetFieldValue failed: %v", err)
	}
	if obj.ID != 99 {
		t.Fatalf("SetFieldValue not applied: %+v", obj)
	}
	if err := SetFieldValue(obj, "Inner.Items", []string{"b", "c"}); err != nil {
		t.Fatalf("SetFieldValue nested failed: %v", err)
	}
	if len(obj.Inner.Items) != 2 {
		t.Fatalf("nested set not applied: %+v", obj.Inner)
	}
	// 不存在字段
	if _, err := FieldValue(obj, "NotExist"); err == nil {
		t.Fatal("missing field must fail")
	}
}

// ===== Crypto =====

// TestMD5 已知向量。
func TestMD5(t *testing.T) {
	// md5("hello") = 5d41402abc4b2a76b9719d911017c592
	if got := MD5("hello"); got != "5d41402abc4b2a76b9719d911017c592" {
		t.Fatalf("md5 mismatch: %s", got)
	}
	// 与 MD5Bytes 一致
	if MD5("hello") != MD5Bytes([]byte("hello")) {
		t.Fatal("MD5Bytes mismatch")
	}
}

// TestSHA 已知向量。
func TestSHA(t *testing.T) {
	// sha1("hello") = aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d
	if got := SHA1("hello"); got != "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d" {
		t.Fatalf("sha1 mismatch: %s", got)
	}
	// sha256("hello") = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
	if got := SHA256("hello"); got != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("sha256 mismatch: %s", got)
	}
}

// TestRandomString 长度与字符集。
func TestRandomString(t *testing.T) {
	value, err := RandomString(16)
	if err != nil {
		t.Fatalf("random failed: %v", err)
	}
	if len(value) != 16 {
		t.Fatalf("length = %d, want 16", len(value))
	}
	for _, r := range value {
		if !strings.ContainsRune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", r) {
			t.Fatalf("illegal char: %q", r)
		}
	}
	// 不同调用不重复
	second, _ := RandomString(16)
	if value == second {
		t.Fatal("random collision")
	}
}

// TestUUIDHelper UUID 转发。
func TestUUIDHelper(t *testing.T) {
	value, err := UUID()
	if err != nil {
		t.Fatalf("uuid failed: %v", err)
	}
	if len(value) != 36 {
		t.Fatalf("uuid length = %d", len(value))
	}
	if empty := UUIDOrEmpty(); len(empty) != 36 {
		t.Fatalf("UUIDOrEmpty failed: %q", empty)
	}
}
