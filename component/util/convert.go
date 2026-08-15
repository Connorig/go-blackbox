package util

import (
	"reflect"
	"strconv"
)

// parseStringToNumber 的辅助实现:字符串按目标类型解析。

func parseInt(text string, kind reflect.Kind) (interface{}, error) {
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return nil, err
	}
	switch kind {
	case reflect.Int:
		return int(value), nil
	case reflect.Int8:
		return int8(value), nil
	case reflect.Int16:
		return int16(value), nil
	case reflect.Int32:
		return int32(value), nil
	default:
		return value, nil
	}
}

func parseUint(text string, kind reflect.Kind) (interface{}, error) {
	value, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return nil, err
	}
	switch kind {
	case reflect.Uint:
		return uint(value), nil
	case reflect.Uint8:
		return uint8(value), nil
	case reflect.Uint16:
		return uint16(value), nil
	case reflect.Uint32:
		return uint32(value), nil
	default:
		return value, nil
	}
}

func parseFloat(text string, kind reflect.Kind) (interface{}, error) {
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil, err
	}
	if kind == reflect.Float32 {
		return float32(value), nil
	}
	return value, nil
}
