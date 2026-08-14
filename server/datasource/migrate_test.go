package datasource

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"
)

// migrationTestModel 是迁移测试使用的表模型。
type migrationTestModel struct {
	ID   int    `gorm:"primarykey"`
	Name string `gorm:"size:64"`
}

// testMigratorInstance 创建 SQLite 实例并返回迁移器。
func testMigratorInstance(t *testing.T) (*Instance, *Migrator) {
	t.Helper()
	withCleanRegistry(t)
	instance, err := NewNamed(context.Background(), "migrate", testSQLiteConfig(t, "migrate"))
	if err != nil {
		t.Fatalf("create instance failed: %v", err)
	}
	t.Cleanup(func() { _ = instance.Close() })
	return instance, NewMigrator(instance)
}

// TestMigrateAppliesInOrder 验证迁移按顺序应用并建表。
func TestMigrateAppliesInOrder(t *testing.T) {
	instance, migrator := testMigratorInstance(t)

	appliedOrder := []string{}
	migrator.migrations = []Migration{
		{
			Name: "create_first",
			Up: func(db *gorm.DB) error {
				appliedOrder = append(appliedOrder, "first")
				return db.Exec(`CREATE TABLE first_table (id INTEGER PRIMARY KEY)`).Error
			},
		},
		{
			Name: "create_second",
			Up: func(db *gorm.DB) error {
				appliedOrder = append(appliedOrder, "second")
				return db.Exec(`CREATE TABLE second_table (id INTEGER PRIMARY KEY)`).Error
			},
		},
	}

	if err := migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if len(appliedOrder) != 2 || appliedOrder[0] != "first" || appliedOrder[1] != "second" {
		t.Fatalf("migrations must apply in order: %v", appliedOrder)
	}

	// 表确实创建
	var tableCount int64
	if err := instance.DB().Raw(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='second_table'`).Scan(&tableCount).Error; err != nil {
		t.Fatalf("query table failed: %v", err)
	}
	if tableCount != 1 {
		t.Fatalf("migration table was not created: %d", tableCount)
	}
}

// TestMigrateIsIdempotent 验证重复执行跳过已应用迁移。
func TestMigrateIsIdempotent(t *testing.T) {
	_, migrator := testMigratorInstance(t)
	runCount := 0
	migrator.migrations = []Migration{
		{
			Name: "once",
			Up: func(db *gorm.DB) error {
				runCount++
				return nil
			},
		},
	}

	if err := migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("first migrate failed: %v", err)
	}
	if err := migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("second migrate failed: %v", err)
	}
	if runCount != 1 {
		t.Fatalf("migration must run once, got %d", runCount)
	}

	names, err := migrator.Status(context.Background())
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if len(names) != 1 || names[0] != "once" {
		t.Fatalf("unexpected applied migrations: %v", names)
	}
}

// TestMigrateRejectsEmptyName 验证空迁移名被拒绝。
func TestMigrateRejectsEmptyName(t *testing.T) {
	_, migrator := testMigratorInstance(t)
	migrator.migrations = []Migration{{Name: "", Up: func(db *gorm.DB) error { return nil }}}
	if err := migrator.Migrate(context.Background()); err == nil {
		t.Fatal("empty migration name must return an error")
	}
}

// TestRollbackRevertsLatestMigration 验证回滚最近迁移并删除记录。
func TestRollbackRevertsLatestMigration(t *testing.T) {
	instance, migrator := testMigratorInstance(t)
	rolledBack := false
	migrator.migrations = []Migration{
		{
			Name: "create_rollback_table",
			Up: func(db *gorm.DB) error {
				return db.Exec(`CREATE TABLE rollback_table (id INTEGER PRIMARY KEY)`).Error
			},
			Down: func(db *gorm.DB) error {
				rolledBack = true
				return db.Exec(`DROP TABLE rollback_table`).Error
			},
		},
	}

	if err := migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if err := migrator.Rollback(context.Background()); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if !rolledBack {
		t.Fatal("down function must be called on rollback")
	}

	names, err := migrator.Status(context.Background())
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("migration record must be removed after rollback: %v", names)
	}

	var tableCount int64
	if err := instance.DB().Raw(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='rollback_table'`).Scan(&tableCount).Error; err != nil {
		t.Fatalf("query table failed: %v", err)
	}
	if tableCount != 0 {
		t.Fatal("rolled back table must be dropped")
	}
}

// TestMigrateWithRealModels 验证迁移与 GORM AutoMigrate 配合建表。
func TestMigrateWithRealModels(t *testing.T) {
	instance, migrator := testMigratorInstance(t)
	migrator.migrations = []Migration{
		{
			Name: "create_models",
			Up: func(db *gorm.DB) error {
				return db.AutoMigrate(&migrationTestModel{})
			},
		},
	}

	if err := migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if err := instance.DB().Create(&migrationTestModel{Name: "row-1"}).Error; err != nil {
		t.Fatalf("insert after migrate failed: %v", err)
	}
}

// TestMigratorNilSafety 验证 nil 迁移器方法返回明确错误。
func TestMigratorNilSafety(t *testing.T) {
	var migrator *Migrator
	if err := migrator.Migrate(context.Background()); err == nil {
		t.Fatal("Migrate on nil migrator must return an error")
	}
	if _, err := migrator.Status(context.Background()); err == nil {
		t.Fatal("Status on nil migrator must return an error")
	}
	if err := migrator.Rollback(context.Background()); err == nil {
		t.Fatal("Rollback on nil migrator must return an error")
	}
}

// TestMigrationNameUniquenessWithinRun 验证同批重名迁移在运行前被拒绝。
func TestMigrationNameUniquenessWithinRun(t *testing.T) {
	_, migrator := testMigratorInstance(t)
	migrator.migrations = []Migration{
		{Name: "dup", Up: func(db *gorm.DB) error { return nil }},
		{Name: "dup", Up: func(db *gorm.DB) error { return errors.New("must not run") }},
	}
	if err := migrator.Migrate(context.Background()); err == nil {
		t.Fatal("duplicate migration names must be rejected before running")
	}
}

