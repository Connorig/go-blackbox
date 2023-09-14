package assert

import "reflect"

/**
* @Author: Connor
* @Date:   23.3.28 14:37
* @Description:
 */
// 断言工具类

// IsNilFixed 判断当前值是否为nil，i 必须为: Ptr、Map、Array、Chan、Slice
func IsNilFixed(i interface{}) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return reflect.ValueOf(i).IsNil()
	}
	return false
}
