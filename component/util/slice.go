package util

// 泛型集合工具(数组/切片/map 常用操作)。

// SliceContains 判断切片是否包含指定元素(comparable 类型)。
func SliceContains[T comparable](list []T, value T) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// SliceFilter 过滤切片,保留 fn 返回 true 的元素。
func SliceFilter[T any](list []T, fn func(item T) bool) []T {
	if len(list) == 0 {
		return nil
	}
	result := make([]T, 0, len(list))
	for _, item := range list {
		if fn(item) {
			result = append(result, item)
		}
	}
	return result
}

// SliceMap 转换切片(映射为另一种类型)。
func SliceMap[T any, R any](list []T, fn func(item T) R) []R {
	if len(list) == 0 {
		return nil
	}
	result := make([]R, 0, len(list))
	for _, item := range list {
		result = append(result, fn(item))
	}
	return result
}

// SliceRemove 移除指定元素(返回新切片,不修改原切片)。
func SliceRemove[T comparable](list []T, value T) []T {
	result := make([]T, 0, len(list))
	for _, item := range list {
		if item != value {
			result = append(result, item)
		}
	}
	return result
}

// SliceRemoveAll 移除全部匹配元素(fn 返回 true 的移除)。
func SliceRemoveAll[T any](list []T, fn func(item T) bool) []T {
	return SliceFilter(list, func(item T) bool { return !fn(item) })
}

// SliceFirst 返回第一个匹配元素;未找到返回零值与 false。
func SliceFirst[T any](list []T, fn func(item T) bool) (T, bool) {
	for _, item := range list {
		if fn(item) {
			return item, true
		}
	}
	var zero T
	return zero, false
}

// SliceGroupBy 按 key 分组(key 必须 comparable)。
func SliceGroupBy[T any, K comparable](list []T, keyFn func(item T) K) map[K][]T {
	result := make(map[K][]T)
	for _, item := range list {
		key := keyFn(item)
		result[key] = append(result[key], item)
	}
	return result
}

// MapKeys 返回 map 的全部键。
func MapKeys[K comparable, V any](values map[K]V) []K {
	keys := make([]K, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

// MapValues 返回 map 的全部值。
func MapValues[K comparable, V any](values map[K]V) []V {
	result := make([]V, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}
