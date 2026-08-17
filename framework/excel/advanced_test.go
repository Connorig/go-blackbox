package excel

import (
	"path/filepath"
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

// TestExportMultiSheet 多 sheet 导出:模板已有 sheet + 新建 sheet。
func TestExportMultiSheet(t *testing.T) {
	templatePath := buildTemplate(t)
	sheets := []SheetData[orderItem]{
		{SheetName: "Sheet1", Template: Template{FilePath: templatePath}, Data: []orderItem{{OrderNo: "S1-001", Quantity: 1}}},
		{SheetName: "明细", Template: Template{FilePath: templatePath}, Data: []orderItem{{OrderNo: "S2-001", Quantity: 2}, {OrderNo: "S2-002", Quantity: 3}}},
	}
	outputPath := filepath.Join(t.TempDir(), "multi.xlsx")
	if err := ExportMultiSheet(sheets, outputPath); err != nil {
		t.Fatalf("multi-sheet export: %v", err)
	}
	f, err := excelize.OpenFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	// 新建 sheet 有表头 + 数据
	value, _ := f.GetCellValue("明细", "A1")
	if value != "订单号" {
		t.Fatalf("new sheet header = %q", value)
	}
	value, _ = f.GetCellValue("明细", "A3")
	if value != "S2-002" {
		t.Fatalf("new sheet data = %q", value)
	}
}

// TestExportBatchProgress 分批写入 + 进度回调。
func TestExportBatchProgress(t *testing.T) {
	templatePath := buildTemplate(t)
	data := make([]orderItem, 105)
	for i := range data {
		data[i] = orderItem{OrderNo: "B-" + string(rune('A'+i%26)), Quantity: i}
	}
	var progress []int
	outputPath := filepath.Join(t.TempDir(), "batch.xlsx")
	err := ExportBatch(Template{FilePath: templatePath}, data, outputPath, BatchConfig{
		BatchSize: 25,
		OnProgress: func(written, total int) {
			progress = append(progress, written)
		},
	})
	if err != nil {
		t.Fatalf("batch export: %v", err)
	}
	// 105 行 / 25 批 = 5 批,进度 [25 50 75 100 105]
	if len(progress) != 5 || progress[4] != 105 {
		t.Fatalf("progress = %v", progress)
	}
	// 导入验证数据完整性
	imported, err := ImportFromFile[orderItem](Template{FilePath: templatePath}, outputPath)
	if err != nil {
		t.Fatalf("import batch: %v", err)
	}
	if len(imported) != 105 || imported[104].Quantity != 104 {
		t.Fatalf("batch import mismatch: %d rows, last=%+v", len(imported), imported[len(imported)-1])
	}
}

// TestImportWithErrorsSkipBlank 导入跳过空行,返回行级错误。
func TestImportWithErrorsSkipBlank(t *testing.T) {
	templatePath := buildTemplate(t)
	// 在模板数据区构造:有效行 + 空行 + 有效行
	f, err := excelize.OpenFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.SetCellValue("Sheet1", "A2", "V-001")
	_ = f.SetCellValue("Sheet1", "B2", "客户甲")
	// 第 3 行留空
	_ = f.SetCellValue("Sheet1", "A4", "V-002")
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	rows, rowErrors := ImportWithErrors[orderItem](Template{FilePath: templatePath}, templatePath)
	if len(rowErrors) != 0 {
		t.Fatalf("unexpected errors: %v", rowErrors)
	}
	if len(rows) != 2 || rows[0].OrderNo != "V-001" || rows[1].OrderNo != "V-002" {
		t.Fatalf("rows = %+v", rows)
	}
}

// TestImportWithErrorsHeaderRowOffset 表头空几行场景:HeaderRow 与 DataStartRow 独立配置。
func TestImportWithErrorsHeaderRowOffset(t *testing.T) {
	templatePath := buildTemplate(t)
	// 构造:第 1 行空,第 2 行表头,第 3 行起数据
	f, err := excelize.OpenFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	headers := []string{"订单号", "客户名称", "数量", "单价", "已发货", "备注"}
	for col, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 2)
		_ = f.SetCellValue("Sheet1", cell, header)
	}
	_ = f.SetCellValue("Sheet1", "A3", "OFF-001")
	_ = f.SetCellValue("Sheet1", "A4", "OFF-002")
	_ = f.SetCellValue("Sheet1", "A5", "OFF-003")
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	tpl := Template{FilePath: templatePath, HeaderRow: 2, DataStartRow: 3}
	rows, rowErrors := ImportWithErrors[orderItem](tpl, templatePath)
	if len(rowErrors) != 0 {
		t.Fatalf("errors: %v", rowErrors)
	}
	if len(rows) != 3 || rows[0].OrderNo != "OFF-001" || rows[2].OrderNo != "OFF-003" {
		t.Fatalf("rows = %+v", rows)
	}
}

// TestExportBatchToWriter 分批导出到 Writer(HTTP 流式下载)。
func TestExportBatchToWriter(t *testing.T) {
	templatePath := buildTemplate(t)
	data := []orderItem{{OrderNo: "W-001", Quantity: 7}}
	var buffer bytes.Buffer
	err := ExportBatchToWriter(Template{FilePath: templatePath}, data, &buffer, BatchConfig{BatchSize: 10})
	if err != nil {
		t.Fatalf("batch writer: %v", err)
	}
	if buffer.Len() == 0 {
		t.Fatal("writer empty")
	}
}
