package util

import (
	"errors"
	"fmt"
	"reflect"
	"time"
)

// 反射工具:结构体属性拷贝(对标 Java BeanUtils.copyProperties)、
// 深拷贝、按字段名读写属性。基于 GoFrame gconv / gin-vue-admin utils 的常用能力整理。

// maxCopyDepth 嵌套拷贝最大深度(防循环引用)。
const maxCopyDepth = 8

// CopyProperties 将 src 的导出字段按「字段名」拷贝到 dst(对标 Java BeanUtils.copyProperties)。
// dst 必须为指向结构体的指针;src 为结构体或结构体指针。
// 规则:
//
//   - 字段名完全匹配(区分大小写);src 有而 dst 没有的字段忽略
//
//   - 类型相同直接赋值;基本数值类型(int 族/float 族)与 string 之间自动转换
//
//   - 嵌套 struct 递归逐字段拷贝(深度上限 8,防循环)
//
//   - 时间互转:util.Time ↔ time.Time ↔ string(自动格式化/解析)
//
//   - 未导出字段、不可设置字段跳过
//
//     type UserDTO struct { Name string; Age int }
//     type UserEntity struct { Name string; Age int64; Extra string }
//     var dto UserDTO
//     util.CopyProperties(&dto, userEntity) // Name 直接拷贝,Age int64→int 自动转换
func CopyProperties(dst, src interface{}) error {
	return copyProperties(dst, src, false)
}

// copyProperties 通用拷贝入口。
func copyProperties(dst, src interface{}, nonBlank bool) error {
	if dst == nil {
		return errors.New("copy: dst is nil")
	}
	dstValue := reflect.ValueOf(dst)
	if dstValue.Kind() != reflect.Ptr || dstValue.IsNil() {
		return errors.New("copy: dst must be a non-nil pointer")
	}
	dstElem := dstValue.Elem()
	if dstElem.Kind() != reflect.Struct {
		return errors.New("copy: dst must point to a struct")
	}
	srcValue := reflect.ValueOf(src)
	if srcValue.Kind() == reflect.Ptr {
		if srcValue.IsNil() {
			return nil
		}
		srcValue = srcValue.Elem()
	}
	if srcValue.Kind() != reflect.Struct {
		return errors.New("copy: src must be a struct or struct pointer")
	}
	return copyStruct(dstElem, srcValue, nonBlank, 0)
}

// copyStruct 递归拷贝结构体字段。
func copyStruct(dst, src reflect.Value, nonBlank bool, depth int) error {
	if depth > maxCopyDepth {
		return nil
	}
	srcType := src.Type()
	for i := 0; i < src.NumField(); i++ {
		srcField := srcType.Field(i)
		if srcField.PkgPath != "" { // 未导出
			continue
		}
		srcFieldValue := src.Field(i)
		if nonBlank && isBlankValue(srcFieldValue) {
			continue
		}
		dstField := dst.FieldByName(srcField.Name)
		if !dstField.IsValid() || !dstField.CanSet() {
			continue
		}
		if err := assignField(dstField, srcFieldValue, nonBlank, depth); err != nil {
			return fmt.Errorf("copy field %q: %w", srcField.Name, err)
		}
	}
	return nil
}

// assignField 单字段赋值:嵌套递归 / 时间互转 / 同类型 / 基本转换。
func assignField(dst, src reflect.Value, nonBlank bool, depth int) error {
	// 解引用接口
	if src.Kind() == reflect.Interface {
		if src.IsNil() {
			return nil
		}
		src = src.Elem()
	}
	if !src.IsValid() {
		return nil
	}
	// 嵌套 struct:优先递归(空值保护逐字段生效;非 blank 模式递归=整体 Set 效果)
	if isStructKind(dst.Kind()) && isStructKind(src.Kind()) && !isTimeType(dst.Type()) && !isTimeType(src.Type()) {
		if dst.Kind() == reflect.Ptr {
			if dst.IsNil() {
				dst.Set(reflect.New(dst.Type().Elem()))
			}
			dst = dst.Elem()
		}
		if src.Kind() == reflect.Ptr {
			if src.IsNil() {
				return nil
			}
			src = src.Elem()
		}
		if dst.Kind() == reflect.Struct && src.Kind() == reflect.Struct {
			return copyStruct(dst, src, nonBlank, depth+1)
		}
	}
	// 同类型直接赋值(基本类型/时间类型等)
	if dst.Type() == src.Type() {
		dst.Set(src)
		return nil
	}
	// 时间互转
	if assignTimeField(dst, src) {
		return nil
	}
	// 基本类型转换(数值族/字符串)
	return assignValue(dst, src)
}

// assignTimeField 时间类型互转(util.Time ↔ time.Time ↔ string)。
// 返回 true 表示已处理。
func assignTimeField(dst, src reflect.Value) bool {
	srcTime, srcIsTime := toTimeValue(src)
	if srcIsTime {
		switch {
		case dst.Type() == reflect.TypeOf(Time{}):
			dst.Set(reflect.ValueOf(NewTime(srcTime)))
			return true
		case dst.Type() == reflect.TypeOf(time.Time{}):
			dst.Set(reflect.ValueOf(srcTime))
			return true
		case dst.Kind() == reflect.String:
			dst.SetString(FormatTime(srcTime))
			return true
		}
		return false
	}
	// src string → dst 时间类型(解析)
	if src.Kind() == reflect.String &&
		(dst.Type() == reflect.TypeOf(Time{}) || dst.Type() == reflect.TypeOf(time.Time{})) {
		parsed := Parse(src.String())
		if dst.Type() == reflect.TypeOf(Time{}) {
			dst.Set(reflect.ValueOf(parsed))
		} else {
			dst.Set(reflect.ValueOf(parsed.Time))
		}
		return true
	}
	return false
}

// toTimeValue 提取 time.Time(支持 time.Time 与 util.Time 及其指针)。
func toTimeValue(value reflect.Value) (time.Time, bool) {
	for value.Kind() == reflect.Ptr || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return time.Time{}, false
		}
		value = value.Elem()
	}
	switch typed := value.Interface().(type) {
	case time.Time:
		return typed, true
	case Time:
		return typed.Time, true
	}
	return time.Time{}, false
}

// isStructKind 是否结构体/指针族。
func isStructKind(kind reflect.Kind) bool {
	return kind == reflect.Struct || kind == reflect.Ptr
}

// isTimeType 是否时间类型。
func isTimeType(t reflect.Type) bool {
	if t == reflect.TypeOf(time.Time{}) || t == reflect.TypeOf(Time{}) {
		return true
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t == reflect.TypeOf(time.Time{}) || t == reflect.TypeOf(Time{})
}

// assignValue 赋值,支持同类型与基本类型转换(保持原行为)。
func assignValue(dst, src reflect.Value) error {
	if src.Kind() == reflect.Interface {
		if src.IsNil() {
			return nil
		}
		src = src.Elem()
	}
	if !src.IsValid() {
		return nil
	}
	if dst.Type() == src.Type() {
		dst.Set(src)
		return nil
	}
	// 数值族互转
	if isNumber(dst.Kind()) && isNumber(src.Kind()) {
		if dst.CanInt() && src.CanInt() {
			dst.SetInt(src.Int())
			return nil
		}
		if dst.CanUint() && src.CanUint() {
			dst.SetUint(src.Uint())
			return nil
		}
		if dst.CanFloat() && src.CanFloat() {
			dst.SetFloat(src.Float())
			return nil
		}
	}
	// string → 数值 / 数值 → string
	if dst.Kind() == reflect.String && isNumber(src.Kind()) {
		dst.SetString(fmt.Sprintf("%v", src.Interface()))
		return nil
	}
	if src.Kind() == reflect.String && isNumber(dst.Kind()) {
		parsed, err := parseStringToNumber(src.String(), dst.Kind())
		if err != nil {
			return err
		}
		dst.Set(parsed)
		return nil
	}
	// string → time.Time(时间字符串反解)
	if src.Kind() == reflect.String && dst.Type() == reflect.TypeOf(time.Time{}) {
		parsed := Parse(src.String())
		if !parsed.IsZero() {
			dst.Set(reflect.ValueOf(parsed.Time))
			return nil
		}
		return fmt.Errorf("type mismatch: cannot assign %s to %s", src.Type(), dst.Type())
	}
	return fmt.Errorf("type mismatch: cannot assign %s to %s", src.Type(), dst.Type())
}

func isNumber(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func parseStringToNumber(text string, kind reflect.Kind) (reflect.Value, error) {
	value := reflect.New(reflect.TypeOf(int64(0))).Elem()
	var parsed interface{}
	var err error
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err = parseInt(text, kind)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err = parseUint(text, kind)
	case reflect.Float32, reflect.Float64:
		parsed, err = parseFloat(text, kind)
	default:
		return value, errors.New("not a number kind")
	}
	if err != nil {
		return value, err
	}
	result := reflect.New(reflect.TypeOf(parsed)).Elem()
	result.Set(reflect.ValueOf(parsed))
	return result, nil
}
