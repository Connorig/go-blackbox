package datasource

import (
	"errors"
	"fmt"
	"strings"
)

// BuildCondition 构建 WHERE 条件(从 sg-mes-api 实战项目吸收的精华封装):
// 将 map 形式的查询参数转换为 GORM 可用的条件串与参数值,
// 支持常见操作符后缀,业务查询零样板。
//
// 用法:
//
//	params := map[string]interface{}{
//	    "client_name like": "%" + keyword + "%",
//	    "created_at >=":    startDate,
//	    "grade":            10,
//	}
//	condition, values, err := datasource.BuildCondition(params)
//	// condition: `client_name like ? AND created_at >= ? AND grade = ?`
//	db.Where(condition, values...).Find(&list)
//
// 支持的键格式:
//   - `field`          等值(= ?)
//   - `field like`     模糊(= LIKE ?)
//   - `field >` `>=` ` <` `<=` `<>`  比较
//   - `field in`       切片展开(IN (?, ?, ...))
//   - `field not in`   NOT IN
//   - `field is null` / `field is not null`  空值判断(值忽略)
//
// 值处理:
//   - 空字符串忽略;nil 忽略;空切片忽略
//   - `like` 值自动前后加 %(传参时已带 % 则不重复加)
func BuildCondition(params map[string]interface{}) (string, []interface{}, error) {
	if len(params) == 0 {
		return "", nil, nil
	}
	var conditions []string
	var values []interface{}

	// 按键排序保证条件顺序稳定(可测试、可缓存)
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sortStrings(keys)

	for _, key := range keys {
		value := params[key]
		if isIgnoredValue(value) {
			continue
		}
		field, operator, err := parseConditionKey(key)
		if err != nil {
			return "", nil, err
		}
		condition, vals, err := buildOneCondition(field, operator, value)
		if err != nil {
			return "", nil, err
		}
		conditions = append(conditions, condition)
		values = append(values, vals...)
	}
	if len(conditions) == 0 {
		return "", nil, nil
	}
	return strings.Join(conditions, " AND "), values, nil
}

// parseConditionKey 解析条件键:字段名 + 操作符后缀。
func parseConditionKey(key string) (field, operator string, err error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "", "", errors.New("datasource: empty condition key")
	}
	parts := strings.Fields(trimmed)
	field = parts[0]
	if len(parts) == 1 {
		return field, "=", nil
	}
	operator = strings.ToLower(strings.Join(parts[1:], " "))
	switch operator {
	case "like", ">", ">=", "<", "<=", "<>", "!=", "in", "not in", "is null", "is not null":
		return field, operator, nil
	default:
		return "", "", fmt.Errorf("datasource: unsupported operator %q in key %q", operator, key)
	}
}

// buildOneCondition 构建单条件。
func buildOneCondition(field, operator string, value interface{}) (string, []interface{}, error) {
	safeField := quoteField(field)
	switch operator {
	case "is null", "is not null":
		return fmt.Sprintf("%s %s", safeField, operator), nil, nil
	case "in", "not in":
		items, ok := toSlice(value)
		if !ok || len(items) == 0 {
			return "", nil, nil // 空切片忽略(调用方已过滤)
		}
		placeholders := make([]string, len(items))
		for i := range items {
			placeholders[i] = "?"
		}
		verb := "IN"
		if operator == "not in" {
			verb = "NOT IN"
		}
		return fmt.Sprintf("%s %s (%s)", safeField, verb, strings.Join(placeholders, ", ")), items, nil
	case "like":
		text, ok := value.(string)
		if !ok {
			return "", nil, errors.New("datasource: like condition value must be string")
		}
		if !strings.Contains(text, "%") {
			text = "%" + text + "%"
		}
		return fmt.Sprintf("%s LIKE ?", safeField), []interface{}{text}, nil
	default:
		return fmt.Sprintf("%s %s ?", safeField, operator), []interface{}{value}, nil
	}
}

// isIgnoredValue 空值忽略规则。
func isIgnoredValue(value interface{}) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []string:
		return len(typed) == 0
	case []interface{}:
		return len(typed) == 0
	case []int:
		return len(typed) == 0
	case []int64:
		return len(typed) == 0
	}
	return false
}

// toSlice 转通用切片(in 条件用)。
func toSlice(value interface{}) ([]interface{}, bool) {
	switch typed := value.(type) {
	case []string:
		result := make([]interface{}, len(typed))
		for i, item := range typed {
			result[i] = item
		}
		return result, true
	case []int:
		result := make([]interface{}, len(typed))
		for i, item := range typed {
			result[i] = item
		}
		return result, true
	case []int64:
		result := make([]interface{}, len(typed))
		for i, item := range typed {
			result[i] = item
		}
		return result, true
	case []interface{}:
		return typed, true
	}
	return nil, false
}

// quoteField 字段名校验与引用(防注入:仅允许字母数字下划线点)。
func quoteField(field string) string {
	for _, r := range field {
		ok := r == '_' || r == '.' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return "1=0" // 非法字段名:返回永假条件,阻止注入
		}
	}
	return field
}

// sortStrings 简单排序(条件顺序稳定)。
func sortStrings(values []string) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}
