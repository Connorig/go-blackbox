package util

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"reflect"
	"time"
)

// 实战工具吸收(来自 sg-mes-api 的 ddd/common 与 internal/utils):
// 非空拷贝、分页计算、时间格式化/容错解析、流式文件哈希。

// CopyPropertiesNonBlank 拷贝属性,但 src 的零值字段不覆盖 dst(空值保护)。
// 对标 sg-mes-api 的 SimpleCopyProperties2 行为:
//   - 字段名 + 类型一致才拷贝
//   - src 字段为零值(空串/0/false/nil)时跳过,保留 dst 原值
//
// 与 CopyProperties(无条件覆盖)互补:更新场景用本函数,避免把空值写库。
func CopyPropertiesNonBlank(dst, src interface{}) error {
	if dst == nil || src == nil {
		return nil
	}
	dstValue := reflect.ValueOf(dst)
	if dstValue.Kind() != reflect.Ptr || dstValue.IsNil() {
		return nil
	}
	dstElem := dstValue.Elem()
	if dstElem.Kind() != reflect.Struct {
		return nil
	}
	srcValue := reflect.ValueOf(src)
	if srcValue.Kind() == reflect.Ptr {
		if srcValue.IsNil() {
			return nil
		}
		srcValue = srcValue.Elem()
	}
	if srcValue.Kind() != reflect.Struct {
		return nil
	}

	srcType := srcValue.Type()
	for i := 0; i < srcValue.NumField(); i++ {
		field := srcType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		value := srcValue.Field(i)
		if isBlankValue(value) {
			continue
		}
		dstField := dstElem.FieldByName(field.Name)
		if !dstField.IsValid() || !dstField.CanSet() {
			continue
		}
		if err := assignValue(dstField, value); err != nil {
			continue // 类型不匹配跳过(与 sg-mes 行为一致)
		}
	}
	return nil
}

// isBlankValue 判断零值(对齐 sg-mes 的 isBlank)。
func isBlankValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.String:
		return value.Len() == 0
	case reflect.Bool:
		return !value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return value.Float() == 0
	case reflect.Interface, reflect.Ptr, reflect.Slice, reflect.Map:
		return value.IsNil()
	}
	return reflect.DeepEqual(value.Interface(), reflect.Zero(value.Type()).Interface())
}

// Pager 分页参数与计算(对齐 sg-mes 的 PagerReq/PagerRes)。
type Pager struct {
	PageNo   int // 页码(从 1 开始;<=0 时按 1)
	PageSize int // 每页数量(<=0 时按 10;>100 时截断为 100)
	Count    int64 // 总条数(查询后设置,用于计算分页信息)
}

// Normalize 归一化分页参数。
func (p *Pager) Normalize() {
	if p.PageNo <= 0 {
		p.PageNo = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 10
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
}

// Offset 分页偏移(limit 前使用)。
func (p *Pager) Offset() int {
	p.Normalize()
	return (p.PageNo - 1) * p.PageSize
}

// Limit 每页数量。
func (p *Pager) Limit() int {
	p.Normalize()
	return p.PageSize
}

// TotalPage 总页数。
func (p *Pager) TotalPage() int {
	p.Normalize()
	if p.Count <= 0 {
		return 0
	}
	total := int(p.Count) / p.PageSize
	if int(p.Count)%p.PageSize != 0 {
		total++
	}
	return total
}

// FirstPage 是否首页。
func (p *Pager) FirstPage() bool {
	p.Normalize()
	return p.PageNo <= 1
}

// LastPage 是否尾页。
func (p *Pager) LastPage() bool {
	return p.PageNo >= p.TotalPage()
}

// PagerResult 分页响应(webiris.OK 的 data 用)。
type PagerResult struct {
	List      interface{} `json:"list"`
	PageNo    int         `json:"page_no"`
	PageSize  int         `json:"page_size"`
	Total     int64       `json:"total"`
	TotalPage int         `json:"total_page"`
	FirstPage bool        `json:"first_page"`
	LastPage  bool        `json:"last_page"`
}

// ---- 时间格式化/解析(对齐 sg-mes 习惯) ----

// FormatTime 格式化时间;零值(0001-01-01)返回空串(数据库未赋值场景友好)。
// format 为空时使用 "2006-01-02 15:04:05"。
func FormatTime(t time.Time, format ...string) string {
	if t.IsZero() {
		return ""
	}
	layout := "2006-01-02 15:04:05"
	if len(format) > 0 && format[0] != "" {
		layout = format[0]
	}
	return t.Format(layout)
}

// ParseTimeByStr 容错解析时间字符串;解析失败返回零值(不 panic)。
// 支持格式按顺序尝试:2006-01-02 15:04:05 / 2006-01-02 15:04 / 2006-01-02。
func ParseTimeByStr(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		time.RFC3339,
	} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// FileHash 计算流式文件/数据哈希(对齐 sg-mes FileHash)。
// algorithm: "md5" 或 "sha1"(默认 sha1);返回小写 hex。
func FileHash(reader io.Reader, algorithm string) string {
	if reader == nil {
		return ""
	}
	var hasher io.Writer
	switch algorithm {
	case "md5":
		hasher = md5.New()
	default:
		hasher = sha1.New()
	}
	if _, err := io.Copy(hasher, reader); err != nil {
		return ""
	}
	switch typed := hasher.(type) {
	case interface{ Sum([]byte) []byte }:
		return hex.EncodeToString(typed.Sum(nil))
	}
	return ""
}
