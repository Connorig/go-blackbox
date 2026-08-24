package datasource

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Connorig/go-blackbox/component/util"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestUtilTimeGORM util.Time 数据库字段 GORM 存取兼容。
func TestUtilTimeGORM(t *testing.T) {
	type timeRow struct {
		ID     int64     `gorm:"primarykey"`
		PaidAt util.Time `gorm:"column:paid_at"`
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "time.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(&timeRow{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	now := time.Now()
	row := timeRow{PaidAt: util.Time{Time: now}}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	var found timeRow
	if err := db.First(&found, row.ID).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if found.PaidAt.IsZero() {
		t.Fatal("paid_at must be scanned")
	}
	if found.PaidAt.Sub(now) > time.Second {
		t.Fatalf("time mismatch: %v vs %v", found.PaidAt, now)
	}
}
