package datasource

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
)

// auditModel 是多实例测试使用的 GORM Model。
type auditModel struct {
	ID   int    `gorm:"primarykey"`
	Name string `gorm:"size:64"`
	Age  int
}

// withCleanRegistry 保存并清空全局实例注册表，测试结束后恢复并关闭测试实例。
func withCleanRegistry(t *testing.T) {
	t.Helper()
	instancesMu.Lock()
	previousDefault := defaultInstance
	previousInstances := instances
	defaultInstance = nil
	instances = map[string]*Instance{}
	instancesMu.Unlock()
	t.Cleanup(func() {
		instancesMu.RLock()
		created := make([]*Instance, 0, len(instances))
		for _, instance := range instances {
			created = append(created, instance)
		}
		instancesMu.RUnlock()
		for _, instance := range created {
			_ = instance.Close()
		}
		instancesMu.Lock()
		defaultInstance = previousDefault
		instances = previousInstances
		instancesMu.Unlock()
	})
}
// testSQLiteConfig 创建指向临时文件的 SQLite 配置。
func testSQLiteConfig(t *testing.T, name string) *Config {
	t.Helper()
	return &Config{
		Driver:       DriverSQLite,
		DSN:          filepath.Join(t.TempDir(), name+".db"),
		AutoMigrate:  true,
		MaxIdleConns: 1,
		MaxOpenConns: 2,
	}
}

// TestSQLiteDefaultInstance 验证默认实例创建、获取与数据写入。
func TestSQLiteDefaultInstance(t *testing.T) {
	withCleanRegistry(t)
	ctx := context.Background()
	instance, err := New(ctx, testSQLiteConfig(t, "default"), &auditModel{})
	if err != nil {
		t.Fatalf("create default instance failed: %v", err)
	}
	t.Cleanup(func() { _ = instance.Close() })

	got, err := Get()
	if err != nil {
		t.Fatalf("get default instance failed: %v", err)
	}
	if got != instance {
		t.Fatal("Get must return the registered default instance")
	}
	if got.Name() != "" {
		t.Fatalf("unexpected default instance name: %q", got.Name())
	}
	if got.Driver() != DriverSQLite {
		t.Fatalf("unexpected driver: %s", got.Driver())
	}

	db := got.DB()
	if err := db.Create(&auditModel{Name: "first", Age: 1}).Error; err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	var count int64
	if err := db.Model(&auditModel{}).Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("unexpected row count: %d", count)
	}
}

// TestSQLiteNamedInstances 验证默认实例与多个具名实例可并行共存。
func TestSQLiteNamedInstances(t *testing.T) {
	withCleanRegistry(t)
	ctx := context.Background()
	defaultInstance, err := New(ctx, testSQLiteConfig(t, "main"), &auditModel{})
	if err != nil {
		t.Fatalf("create default instance failed: %v", err)
	}
	t.Cleanup(func() { _ = defaultInstance.Close() })

	legacy, err := NewNamed(ctx, "legacy", testSQLiteConfig(t, "legacy"), &auditModel{})
	if err != nil {
		t.Fatalf("create named instance failed: %v", err)
	}
	t.Cleanup(func() { _ = legacy.Close() })

	reporting, err := NewNamed(ctx, "reporting", testSQLiteConfig(t, "reporting"), &auditModel{})
	if err != nil {
		t.Fatalf("create second named instance failed: %v", err)
	}
	t.Cleanup(func() { _ = reporting.Close() })

	gotLegacy, err := GetNamed("legacy")
	if err != nil {
		t.Fatalf("get named instance failed: %v", err)
	}
	if gotLegacy != legacy || gotLegacy.Name() != "legacy" {
		t.Fatal("GetNamed must return the registered named instance")
	}
	if _, err := GetNamed("missing"); err == nil {
		t.Fatal("GetNamed for missing instance must return an error")
	}

	// 三个实例相互独立
	if err := defaultInstance.DB().Create(&auditModel{Name: "main-row"}).Error; err != nil {
		t.Fatalf("insert into default failed: %v", err)
	}
	if err := legacy.DB().Create(&auditModel{Name: "legacy-row"}).Error; err != nil {
		t.Fatalf("insert into legacy failed: %v", err)
	}
	var defaultCount, legacyCount int64
	_ = defaultInstance.DB().Model(&auditModel{}).Count(&defaultCount).Error
	_ = legacy.DB().Model(&auditModel{}).Count(&legacyCount).Error
	if defaultCount != 1 || legacyCount != 1 {
		t.Fatalf("instances must be isolated: default=%d legacy=%d", defaultCount, legacyCount)
	}

	if len(Instances()) != 3 {
		t.Fatalf("expected 3 registered instances, got %d", len(Instances()))
	}
}

// TestSQLiteDuplicateName 验证同名实例重复注册返回错误。
func TestSQLiteDuplicateName(t *testing.T) {
	withCleanRegistry(t)
	ctx := context.Background()
	first, err := NewNamed(ctx, "dup", testSQLiteConfig(t, "dup1"), &auditModel{})
	if err != nil {
		t.Fatalf("create first named instance failed: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })

	if _, err := NewNamed(ctx, "dup", testSQLiteConfig(t, "dup2"), &auditModel{}); err == nil {
		t.Fatal("duplicate named instance must return an error")
	}
}

// TestSQLiteInstanceHealthAndClose 验证 Health 与 Close 的幂等行为。
func TestSQLiteInstanceHealthAndClose(t *testing.T) {
	withCleanRegistry(t)
	ctx := context.Background()
	instance, err := NewNamed(ctx, "health", testSQLiteConfig(t, "health"), &auditModel{})
	if err != nil {
		t.Fatalf("create instance failed: %v", err)
	}

	if err := instance.Health(ctx); err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	if err := instance.Close(); err != nil {
		t.Fatalf("close instance failed: %v", err)
	}
	if err := instance.Close(); err != nil {
		t.Fatalf("second close must be idempotent: %v", err)
	}
	if err := instance.Health(ctx); err == nil {
		t.Fatal("health check on closed instance must return an error")
	}
	if instance.DB() != nil {
		t.Fatal("DB must return nil after close")
	}
}

// TestWithTxRollsBackOnError 验证事务失败自动回滚。
func TestWithTxRollsBackOnError(t *testing.T) {
	withCleanRegistry(t)
	ctx := context.Background()
	instance, err := NewNamed(ctx, "tx", testSQLiteConfig(t, "tx"), &auditModel{})
	if err != nil {
		t.Fatalf("create instance failed: %v", err)
	}
	t.Cleanup(func() { _ = instance.Close() })

	err = instance.WithTx(ctx, func(tx *gorm.DB) error {
		if tx == nil {
			return errors.New("transaction db is nil")
		}
		if err := tx.Create(&auditModel{Name: "rollback-row"}).Error; err != nil {
			return err
		}
		return errors.New("trigger rollback")
	})
	if err == nil {
		t.Fatal("transaction must propagate the error")
	}

	var count int64
	if err := instance.DB().Model(&auditModel{}).Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("rolled back transaction must leave no rows, got %d", count)
	}

	// 成功事务应提交
	err = instance.WithTx(ctx, func(tx *gorm.DB) error {
		return tx.Create(&auditModel{Name: "committed-row"}).Error
	})
	if err != nil {
		t.Fatalf("successful transaction failed: %v", err)
	}
	if err := instance.DB().Model(&auditModel{}).Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("committed transaction must leave one row, got %d", count)
	}
}

// TestPageQuery 验证分页查询的总数与页数据。
func TestPageQuery(t *testing.T) {
	withCleanRegistry(t)
	ctx := context.Background()
	instance, err := NewNamed(ctx, "page", testSQLiteConfig(t, "page"), &auditModel{})
	if err != nil {
		t.Fatalf("create instance failed: %v", err)
	}
	t.Cleanup(func() { _ = instance.Close() })

	for index := 1; index <= 25; index++ {
		if err := instance.DB().Create(&auditModel{Name: "row", Age: index}).Error; err != nil {
			t.Fatalf("insert row %d failed: %v", index, err)
		}
	}

	var pageRows []auditModel
	result, err := PageOn(instance.DB().Model(&auditModel{}).Order("age"), ctx, instance.DB().Model(&auditModel{}), 2, 10, &pageRows)
	if err != nil {
		t.Fatalf("page query failed: %v", err)
	}
	if result.Total != 25 || result.TotalPages != 3 || result.Page != 2 || result.PageSize != 10 {
		t.Fatalf("unexpected page result: %+v", result)
	}
	if len(pageRows) != 10 {
		t.Fatalf("expected 10 rows on page 2, got %d", len(pageRows))
	}
	if pageRows[0].Age != 11 {
		t.Fatalf("unexpected first row age on page 2: %d", pageRows[0].Age)
	}
}

// TestSQLiteConfigRequiresDSN 验证 SQLite 缺少 DSN 时返回明确错误。
func TestSQLiteConfigRequiresDSN(t *testing.T) {
	withCleanRegistry(t)
	if _, err := normalizeDatabaseConfig(&Config{Driver: DriverSQLite}); err == nil {
		t.Fatal("SQLite config without DSN must return an error")
	}
}
