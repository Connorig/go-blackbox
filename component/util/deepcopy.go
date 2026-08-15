package util

import (
	"fmt"
	"reflect"
)

// 深拷贝与字段访问(与 CopyProperties 同文件的既有能力,保持完整)。

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
			return reflect.Zero(value.Type()).Interface(), nil
		}
		copied, err := deepCopyValue(value.Elem())
		if err != nil {
			return nil, err
		}
		result := reflect.New(value.Type().Elem())
		if copied != nil {
			result.Elem().Set(reflect.ValueOf(copied))
		}
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
