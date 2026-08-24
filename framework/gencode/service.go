package gencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

// 生成记录表名。
const recordTableName = "gbx_gen_record"

// genRecord 已生成标记(持久化:再次生成提示覆盖)。
type genRecord struct {
	Table       string    `gorm:"column:table_name;primaryKey"`
	GeneratedAt time.Time `gorm:"column:generated_at"`
	FilesJSON   string    `gorm:"column:files_json"`  // 文件路径列表
	HashesJSON  string    `gorm:"column:hashes_json"` // 路径→哈希
}

// TableName 记录表名。
func (genRecord) TableName() string { return recordTableName }

// Service 低代码生成服务。
type Service struct {
	meta       *Meta
	modulePath string // 业务项目 module 路径(import 用)
	outputDir  string // 生成文件输出根目录(默认 ./)
}

// NewService 创建生成服务。
func NewService(db *gorm.DB, modulePath, outputDir string) (*Service, error) {
	if db == nil {
		return nil, errors.New("gencode: db is nil")
	}
	if modulePath == "" {
		return nil, errors.New("gencode: module path is required")
	}
	if outputDir == "" {
		outputDir = "."
	}
	service := &Service{
		meta:       NewMeta(db),
		modulePath: modulePath,
		outputDir:  outputDir,
	}
	// 初始化记录表
	if err := db.AutoMigrate(&genRecord{}); err != nil {
		return nil, fmt.Errorf("gencode: init record table: %w", err)
	}
	return service, nil
}

// TableInfo 表 + 已生成标记。
type TableInfo struct {
	TableMeta
	Generated   bool       `json:"generated"`
	GeneratedAt *time.Time `json:"generated_at,omitempty"`
}

// ListTables 表列表(含生成标记)。
func (s *Service) ListTables(ctx context.Context) ([]TableInfo, error) {
	tables, err := s.meta.ListTables(ctx)
	if err != nil {
		return nil, err
	}
	records, err := s.records(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]TableInfo, 0, len(tables))
	for _, table := range tables {
		info := TableInfo{TableMeta: table}
		if record, ok := records[table.Name]; ok {
			info.Generated = true
			info.GeneratedAt = &record.GeneratedAt
		}
		result = append(result, info)
	}
	return result, nil
}

// ReadTable 表详情(字段)。
func (s *Service) ReadTable(ctx context.Context, tableName string) (*TableMeta, error) {
	return s.meta.ReadTable(ctx, tableName)
}

// AddColumn 新增字段(ALTER TABLE ADD COLUMN)。
func (s *Service) AddColumn(ctx context.Context, tableName string, column ColumnMeta) error {
	if column.Name == "" || column.DataType == "" {
		return errors.New("gencode: column name and type are required")
	}
	ddl := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s",
		quoteIdent(tableName), quoteIdent(column.Name), columnDDLType(column))
	if !column.Nullable {
		ddl += " NOT NULL"
	}
	if column.Default != "" {
		ddl += " DEFAULT " + column.Default
	}
	return s.meta.DB().WithContext(ctx).Exec(ddl).Error
}

// DropColumn 删除字段(驱动支持时)。
func (s *Service) DropColumn(ctx context.Context, tableName, columnName string) error {
	dialect := s.meta.dialect()
	if dialect == "mysql" || dialect == "sqlite" {
		// MySQL 8+/SQLite 3.35+ 支持 DROP COLUMN
	}
	ddl := fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", quoteIdent(tableName), quoteIdent(columnName))
	return s.meta.DB().WithContext(ctx).Exec(ddl).Error
}

// SyncTable 同步表结构:重读元数据并返回(对比记录中字段,标注新增/删除)。
func (s *Service) SyncTable(ctx context.Context, tableName string) (*TableMeta, error) {
	return s.meta.ReadTable(ctx, tableName)
}

// Preview 生成代码预览(不落盘)。
func (s *Service) Preview(ctx context.Context, tableName string) (*GenResult, error) {
	table, err := s.meta.ReadTable(ctx, tableName)
	if err != nil {
		return nil, err
	}
	return Generate(*table, s.modulePath)
}

// Generate 生成代码并写入文件。
// 已生成过的表需要 force=true 才覆盖(返回待覆盖文件清单)。
func (s *Service) Generate(ctx context.Context, tableName string, force bool) (*GenResult, []string, error) {
	table, err := s.meta.ReadTable(ctx, tableName)
	if err != nil {
		return nil, nil, err
	}
	result, err := Generate(*table, s.modulePath)
	if err != nil {
		return nil, nil, err
	}

	// 覆盖检查:已生成过的表,再次生成提示覆盖(列出全部将写文件)
	var overwritten []string
	if _, exists, err := s.findRecord(ctx, tableName); err != nil {
		return nil, nil, err
	} else if exists {
		for _, file := range result.Files {
			overwritten = append(overwritten, file.Path)
		}
		if !force {
			return result, overwritten, nil // 需要确认
		}
	}

	// 写文件
	for _, file := range result.Files {
		path := filepath.Join(s.outputDir, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, nil, err
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
			return nil, nil, err
		}
	}
	// 更新记录
	if err := s.saveRecord(ctx, tableName, result.Files); err != nil {
		return nil, nil, err
	}
	return result, overwritten, nil
}

// ---- 记录管理 ----

// records 读取全部生成记录。
func (s *Service) records(ctx context.Context) (map[string]genRecord, error) {
	var list []genRecord
	if err := s.meta.DB().WithContext(ctx).Find(&list).Error; err != nil {
		return nil, err
	}
	result := make(map[string]genRecord, len(list))
	for _, record := range list {
		result[record.Table] = record
	}
	return result, nil
}

// findRecord 查询单表记录。
func (s *Service) findRecord(ctx context.Context, tableName string) (genRecord, bool, error) {
	var record genRecord
	err := s.meta.DB().WithContext(ctx).First(&record, "table_name = ?", tableName).Error
	if err == nil {
		return record, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return genRecord{}, false, nil
	}
	return genRecord{}, false, err
}

// saveRecord 写入/更新生成记录。
func (s *Service) saveRecord(ctx context.Context, tableName string, files []GeneratedFile) error {
	paths := make([]string, 0, len(files))
	hashes := make(map[string]string, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
		hashes[file.Path] = file.Hash
	}
	pathsJSON, _ := json.Marshal(paths)
	hashesJSON, _ := json.Marshal(hashes)
	record := genRecord{
		Table:       tableName,
		GeneratedAt: time.Now(),
		FilesJSON:   string(pathsJSON),
		HashesJSON:  string(hashesJSON),
	}
	db := s.meta.DB().WithContext(ctx)
	var exist genRecord
	err := db.First(&exist, "table_name = ?", tableName).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(&record).Error
	}
	return db.Model(&genRecord{}).Where("table_name = ?", tableName).
		Updates(map[string]interface{}{
			"generated_at": record.GeneratedAt,
			"files_json":   record.FilesJSON,
			"hashes_json":  record.HashesJSON,
		}).Error
}

// hashes 解析哈希映射。
func (r *genRecord) hashes() map[string]string {
	var result map[string]string
	_ = json.Unmarshal([]byte(r.HashesJSON), &result)
	return result
}

// columnDDLType 字段类型转 DDL(补长度)。
func columnDDLType(column ColumnMeta) string {
	upper := strings.ToUpper(column.DataType)
	if column.Length > 0 && (strings.Contains(upper, "VARCHAR") || strings.Contains(upper, "CHAR")) {
		return fmt.Sprintf("%s(%d)", upper, column.Length)
	}
	return upper
}
