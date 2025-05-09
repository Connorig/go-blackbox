package gormp

import (
	"fmt"
	"strings"
)

/**
* @Author: Connor
* @Date:   23.2.23 14:25
* @Description:
 */

// BuildCondition SQL条件复杂查询
func BuildCondition(where map[string]interface{}) (whereSql string,
	values []interface{}, err error) {
	for key, value := range where {
		conditionKey := strings.Split(key, " ")
		if len(conditionKey) > 2 {
			return "", nil, fmt.Errorf("" +
				"map构建的条件格式不对，类似于'age >'")
		}
		if whereSql != "" {
			whereSql += " AND "
		}
		switch len(conditionKey) {
		case 1:
			whereSql += fmt.Sprint(conditionKey[0], " = ?")
			values = append(values, value)
			break
		case 2:
			field := conditionKey[0]
			switch conditionKey[1] {
			case "=":
				whereSql += fmt.Sprint(field, " = ?")
				values = append(values, value)
				break
			case ">":
				whereSql += fmt.Sprint(field, " > ?")
				values = append(values, value)
				break
			case ">=":
				whereSql += fmt.Sprint(field, " >= ?")
				values = append(values, value)
				break
			case "<":
				whereSql += fmt.Sprint(field, " < ?")
				values = append(values, value)
				break
			case "<=":
				whereSql += fmt.Sprint(field, " <= ?")
				values = append(values, value)
				break
			case "in":
				whereSql += fmt.Sprint(field, " in (?)")
				valueStr := fmt.Sprintf("%s", value)
				if len(valueStr) > 1 {
					values = append(values, strings.Split(valueStr, ","))
				} else {
					values = append(values, value)
				}
				break
			case "like":
				whereSql += fmt.Sprint(field, " like ?")
				values = append(values, value)
				break
			case "<>":
				whereSql += fmt.Sprint(field, " != ?")
				values = append(values, value)
				break
			case "!=":
				whereSql += fmt.Sprint(field, " != ?")
				values = append(values, value)
				break
			}
			break
		}
	}
	return
}

// BuildConditionWithOr  SQL条件复杂查询
// isAddWhere 是否在开头添加 WHERE
func BuildConditionWithOr(isAddWhere bool, where map[string]interface{}) (whereSql string, values []interface{}, err error) {
	for key, value := range where {
		conditionKey := strings.Split(key, " ")

		if whereSql != "" {
			whereSql += " AND "
		} else {
			if isAddWhere {
				whereSql += " WHERE "
			}
		}

		if len(conditionKey) > 2 {
			if strings.Contains(key, "or") || strings.Contains(key, "OR") {
				field1 := conditionKey[0]
				fieldWhere1 := conditionKey[1]

				field2 := conditionKey[len(conditionKey)-2]
				fieldWhere2 := conditionKey[len(conditionKey)-1]

				whereSql += fmt.Sprint(" ( ", field1, " ", fieldWhere1, " ? ")
				values = append(values, value)
				whereSql += conditionKey[2] + " "
				whereSql += fmt.Sprint(field2, " ", fieldWhere2, " ? )  ")
				values = append(values, value)
				continue
			}
		}

		switch len(conditionKey) {
		case 1:
			whereSql += fmt.Sprint(conditionKey[0], " = ?")
			values = append(values, value)
			break
		case 2:
			field := conditionKey[0]
			switch conditionKey[1] {
			case "=":
				whereSql += fmt.Sprint(field, " = ?")
				values = append(values, value)
				break
			case ">":
				whereSql += fmt.Sprint(field, " > ?")
				values = append(values, value)
				break
			case ">=":
				whereSql += fmt.Sprint(field, " >= ?")
				values = append(values, value)
				break
			case "<":
				whereSql += fmt.Sprint(field, " < ?")
				values = append(values, value)
				break
			case "<=":
				whereSql += fmt.Sprint(field, " <= ?")
				values = append(values, value)
				break
			case "in":
				whereSql += fmt.Sprint(field, " in (?)")
				valueStr := fmt.Sprintf("%s", value)
				if len(valueStr) > 1 {
					values = append(values, strings.Split(valueStr, ","))
				} else {
					values = append(values, value)
				}
				break
			case "like":
				whereSql += fmt.Sprint(field, " like ?")
				values = append(values, value)
				break
			case "<>":
				whereSql += fmt.Sprint(field, " != ?")
				values = append(values, value)
				break
			case "!=":
				whereSql += fmt.Sprint(field, " != ?")
				values = append(values, value)
				break
			}
			break
		}
	}
	return
}
