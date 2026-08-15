package util

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

// Time 是封装后的时间类型(内嵌 time.Time):
//   - JSON 序列化自动输出 "2006-01-02 15:04:05"(零值输出空串)——接口返回无需额外格式化
//   - JSON 反序列化容错解析(多种格式)
//   - GORM 兼容(实现 Valuer/Scanner,数据库字段照常存取)
//   - 保留 time.Time 全部方法(t.Time() 获取原始值,兼容老逻辑)
//
// 用法:
//
//	type Order struct {
//	    PaidAt util.Time `json:"paid_at"` // 数据库存 DATETIME,接口返回 "2026-08-16 01:30:00"
//	}
//
// 对齐 sg-mes-api 痛点:model 时间字段无需在接口查询时额外 FormatTime。
const (
	// DefaultTimeLayout 默认展示格式。
	DefaultTimeLayout = "2006-01-02 15:04:05"
	// DateLayout 日期格式。
	DateLayout = "2006-01-02"
)

// Time 时间类型。
type Time struct {
	time.Time
}

// Now 当前时间。
func Now() Time {
	return Time{Time: time.Now()}
}

// NewTime 从 time.Time 构造。
func NewTime(t time.Time) Time {
	return Time{Time: t}
}

// Parse 解析时间字符串(容错:依次尝试多种格式);失败返回零值。
func Parse(value string) Time {
	if value == "" {
		return Time{}
	}
	for _, layout := range []string{
		DefaultTimeLayout,
		"2006-01-02 15:04",
		DateLayout,
		time.RFC3339,
		"2006-01-02T15:04:05",
	} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return Time{Time: parsed}
		}
	}
	return Time{}
}

// ParseE 解析时间字符串,失败返回错误。
func ParseE(value string) (Time, error) {
	if value == "" {
		return Time{}, nil
	}
	for _, layout := range []string{
		DefaultTimeLayout,
		"2006-01-02 15:04",
		DateLayout,
		time.RFC3339,
		"2006-01-02T15:04:05",
	} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return Time{Time: parsed}, nil
		}
	}
	return Time{}, fmt.Errorf("util.Time: cannot parse %q", value)
}

// String 格式化输出(零值返回空串)。
func (t Time) String() string {
	if t.IsZero() {
		return ""
	}
	return t.Format(DefaultTimeLayout)
}

// Format 按指定格式输出(零值返回空串)。
func (t Time) Format(layout string) string {
	if t.IsZero() {
		return ""
	}
	if layout == "" {
		layout = DefaultTimeLayout
	}
	return t.Time.Format(layout)
}

// MarshalJSON 序列化为 "2006-01-02 15:04:05"(零值输出空串)。
func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(`"` + t.String() + `"`), nil
}

// UnmarshalJSON 容错解析。
func (t *Time) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	text = strings.Trim(text, `"`)
	if text == "" || text == "null" {
		t.Time = time.Time{}
		return nil
	}
	parsed := Parse(text)
	if parsed.IsZero() {
		return fmt.Errorf("util.Time: invalid time %q", text)
	}
	t.Time = parsed.Time
	return nil
}

// Value 实现 driver.Valuer(GORM 写入:存 time.Time)。
func (t Time) Value() (driver.Value, error) {
	if t.IsZero() {
		return nil, nil
	}
	return t.Time, nil
}

// Scan 实现 sql.Scanner(GORM 读取)。
func (t *Time) Scan(value interface{}) error {
	if value == nil {
		t.Time = time.Time{}
		return nil
	}
	switch typed := value.(type) {
	case time.Time:
		t.Time = typed
		return nil
	case string:
		t.Time = Parse(typed).Time
		return nil
	case []byte:
		t.Time = Parse(string(typed)).Time
		return nil
	default:
		return fmt.Errorf("util.Time: cannot scan type %T", value)
	}
}
