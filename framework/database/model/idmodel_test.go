package model

import (
	"path/filepath"
	"testing"
	"time"

	sid "github.com/Connorig/go-blackbox/framework/database/id"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// testSnowflakeRow 雪花模型测试表(叠加组织字段,验证组合用法)。
type testSnowflakeRow struct {
	SnowflakeModel
	OrgFields
	Name string `gorm:"column:name;size:64"`
}

// testStringIDRow 字符串 ID 模型测试表。
type testStringIDRow struct {
	StringIDModel
	Name string `gorm:"column:name;size:64"`
}

// TestSnowflakeModelAutoID 验证雪花 ID 自动生成与时间维护。
func TestSnowflakeModelAutoID(t *testing.T) {
	db := openModelDB(t, &testSnowflakeRow{})
	row := &testSnowflakeRow{Name: "snow", OrgFields: OrgFields{OrgID: 10, DeptID: 20}}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if row.ID <= 0 {
		t.Fatal("snowflake id must be auto-generated")
	}
	if _, _, _, err := sid.ParseSnowflake(row.ID); err != nil {
		t.Fatalf("generated id is not a valid snowflake: %v", err)
	}
	if row.GmtCreate.IsZero() || row.GmtModified.IsZero() {
		t.Fatal("gmt_create/gmt_modified must be auto-filled")
	}
	if row.OrgID != 10 || row.DeptID != 20 {
		t.Fatalf("org fields lost: org=%d dept=%d", row.OrgID, row.DeptID)
	}

	// 更新自动刷新 gmt_modified
	old := row.GmtModified
	time.Sleep(1100 * time.Millisecond) // SQLite 秒级精度
	if err := db.Model(row).Updates(map[string]interface{}{"name": "snow2"}).Error; err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if !row.GmtModified.After(old) {
		t.Fatal("gmt_modified must be refreshed on update")
	}
}

// TestSnowflakeModelExplicitID 显式赋值优先,不覆盖。
func TestSnowflakeModelExplicitID(t *testing.T) {
	db := openModelDB(t, &testSnowflakeRow{})
	row := &testSnowflakeRow{SnowflakeModel: SnowflakeModel{ID: 42}, Name: "explicit"}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if row.ID != 42 {
		t.Fatalf("explicit id must be kept, got %d", row.ID)
	}
}

// TestStringIDModelAutoID 验证 UUID 主键自动生成。
func TestStringIDModelAutoID(t *testing.T) {
	db := openModelDB(t, &testStringIDRow{})
	row := &testStringIDRow{Name: "uuid-row"}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if len(row.ID) != 36 {
		t.Fatalf("uuid id must be 36 chars, got %q", row.ID)
	}
	if row.GmtCreate.IsZero() || row.GmtModified.IsZero() {
		t.Fatal("gmt_create/gmt_modified must be auto-filled")
	}
	// 唯一性:连插两条
	second := &testStringIDRow{Name: "uuid-row-2"}
	if err := db.Create(second).Error; err != nil {
		t.Fatalf("create second failed: %v", err)
	}
	if second.ID == row.ID {
		t.Fatal("two rows must not share the same uuid")
	}
}

// TestCompositePrimaryKey 验证组合主键(订单明细场景)。
func TestCompositePrimaryKey(t *testing.T) {
	type orderItem struct {
		OrderID  int64 `gorm:"primarykey;column:order_id"`
		ItemSeq  int   `gorm:"primarykey;column:item_seq"`
		SkuID    int64 `gorm:"column:sku_id"`
		Quantity int   `gorm:"column:quantity"`
	}
	db := openModelDB(t, &orderItem{})
	item := &orderItem{OrderID: 1, ItemSeq: 1, SkuID: 100, Quantity: 2}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create composite pk failed: %v", err)
	}
	var found orderItem
	if err := db.First(&found, "order_id = ? AND item_seq = ?", 1, 1).Error; err != nil {
		t.Fatalf("query composite pk failed: %v", err)
	}
	if found.SkuID != 100 {
		t.Fatalf("composite pk row lost: %+v", found)
	}
}

// openModelDB 打开临时 SQLite 并迁移表。
func openModelDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "model.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db failed: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
