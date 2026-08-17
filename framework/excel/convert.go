package excel

import (
	"reflect"
	"strconv"
)

// parseIntField 解析整数字段(支持基础 int 类型)。
func parseIntField(f reflect.Value, value string) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return
	}
	f.SetInt(parsed)
}

// parseFloatField 解析浮点字段。
func parseFloatField(f reflect.Value, value string) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return
	}
	f.SetFloat(parsed)
}
