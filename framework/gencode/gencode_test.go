package gencode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// testDB 创建临时 SQLite 并建测试表。
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "gen.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.Exec(`CREATE TABLE test_mycat (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		status INTEGER DEFAULT 1,
		remark TEXT,
		gmt_create DATETIME,
		gmt_modified DATETIME,
		is_deleted INTEGER DEFAULT 0
	)`).Error; err != nil {
		t.Fatalf("create table failed: %v", err)
	}
	if err := db.Exec(`CREATE TABLE test_order (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_no VARCHAR(64) NOT NULL,
		amount REAL,
		gmt_create DATETIME,
		gmt_modified DATETIME,
		is_deleted INTEGER DEFAULT 0
	)`).Error; err != nil {
		t.Fatalf("create table failed: %v", err)
	}
	return db
}

// TestMetaListTables 元数据读取(SQLite 全链路)。
func TestMetaListTables(t *testing.T) {
	db := testDB(t)
	meta := NewMeta(db)
	tables, err := meta.ListTables(context.Background())
	if err != nil {
		t.Fatalf("list tables failed: %v", err)
	}
	if len(tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(tables))
	}
	// 验证字段
	var mycat *TableMeta
	for i := range tables {
		if tables[i].Name == "test_mycat" {
			mycat = &tables[i]
		}
	}
	if mycat == nil {
		t.Fatal("test_mycat not found")
	}
	names := map[string]bool{}
	for _, column := range mycat.Columns {
		names[column.Name] = true
	}
	for _, want := range []string{"id", "name", "status", "remark"} {
		if !names[want] {
			t.Fatalf("column %q missing: %v", want, names)
		}
	}
	// 主键标记
	for _, column := range mycat.Columns {
		if column.Name == "id" && !column.PrimaryKey {
			t.Fatal("id must be primary key")
		}
	}
}

// TestGenerate 生成 DDD 代码五件套。
func TestGenerate(t *testing.T) {
	db := testDB(t)
	meta := NewMeta(db)
	table, err := meta.ReadTable(context.Background(), "test_order")
	if err != nil {
		t.Fatalf("read table failed: %v", err)
	}
	result, err := Generate(*table, "github.com/example/app")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if len(result.Files) != 5 {
		t.Fatalf("expected 5 files, got %d", len(result.Files))
	}
	paths := map[string]bool{}
	for _, file := range result.Files {
		paths[file.Path] = true
		if file.Content == "" {
			t.Fatalf("empty content: %s", file.Path)
		}
	}
	for _, want := range []string{
		"internal/model/test_order.go",
		"internal/filter/test_order.go",
		"internal/repository/test_order.go",
		"internal/service/test_order.go",
		"internal/handler/test_order.go",
	} {
		if !paths[want] {
			t.Fatalf("missing generated file: %s", want)
		}
	}
	// 模型内容:类型映射 + 基础字段(按路径定位,map 迭代无序)
	var modelFile GeneratedFile
	for _, file := range result.Files {
		if file.Path == "internal/model/test_order.go" {
			modelFile = file
		}
	}
	if modelFile.Content == "" {
		t.Fatal("model file not found")
	}
	if !strings.Contains(modelFile.Content, "TestOrder") {
		t.Fatal("model name wrong")
	}
	if !strings.Contains(modelFile.Content, "model.StandardModel") {
		t.Fatal("must embed StandardModel")
	}
	if !strings.Contains(modelFile.Content, "OrderNo string") {
		t.Fatalf("order_no must map to string:\n%s", modelFile.Content)
	}
	if !strings.Contains(modelFile.Content, "Amount float64") {
		t.Fatalf("amount must map to float64:\n%s", modelFile.Content)
	}
	// 路由片段
	if !strings.Contains(result.RouteCode, "/api/v1/test-order") {
		t.Fatal("route code wrong")
	}
}

// TestServiceGenerateAndOverwrite 生成 + 覆盖保护。
func TestServiceGenerateAndOverwrite(t *testing.T) {
	db := testDB(t)
	outputDir := t.TempDir()
	service, err := NewService(db, "github.com/example/app", outputDir)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}
	ctx := context.Background()

	// 首次生成:无覆盖
	result, overwritten, err := service.Generate(ctx, "test_mycat", false)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if len(overwritten) != 0 {
		t.Fatalf("first generate must not overwrite: %v", overwritten)
	}
	// 文件已落盘
	modelPath := filepath.Join(outputDir, "internal", "model", "test_mycat.go")
	if _, err := os.Stat(modelPath); err != nil {
		t.Fatalf("generated file missing: %v", err)
	}

	// 再次生成:需确认(不落盘)
	_, overwritten, err = service.Generate(ctx, "test_mycat", false)
	if err != nil {
		t.Fatalf("second generate failed: %v", err)
	}
	if len(overwritten) == 0 {
		t.Fatal("second generate must report overwritten files")
	}

	// force 覆盖成功
	result, overwritten, err = service.Generate(ctx, "test_mycat", true)
	if err != nil {
		t.Fatalf("force generate failed: %v", err)
	}
	if len(overwritten) == 0 {
		t.Fatal("force generate must overwrite changed files")
	}
	_ = result

	// 表列表标记
	tables, err := service.ListTables(ctx)
	if err != nil {
		t.Fatalf("list tables failed: %v", err)
	}
	found := false
	for _, table := range tables {
		if table.Name == "test_mycat" && table.Generated {
			found = true
		}
	}
	if !found {
		t.Fatal("generated flag must be set")
	}
}

// TestAddDropColumn 字段增删(DDL)。
func TestAddDropColumn(t *testing.T) {
	db := testDB(t)
	service, err := NewService(db, "github.com/example/app", t.TempDir())
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}
	ctx := context.Background()

	if err := service.AddColumn(ctx, "test_mycat", ColumnMeta{
		Name: "age", DataType: "INTEGER", Length: 0, Nullable: true, Comment: "年龄",
	}); err != nil {
		t.Fatalf("add column failed: %v", err)
	}
	table, err := service.ReadTable(ctx, "test_mycat")
	if err != nil {
		t.Fatalf("read table failed: %v", err)
	}
	found := false
	for _, column := range table.Columns {
		if column.Name == "age" {
			found = true
		}
	}
	if !found {
		t.Fatal("added column not visible")
	}
	if err := service.DropColumn(ctx, "test_mycat", "age"); err != nil {
		t.Fatalf("drop column failed: %v", err)
	}
	table, _ = service.ReadTable(ctx, "test_mycat")
	for _, column := range table.Columns {
		if column.Name == "age" {
			t.Fatal("dropped column still visible")
		}
	}
}
