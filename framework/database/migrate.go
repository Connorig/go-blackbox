package datasource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Migration 定义一次数据库结构迁移。
// Name 必须全局唯一；Up 在事务中执行并记录版本，Down 用于回滚（可选）。
type Migration struct {
	// Name 是迁移名称（如 "20260815_create_orders"），重复名称会被拒绝。
	Name string
	// Up 执行迁移内容。
	Up func(db *gorm.DB) error
	// Down 回滚迁移内容；为 nil 时该迁移不可回滚。
	Down func(db *gorm.DB) error
}

// Migrator 是轻量版本化迁移执行器。
// 迁移记录保存在 schema_migrations 表（可按需改名）。
type Migrator struct {
	instance  *Instance
	migrations []Migration
	tableName string
}

// NewMigrator 创建迁移器。
// 迁移按传入顺序执行，未应用的部分在 Migrate 时按序补齐。
func NewMigrator(instance *Instance, migrations ...Migration) *Migrator {
	return &Migrator{
		instance:   instance,
		migrations: migrations,
		tableName:  "schema_migrations",
	}
}

// WithTableName 覆盖默认迁移记录表名。
func (m *Migrator) WithTableName(tableName string) *Migrator {
	if strings.TrimSpace(tableName) != "" {
		m.tableName = strings.TrimSpace(tableName)
	}
	return m
}

// Migrate 应用全部未执行的迁移（每个迁移在独立事务中执行）。
// 已应用的迁移会被跳过，可重复调用。
func (m *Migrator) Migrate(ctx context.Context) error {
	if m == nil || m.instance == nil {
		return errors.New("migrate: migrator instance is nil")
	}
	if ctx == nil {
		return errors.New("migrate: context is nil")
	}
	db := m.instance.DB()
	if db == nil {
		return errors.New("migrate: database instance is closed")
	}

	if err := m.ensureMigrationTable(ctx, db); err != nil {
		return err
	}
	// 校验同批迁移名称唯一，避免运行时重复应用。
	seen := make(map[string]bool, len(m.migrations))
	for _, migration := range m.migrations {
		if migration.Name == "" {
			return errors.New("migrate: migration name is empty")
		}
		if seen[migration.Name] {
			return fmt.Errorf("migrate: duplicate migration name %q", migration.Name)
		}
		seen[migration.Name] = true
	}
	applied, err := m.appliedNames(ctx, db)
	if err != nil {
		return err
	}

	for _, migration := range m.migrations {
		if applied[migration.Name] {
			continue
		}
		if migration.Up == nil {
			return fmt.Errorf("migrate %q: up function is nil", migration.Name)
		}
		if err := m.applyOne(ctx, db, migration); err != nil {
			return err
		}
	}
	return nil
}

// Status 返回已应用的迁移名称列表。
func (m *Migrator) Status(ctx context.Context) ([]string, error) {
	if m == nil || m.instance == nil {
		return nil, errors.New("migration status: migrator instance is nil")
	}
	db := m.instance.DB()
	if db == nil {
		return nil, errors.New("migration status: database instance is closed")
	}
	if err := m.ensureMigrationTable(ctx, db); err != nil {
		return nil, err
	}
	applied, err := m.appliedNames(ctx, db)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(applied))
	for name := range applied {
		names = append(names, name)
	}
	return names, nil
}

// Rollback 回滚最近一个已应用且提供 Down 的迁移。
// 没有可回滚的迁移时返回明确错误。
func (m *Migrator) Rollback(ctx context.Context) error {
	if m == nil || m.instance == nil {
		return errors.New("rollback: migrator instance is nil")
	}
	db := m.instance.DB()
	if db == nil {
		return errors.New("rollback: database instance is closed")
	}
	if err := m.ensureMigrationTable(ctx, db); err != nil {
		return err
	}
	applied, err := m.appliedNames(ctx, db)
	if err != nil {
		return err
	}

	// 从后向前找最近一个已应用迁移
	for index := len(m.migrations) - 1; index >= 0; index-- {
		migration := m.migrations[index]
		if !applied[migration.Name] {
			continue
		}
		if migration.Down == nil {
			return fmt.Errorf("rollback %q: down function is not defined", migration.Name)
		}
		return m.rollbackOne(ctx, db, migration)
	}
	return errors.New("rollback: no applied migration found")
}

// migrationRow 是迁移记录表结构。
type migrationRow struct {
	Name      string    `gorm:"primarykey;size:255"`
	AppliedAt time.Time
}

// ensureMigrationTable 创建迁移记录表（原生 SQL，表名与读写路径一致）。
func (m *Migrator) ensureMigrationTable(ctx context.Context, db *gorm.DB) error {
	createSQL := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s (name VARCHAR(255) PRIMARY KEY, applied_at TIMESTAMP NOT NULL)",
		m.tableName)
	if err := db.WithContext(ctx).Exec(createSQL).Error; err != nil {
		return fmt.Errorf("create migration table %q: %w", m.tableName, err)
	}
	return nil
}

// appliedNames 返回已应用迁移名称集合。
func (m *Migrator) appliedNames(ctx context.Context, db *gorm.DB) (map[string]bool, error) {
	var rows []migrationRow
	if err := db.WithContext(ctx).Table(m.tableName).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}
	result := make(map[string]bool, len(rows))
	for _, row := range rows {
		result[row.Name] = true
	}
	return result, nil
}

// applyOne 在事务中执行单个迁移并记录版本。
func (m *Migrator) applyOne(ctx context.Context, db *gorm.DB, migration Migration) error {
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := migration.Up(tx); err != nil {
			return err
		}
		return tx.Table(m.tableName).Create(&migrationRow{Name: migration.Name, AppliedAt: time.Now()}).Error
	})
	if err != nil {
		return fmt.Errorf("apply migration %q: %w", migration.Name, err)
	}
	return nil
}

// rollbackOne 在事务中执行 Down 并删除版本记录。
func (m *Migrator) rollbackOne(ctx context.Context, db *gorm.DB, migration Migration) error {
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := migration.Down(tx); err != nil {
			return err
		}
		return tx.Table(m.tableName).Where("name = ?", migration.Name).Delete(&migrationRow{}).Error
	})
	if err != nil {
		return fmt.Errorf("rollback migration %q: %w", migration.Name, err)
	}
	return nil
}
