package util

import (
	"errors"
	"fmt"
	"reflect"
)

// 反射工具:结构体属性拷贝(对标 Java BeanUtils.copyProperties)、
// 深拷贝、按字段名读写属性。基于 GoFrame gconv / gin-vue-admin utils 的常用能力整理。

// CopyProperties 将 src 的导出字段按「字段名」拷贝到 dst(对标 Java BeanUtils.copyProperties)。
// dst 必须为指向结构体的指针;src 为结构体或结构体指针。
// 规则:
//   - 字段名完全匹配(区分大小写);src 有而 dst 没有的字段忽略
//   - 类型相同直接赋值;基本数值类型(int 族/float 族)与 string 之间自动转换
//   - dst 的零值字段会被 src 覆盖(与 BeanUtils 一致);dst 已有非零值也会被覆盖
//   - 未导出字段、不可设置字段跳过
//
//	type UserDTO struct { Name string; Age int }
//	type UserEntity struct { Name string; Age int64; Extra string }
//	var dto UserDTO
//	util.CopyProperties(&dto, userEntity) // Name 直接拷贝,Age int64→int 自动转换
func CopyProperties(dst, src interface{}) error {
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

	srcType := srcValue.Type()
	for i := 0; i < srcValue.NumField(); i++ {
		srcField := srcType.Field(i)
		if srcField.PkgPath != "" { // 未导出字段
			continue
		}
		srcFieldValue := srcValue.Field(i)
		dstField := dstElem.FieldByName(srcField.Name)
		if !dstField.IsValid() || !dstField.CanSet() {
			continue
		}
		if err := assignValue(dstField, srcFieldValue); err != nil {
			return fmt.Errorf("copy field %q: %w", srcField.Name, err)
		}
	}
	return nil
}

// assignValue 赋值,支持同类型与基本类型转换。
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

// DeepCopy 深拷贝任意值(对标 Java 序列化深拷贝):
// 支持基本类型、string、struct、map、slice、array、指针、interface 的组合嵌套。
// 结构体中的未导出字段无法复制(反射限制),导出字段递归复制。
func DeepCopy(src interface{}) (interface{}, error) {
	if src == nil {
		return nil, nil
	}
	return deepCopyValue(reflect.ValueOf(src))
}

func deepCopyValue(value reflect.Value) (interface{}, error) {
	if !value.IsValid() {
		return nil, nil
	}
	switch value.Kind() {
	case reflect.Ptr:
		if value.IsNil() {
			// 返回同类型 nil 指针
			return reflect.Zero(value.Type()).Interface(), nil
		}
		copied, err := deepCopyValue(value.Elem())
		if err != nil {
			return nil, err
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(reflect.ValueOf(copied))
		return result.Interface(), nil
	case reflect.Interface:
		if value.IsNil() {
			return nil, nil
		}
		return deepCopyValue(value.Elem())
	case reflect.Struct:
		result := reflect.New(value.Type()).Elem()
		for i := 0; i < value.NumField(); i++ {
			field := value.Type().Field(i)
			if field.PkgPath != "" {
				continue
			}
			copied, err := deepCopyValue(value.Field(i))
			if err != nil {
				return nil, err
			}
			if copied != nil {
				result.Field(i).Set(reflect.ValueOf(copied))
			}
		}
		return result.Interface(), nil
	case reflect.Map:
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			keyCopied, err := deepCopyValue(iter.Key())
			if err != nil {
				return nil, err
			}
			valCopied, err := deepCopyValue(iter.Value())
			if err != nil {
				return nil, err
			}
			result.SetMapIndex(reflect.ValueOf(keyCopied), reflect.ValueOf(valCopied))
		}
		return result.Interface(), nil
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type()).Interface(), nil
		}
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			copied, err := deepCopyValue(value.Index(i))
			if err != nil {
				return nil, err
			}
			if copied != nil {
				result.Index(i).Set(reflect.ValueOf(copied))
			}
		}
		return result.Interface(), nil
	case reflect.Array:
		result := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			copied, err := deepCopyValue(value.Index(i))
			if err != nil {
				return nil, err
			}
			if copied != nil {
				result.Index(i).Set(reflect.ValueOf(copied))
			}
		}
		return result.Interface(), nil
	default:
		// 基本类型直接复制
		return value.Interface(), nil
	}
}

// FieldValue 按字段名读取结构体属性(支持嵌套字段名如 "User.Name")。
func FieldValue(obj interface{}, fieldName string) (interface{}, error) {
	value, err := fieldPath(reflect.ValueOf(obj), fieldName)
	if err != nil {
		return nil, err
	}
	if !value.IsValid() {
		return nil, fmt.Errorf("field %q not found", fieldName)
	}
	return value.Interface(), nil
}

// SetFieldValue 按字段名设置结构体属性(支持嵌套字段名)。
// value 为 nil 时清空该字段(设为类型零值)。
func SetFieldValue(obj interface{}, fieldName string, value interface{}) error {
	target, err := fieldPath(reflect.ValueOf(obj), fieldName)
	if err != nil {
		return err
	}
	if !target.IsValid() || !target.CanSet() {
		return fmt.Errorf("field %q is not settable", fieldName)
	}
	if value == nil {
		target.Set(reflect.Zero(target.Type()))
		return nil
	}
	return assignValue(target, reflect.ValueOf(value))
}

// fieldPath 解析点号路径并返回字段 Value(自动解引用指针)。
func fieldPath(root reflect.Value, path string) (reflect.Value, error) {
	value := root
	parts := splitPath(path)
	for i, part := range parts {
		for value.Kind() == reflect.Ptr || value.Kind() == reflect.Interface {
			if value.IsNil() {
				return reflect.Value{}, fmt.Errorf("field %q: nil pointer at %q", path, part)
			}
			value = value.Elem()
		}
		if value.Kind() != reflect.Struct {
			return reflect.Value{}, fmt.Errorf("field %q: %q is not a struct", path, part)
		}
		field := value.FieldByName(part)
		if !field.IsValid() {
			return reflect.Value{}, fmt.Errorf("field %q not found", path)
		}
		if i == len(parts)-1 {
			return field, nil
		}
		value = field
	}
	return reflect.Value{}, fmt.Errorf("field %q not found", path)
}

func splitPath(path string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			parts = append(parts, path[start:i])
			start = i + 1
		}
	}
	parts = append(parts, path[start:])
	return parts
}
