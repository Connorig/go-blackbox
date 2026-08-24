package datasource

import (
	"testing"
)

// TestBuildConditionBasic 等值/比较/like。
func TestBuildConditionBasic(t *testing.T) {
	condition, values, err := BuildCondition(map[string]interface{}{
		"client_name like": "connor",
		"grade":            10,
		"created_at >=":    "2026-01-01",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if condition != "client_name LIKE ? AND created_at >= ? AND grade = ?" {
		t.Fatalf("condition = %q", condition)
	}
	if len(values) != 3 {
		t.Fatalf("values = %v", values)
	}
	if values[0] != "%connor%" {
		t.Fatalf("like value = %v", values[0])
	}
}

// TestBuildConditionLikeWithPercent 已带 % 不重复加。
func TestBuildConditionLikeWithPercent(t *testing.T) {
	_, values, err := BuildCondition(map[string]interface{}{"name like": "%abc"})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if values[0] != "%abc" {
		t.Fatalf("value = %v", values[0])
	}
}

// TestBuildConditionIn in/not in 切片展开。
func TestBuildConditionIn(t *testing.T) {
	condition, values, err := BuildCondition(map[string]interface{}{
		"id in": []int64{1, 2, 3},
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if condition != "id IN (?, ?, ?)" {
		t.Fatalf("condition = %q", condition)
	}
	if len(values) != 3 {
		t.Fatalf("values = %v", values)
	}
}

// TestBuildConditionNull is null / is not null。
func TestBuildConditionNull(t *testing.T) {
	condition, values, err := BuildCondition(map[string]interface{}{
		"deleted_at is null": true,
		"remark is not null": true,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if condition != "deleted_at is null AND remark is not null" {
		t.Fatalf("condition = %q", condition)
	}
	if len(values) != 0 {
		t.Fatalf("values = %v", values)
	}
}

// TestBuildConditionIgnoreEmpty 空值忽略。
func TestBuildConditionIgnoreEmpty(t *testing.T) {
	condition, values, err := BuildCondition(map[string]interface{}{
		"name like": "",
		"status":    nil,
		"grade":     5,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if condition != "grade = ?" {
		t.Fatalf("condition = %q", condition)
	}
	if len(values) != 1 {
		t.Fatalf("values = %v", values)
	}
}

// TestBuildConditionEmptyParams 空参数。
func TestBuildConditionEmptyParams(t *testing.T) {
	condition, values, err := BuildCondition(nil)
	if err != nil || condition != "" || values != nil {
		t.Fatalf("empty params: %q %v %v", condition, values, err)
	}
}

// TestBuildConditionUnsupportedOperator 非法操作符报错。
func TestBuildConditionUnsupportedOperator(t *testing.T) {
	if _, _, err := BuildCondition(map[string]interface{}{"name xxx": 1}); err == nil {
		t.Fatal("unsupported operator must fail")
	}
}

// TestBuildConditionSQLInjection 字段名注入防护(报错或永假条件均可)。
func TestBuildConditionSQLInjection(t *testing.T) {
	condition, _, err := BuildCondition(map[string]interface{}{
		"name; DROP TABLE users": 1,
	})
	if err == nil && condition != "1=0 = ?" {
		t.Fatalf("injection not blocked: %q", condition)
	}
}
