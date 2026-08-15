package util

import (
	"encoding/json"
	"testing"
	"time"
)

// TestTimeJSON 序列化输出 yyyy-MM-dd HH:mm:ss,零值空串。
func TestTimeJSON(t *testing.T) {
	value := Time{Time: time.Date(2026, 8, 16, 1, 30, 0, 0, time.Local)}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(data) != `"2026-08-16 01:30:00"` {
		t.Fatalf("marshal = %s", data)
	}
	// 零值
	zero, _ := json.Marshal(Time{})
	if string(zero) != `""` {
		t.Fatalf("zero marshal = %s", zero)
	}
}

// TestTimeUnmarshal 容错解析。
func TestTimeUnmarshal(t *testing.T) {
	var value Time
	if err := json.Unmarshal([]byte(`"2026-08-16 01:30:00"`), &value); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if value.Year() != 2026 || value.Hour() != 1 {
		t.Fatalf("parsed = %v", value)
	}
	// 空串
	var empty Time
	if err := json.Unmarshal([]byte(`""`), &empty); err != nil || !empty.IsZero() {
		t.Fatalf("empty unmarshal: %v %v", empty, err)
	}
	// 非法 → 错误
	var bad Time
	if err := json.Unmarshal([]byte(`"not-a-time"`), &bad); err == nil {
		t.Fatal("invalid time must fail")
	}
}

// TestTimeParse 解析/格式化。
func TestTimeParse(t *testing.T) {
	parsed := Parse("2026-08-16 01:30:00")
	if parsed.IsZero() || parsed.Hour() != 1 {
		t.Fatalf("parse failed: %v", parsed)
	}
	if parsed.String() != "2026-08-16 01:30:00" {
		t.Fatalf("string = %q", parsed.String())
	}
	if !Parse("").IsZero() || !Parse("bad").IsZero() {
		t.Fatal("invalid parse must return zero")
	}
	if _, err := ParseE("bad"); err == nil {
		t.Fatal("ParseE must fail")
	}
}

// TestCopyNestedStruct 嵌套 struct 递归拷贝。
func TestCopyNestedStruct(t *testing.T) {
	type address struct {
		City    string
		ZipCode string
	}
	type source struct {
		Name    string
		Address address
	}
	type target struct {
		Name    string
		Address address
	}
	src := source{Name: "connor", Address: address{City: "上海", ZipCode: "200000"}}
	var dst target
	if err := CopyProperties(&dst, src); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if dst.Address.City != "上海" || dst.Address.ZipCode != "200000" {
		t.Fatalf("nested copy failed: %+v", dst.Address)
	}
}

// TestCopyTimeConversion 时间互转(util.Time ↔ time.Time ↔ string)。
func TestCopyTimeConversion(t *testing.T) {
	type srcModel struct {
		PaidAt   time.Time
		CreateAt Time
		DoneAt   string
	}
	type dstVO struct {
		PaidAt   string    // time.Time → string(格式化)
		CreateAt time.Time // util.Time → time.Time
		DoneAt   Time      // string → util.Time(解析)
	}
	base := time.Date(2026, 8, 16, 1, 30, 0, 0, time.Local)
	src := srcModel{
		PaidAt:   base,
		CreateAt: Time{Time: base},
		DoneAt:   "2026-08-16 02:00:00",
	}
	var vo dstVO
	if err := CopyProperties(&vo, src); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if vo.PaidAt != "2026-08-16 01:30:00" {
		t.Fatalf("time→string = %q", vo.PaidAt)
	}
	if vo.CreateAt.Hour() != 1 {
		t.Fatalf("util.Time→time.Time = %v", vo.CreateAt)
	}
	if vo.DoneAt.Hour() != 2 {
		t.Fatalf("string→util.Time = %v", vo.DoneAt)
	}
}

// TestCopyNonBlankNested 嵌套 + 空值保护。
func TestCopyNonBlankNested(t *testing.T) {
	type inner struct {
		A string
		B string
	}
	type srcT struct {
		Inner inner
	}
	type dstT struct {
		Inner inner
	}
	src := srcT{Inner: inner{A: "new", B: ""}}
	dst := dstT{Inner: inner{A: "keep", B: "keep-b"}}
	if err := CopyPropertiesNonBlank(&dst, src); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	if dst.Inner.A != "new" {
		t.Fatalf("nested non-blank copy failed: %+v", dst.Inner)
	}
	if dst.Inner.B != "keep-b" {
		t.Fatalf("nested blank must keep dst: %+v", dst.Inner)
	}
}

// TestCopyDOTOVO DTO/VO 实战场景:model(util.Time)→ json 对象(string 时间)。
func TestCopyDOTOVO(t *testing.T) {
	type orderModel struct {
		ID      int64
		OrderNo string
		PaidAt  Time
	}
	type orderVO struct {
		ID      int64  `json:"id"`
		OrderNo string `json:"order_no"`
		PaidAt  string `json:"paid_at"`
	}
	model := orderModel{
		ID:      1,
		OrderNo: "NO-001",
		PaidAt:  Time{Time: time.Date(2026, 8, 16, 1, 30, 0, 0, time.Local)},
	}
	var vo orderVO
	if err := CopyPropertiesNonBlank(&vo, model); err != nil {
		t.Fatalf("copy failed: %v", err)
	}
	data, err := json.Marshal(vo)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	// 时间已格式化,无需额外处理
	if parsed["paid_at"] != "2026-08-16 01:30:00" {
		t.Fatalf("paid_at = %v", parsed["paid_at"])
	}
	if parsed["order_no"] != "NO-001" {
		t.Fatalf("order_no = %v", parsed["order_no"])
	}
}
