package datasource

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// orgRow 组织数据测试表(对齐 OrgFields 字段名)。
type orgRow struct {
	ID     int64 `gorm:"primarykey;column:id"`
	OrgID  int64 `gorm:"column:org_id;index"`
	DeptID int64 `gorm:"column:dept_id;index"`
	Name   string `gorm:"column:name;size:64"`
}

// openScopeDB 打开临时 SQLite 并准备两组织数据。
func openScopeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "scope.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&orgRow{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db failed: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	rows := []orgRow{
		{OrgID: 1, DeptID: 10, Name: "org1-dept10"},
		{OrgID: 1, DeptID: 11, Name: "org1-dept11"},
		{OrgID: 2, DeptID: 20, Name: "org2-dept20"},
		{OrgID: 2, DeptID: 21, Name: "org2-dept21"},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed failed: %v", err)
		}
	}
	return db
}

// TestDataScopeConditionOrgOnly 仅组织过滤。
func TestDataScopeConditionOrgOnly(t *testing.T) {
	db := openScopeDB(t)
	scope := DataScope{OrgID: 1}
	var rows []orgRow
	if err := db.Scopes(scope.Condition()).Find(&rows).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("org 1 must have 2 rows, got %d", len(rows))
	}
	for _, row := range rows {
		if row.OrgID != 1 {
			t.Fatalf("leaked row from other org: %+v", row)
		}
	}
}

// TestDataScopeConditionOrgAndDept 组织+部门组合过滤。
func TestDataScopeConditionOrgAndDept(t *testing.T) {
	db := openScopeDB(t)
	scope := DataScope{OrgID: 1, DeptID: 11}
	var rows []orgRow
	if err := db.Scopes(scope.Condition()).Find(&rows).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "org1-dept11" {
		t.Fatalf("unexpected result: %+v", rows)
	}
}

// TestDataScopeConditionEmpty 空范围不产生过滤(全量可见,超管场景)。
func TestDataScopeConditionEmpty(t *testing.T) {
	db := openScopeDB(t)
	scope := DataScope{}
	if !scope.IsEmpty() {
		t.Fatal("empty scope must be detected")
	}
	var rows []orgRow
	if err := db.Scopes(scope.Condition()).Find(&rows).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("empty scope must return all rows, got %d", len(rows))
	}
}

// TestDataScopeWithContext 上下文注入与读取。
func TestDataScopeWithContext(t *testing.T) {
	ctx := WithScope(context.Background(), DataScope{OrgID: 9, DeptID: 99})
	scope, ok := ScopeFrom(ctx)
	if !ok {
		t.Fatal("scope must be readable")
	}
	if scope.OrgID != 9 || scope.DeptID != 99 {
		t.Fatalf("scope mismatch: %+v", scope)
	}
	// 未注入
	if _, ok := ScopeFrom(context.Background()); ok {
		t.Fatal("empty context must not have scope")
	}
	if must := MustScope(context.Background()); !must.IsEmpty() {
		t.Fatalf("MustScope must be empty, got %+v", must)
	}
	// nil context 安全
	if scope, ok := ScopeFrom(nil); ok || !scope.IsEmpty() {
		t.Fatal("ScopeFrom(nil) must be safe and empty")
	}
}

// TestDataScopeEndToEnd 端到端:注入 ctx → 查询自动隔离。
func TestDataScopeEndToEnd(t *testing.T) {
	db := openScopeDB(t)
	ctx := WithScope(context.Background(), DataScope{OrgID: 2})
	var rows []orgRow
	if err := db.WithContext(ctx).Scopes(MustScope(ctx).Condition()).Find(&rows).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("org 2 must have 2 rows, got %d", len(rows))
	}
}

// TestDataScopeConditionFor 自定义列名。
func TestDataScopeConditionFor(t *testing.T) {
	db := openScopeDB(t)
	scope := DataScope{OrgID: 2}
	var rows []orgRow
	if err := db.Scopes(scope.ConditionFor("org_id", "dept_id")).Find(&rows).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("custom columns must filter too, got %d", len(rows))
	}
}
