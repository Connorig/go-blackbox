// Package gencode 提供 Web 化低代码生成平台(对标 RuoYi 代码生成器):
// 在线查看数据库表/字段、编辑字段、同步表结构、一键生成 DDD 代码。
package gencode

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// ColumnMeta 字段元数据。
type ColumnMeta struct {
	Name       string `json:"name"`        // 列名
	DataType   string `json:"data_type"`   // 数据库类型(INTEGER/TEXT/VARCHAR...)
	Length     int    `json:"length"`      // 长度(0=无)
	Nullable   bool   `json:"nullable"`    // 是否可空
	Default    string `json:"default"`     // 默认值(空=无)
	Comment    string `json:"comment"`     // 注释
	PrimaryKey bool   `json:"primary_key"` // 是否主键
	AutoIncr   bool   `json:"auto_incr"`   // 是否自增
}

// TableMeta 表元数据。
type TableMeta struct {
	Name    string       `json:"name"`    // 表名
	Comment string       `json:"comment"` // 表注释
	Columns []ColumnMeta `json:"columns"` // 字段
}

// Meta 元数据读取器(按驱动分支)。
type Meta struct {
	db *gorm.DB
}

// NewMeta 创建元数据读取器。
func NewMeta(db *gorm.DB) *Meta {
	return &Meta{db: db}
}

// DB 返回底层数据库(生成记录表使用)。
func (m *Meta) DB() *gorm.DB {
	if m == nil {
		return nil
	}
	return m.db
}

// dialect 当前数据库方言。
func (m *Meta) dialect() string {
	if m == nil || m.db == nil {
		return ""
	}
	return m.db.Dialector.Name()
}

// ListTables 读取全部业务表(排除系统表)。
func (m *Meta) ListTables(ctx context.Context) ([]TableMeta, error) {
	if m == nil || m.db == nil {
		return nil, errors.New("gencode: meta db is nil")
	}
	switch m.dialect() {
	case "sqlite":
		return m.listTablesSQLite(ctx)
	case "postgres", "postgresql":
		return m.listTablesPostgres(ctx)
	case "mysql":
		return m.listTablesMySQL(ctx)
	default:
		return nil, fmt.Errorf("gencode: unsupported dialect %q", m.dialect())
	}
}

// ReadTable 读取单表完整元数据(字段)。
func (m *Meta) ReadTable(ctx context.Context, tableName string) (*TableMeta, error) {
	tables, err := m.ListTables(ctx)
	if err != nil {
		return nil, err
	}
	for i := range tables {
		if tables[i].Name == tableName {
			return &tables[i], nil
		}
	}
	return nil, fmt.Errorf("gencode: table %q not found", tableName)
}

// sqliteTables SQLite:sqlite_master + PRAGMA table_info。
func (m *Meta) listTablesSQLite(ctx context.Context) ([]TableMeta, error) {
	var rows []struct {
		Name string
		SQL  string
	}
	if err := m.db.WithContext(ctx).Raw(
		`SELECT name, sql FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	tables := make([]TableMeta, 0, len(rows))
	for _, row := range rows {
		table := TableMeta{Name: row.Name}
		columns, err := m.readTableColumnsSQLite(ctx, row.Name)
		if err != nil {
			return nil, err
		}
		table.Columns = columns
		tables = append(tables, table)
	}
	return tables, nil
}

// readTableColumnsSQLite PRAGMA table_info。
func (m *Meta) readTableColumnsSQLite(ctx context.Context, tableName string) ([]ColumnMeta, error) {
	var rows []struct {
		CID      int
		Name     string
		Type     string
		NotNull  int
		Default  *string
		PK       int
	}
	if err := m.db.WithContext(ctx).Raw("PRAGMA table_info(" + quoteIdent(tableName) + ")").Scan(&rows).Error; err != nil {
		return nil, err
	}
	columns := make([]ColumnMeta, 0, len(rows))
	for _, row := range rows {
		col := ColumnMeta{
			Name:       row.Name,
			DataType:   normalizeType(row.Type),
			PrimaryKey: row.PK > 0,
			Nullable:   row.NotNull == 0,
			AutoIncr:   row.PK > 0,
		}
		if row.Default != nil {
			col.Default = *row.Default
		}
		columns = append(columns, col)
	}
	return columns, nil
}

// listTablesPostgres PostgreSQL:information_schema + 注释。
func (m *Meta) listTablesPostgres(ctx context.Context) ([]TableMeta, error) {
	var rows []struct {
		TableName string
		Comment   *string
	}
	if err := m.db.WithContext(ctx).Raw(`
		SELECT t.table_name,
		       obj_description(c.oid) AS comment
		FROM information_schema.tables t
		LEFT JOIN pg_class c ON c.relname = t.table_name
		WHERE t.table_schema = 'public' AND t.table_type = 'BASE TABLE'
		ORDER BY t.table_name`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	tables := make([]TableMeta, 0, len(rows))
	for _, row := range rows {
		table := TableMeta{Name: row.TableName}
		if row.Comment != nil {
			table.Comment = *row.Comment
		}
		columns, err := m.readTableColumnsPostgres(ctx, row.TableName)
		if err != nil {
			return nil, err
		}
		table.Columns = columns
		tables = append(tables, table)
	}
	return tables, nil
}

// readTableColumnsPostgres information_schema.columns。
func (m *Meta) readTableColumnsPostgres(ctx context.Context, tableName string) ([]ColumnMeta, error) {
	var rows []struct {
		ColumnName    string
		DataType      string
		CharMaxLength *int
		IsNullable    string
		ColumnDefault *string
		Comment       *string
	}
	if err := m.db.WithContext(ctx).Raw(`
		SELECT c.column_name, c.data_type, c.character_maximum_length,
		       c.is_nullable, c.column_default,
		       col_description(format('%I.%I', c.table_schema, c.table_name)::regclass::oid, c.ordinal_position) AS comment
		FROM information_schema.columns c
		WHERE c.table_schema = 'public' AND c.table_name = ?
		ORDER BY c.ordinal_position`, tableName).Scan(&rows).Error; err != nil {
		return nil, err
	}
	columns := make([]ColumnMeta, 0, len(rows))
	for _, row := range rows {
		col := ColumnMeta{
			Name:     row.ColumnName,
			DataType: normalizeType(row.DataType),
			Nullable: row.IsNullable == "YES",
		}
		if row.CharMaxLength != nil {
			col.Length = *row.CharMaxLength
		}
		if row.ColumnDefault != nil {
			col.Default = *row.ColumnDefault
		}
		if row.Comment != nil {
			col.Comment = *row.Comment
		}
		columns = append(columns, col)
	}
	return columns, nil
}

// listTablesMySQL MySQL:information_schema。
func (m *Meta) listTablesMySQL(ctx context.Context) ([]TableMeta, error) {
	var rows []struct {
		TableName string
		Comment   string
	}
	if err := m.db.WithContext(ctx).Raw(`
		SELECT table_name, table_comment FROM information_schema.tables
		WHERE table_schema = DATABASE() ORDER BY table_name`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	tables := make([]TableMeta, 0, len(rows))
	for _, row := range rows {
		table := TableMeta{Name: row.TableName, Comment: row.Comment}
		columns, err := m.readTableColumnsMySQL(ctx, row.TableName)
		if err != nil {
			return nil, err
		}
		table.Columns = columns
		tables = append(tables, table)
	}
	return tables, nil
}

// readTableColumnsMySQL information_schema.columns。
func (m *Meta) readTableColumnsMySQL(ctx context.Context, tableName string) ([]ColumnMeta, error) {
	var rows []struct {
		ColumnName    string
		DataType      string
		CharMaxLength *int
		IsNullable    string
		ColumnDefault *string
		ColumnKey     string
		Extra         string
		ColumnComment string
	}
	if err := m.db.WithContext(ctx).Raw(`
		SELECT column_name, data_type, character_maximum_length, is_nullable,
		       column_default, column_key, extra, column_comment
		FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ?
		ORDER BY ordinal_position`, tableName).Scan(&rows).Error; err != nil {
		return nil, err
	}
	columns := make([]ColumnMeta, 0, len(rows))
	for _, row := range rows {
		col := ColumnMeta{
			Name:       row.ColumnName,
			DataType:   normalizeType(row.DataType),
			Nullable:   row.IsNullable == "YES",
			Comment:    row.ColumnComment,
			PrimaryKey: row.ColumnKey == "PRI",
			AutoIncr:   strings.Contains(row.Extra, "auto_increment"),
		}
		if row.CharMaxLength != nil {
			col.Length = *row.CharMaxLength
		}
		if row.ColumnDefault != nil {
			col.Default = *row.ColumnDefault
		}
		columns = append(columns, col)
	}
	return columns, nil
}

// normalizeType 规范化类型名。
func normalizeType(dbType string) string {
	upper := strings.ToUpper(strings.TrimSpace(dbType))
	// 提取括号前的类型名(VARCHAR(64) → VARCHAR)
	if idx := strings.Index(upper, "("); idx > 0 {
		upper = upper[:idx]
	}
	return upper
}

// quoteIdent 标识符加引号(防注入)。
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
