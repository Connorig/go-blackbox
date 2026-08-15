package model

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// testStandardModel 是 StandardModel 测试表。
type testStandardModel struct {
	StandardModel
	Name string `gorm:"column:name;size:64"`
}

// TestStandardModelAutoFields 验证阿里规范字段自动维护（gmt_create/gmt_modified/is_deleted）。
func TestStandardModelAutoFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "standard.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&testStandardModel{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db failed: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	// 验证建表字段名符合阿里规范
	var columns []string
	if err := db.Raw(`SELECT name FROM pragma_table_info('test_standard_models')`).Scan(&columns).Error; err != nil {
		t.Fatalf("query columns failed: %v", err)
	}
	expected := map[string]bool{"id": false, "gmt_create": false, "gmt_modified": false, "is_deleted": false, "name": false}
	for _, column := range columns {
		if _, ok := expected[column]; ok {
			expected[column] = true
		}
	}
	for column, found := range expected {
		if !found {
			t.Fatalf("column %q must exist in table", column)
		}
	}

	// 创建:gmt_create/gmt_modified 自动填充
	row := &testStandardModel{Name: "first"}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if row.GmtCreate.IsZero() || row.GmtModified.IsZero() {
		t.Fatal("gmt_create/gmt_modified must be auto-filled on create")
	}

	// 更新:gmt_modified 自动刷新
	oldModified := row.GmtModified
	if err := db.Model(row).Updates(map[string]interface{}{"name": "updated"}).Error; err != nil {
		t.Fatalf("update failed: %v", err)
	}
	var refreshed testStandardModel
	if err := db.First(&refreshed, row.ID).Error; err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if !refreshed.GmtModified.After(oldModified) {
		t.Fatal("gmt_modified must refresh on update")
	}
	if refreshed.GmtCreate.Unix() != row.GmtCreate.Unix() {
		t.Fatalf("gmt_create must not change on update: %v vs %v", refreshed.GmtCreate, row.GmtCreate)
	}
	if refreshed.IsDeleted != 0 {
		t.Fatalf("is_deleted default must be 0, got %d", refreshed.IsDeleted)
	}
}

// TestStandardModelSoftDeleteFlag 验证手动软删除标记约定（is_deleted=1）。
func TestStandardModelSoftDeleteFlag(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "softdelete.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&testStandardModel{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db failed: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	row := &testStandardModel{Name: "to-delete"}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	// 业务侧逻辑删除:更新 is_deleted=1（与 gmt_modified 同步维护）
	if err := db.Model(row).Update("is_deleted", 1).Error; err != nil {
		t.Fatalf("soft delete failed: %v", err)
	}
	var count int64
	if err := db.Model(&testStandardModel{}).Where("is_deleted = 0").Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("soft-deleted row must be excluded, count=%d", count)
	}
}
