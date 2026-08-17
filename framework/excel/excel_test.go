package excel

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

// orderItem 测试用订单行(含基础类型与嵌套)。
type orderItem struct {
	OrderNo   string `excel:"订单号"`
	Customer  string `excel:"客户名称"`
	Quantity  int    `excel:"数量"`
	Price     float64 `excel:"单价"`
	Shipped   bool   `excel:"已发货"`
	Remark    string `excel:"备注"`
}

// buildTemplate 生成测试模板:表头 + 样式(仅表头,无数据)。
func buildTemplate(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "template.xlsx")
	f := excelize.NewFile()
	sheet := "Sheet1"
	headers := []string{"订单号", "客户名称", "数量", "单价", "已发货", "备注"}
	for col, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		if err := f.SetCellValue(sheet, cell, header); err != nil {
			t.Fatalf("set header: %v", err)
		}
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("save template: %v", err)
	}
	return path
}

// TestExportImportRoundTrip 导出→导入往返:字段全保真。
func TestExportImportRoundTrip(t *testing.T) {
	templatePath := buildTemplate(t)
	data := []orderItem{
		{OrderNo: "SO-001", Customer: "吉利汽车", Quantity: 12, Price: 3.5, Shipped: true, Remark: "加急"},
		{OrderNo: "SO-002", Customer: "远景汽配", Quantity: 5, Price: 12.8, Shipped: false, Remark: ""},
	}
	outputPath := filepath.Join(t.TempDir(), "out.xlsx")
	if err := ExportToFile(Template{FilePath: templatePath}, data, outputPath); err != nil {
		t.Fatalf("export: %v", err)
	}
	imported, err := ImportFromFile[orderItem](Template{FilePath: templatePath}, outputPath)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(imported) != 2 {
		t.Fatalf("imported %d items, want 2", len(imported))
	}
	first := imported[0]
	if first.OrderNo != "SO-001" || first.Customer != "吉利汽车" || first.Quantity != 12 || first.Price != 3.5 || !first.Shipped || first.Remark != "加急" {
		t.Fatalf("roundtrip first = %+v", first)
	}
	second := imported[1]
	if second.OrderNo != "SO-002" || second.Shipped || second.Remark != "" {
		t.Fatalf("roundtrip second = %+v", second)
	}
}

// TestExportToWriter 导出到 io.Writer 字节流。
func TestExportToWriter(t *testing.T) {
	templatePath := buildTemplate(t)
	data := []orderItem{{OrderNo: "SO-100", Quantity: 1}}
	var buffer bytes.Buffer
	if err := Export(Template{FilePath: templatePath}, data, &buffer); err != nil {
		t.Fatalf("export to writer: %v", err)
	}
	if buffer.Len() == 0 {
		t.Fatal("writer empty")
	}
	// 字节流可被再次打开
	parsed, err := Import[orderItem](Template{FilePath: templatePath}, bytes.NewReader(buffer.Bytes()))
	if err != nil {
		t.Fatalf("import from bytes: %v", err)
	}
	if len(parsed) != 1 || parsed[0].OrderNo != "SO-100" {
		t.Fatalf("roundtrip bytes = %+v", parsed)
	}
}

// TestImportEmptyData 空数据区导入返回空列表不报错。
func TestImportEmptyData(t *testing.T) {
	templatePath := buildTemplate(t)
	items, err := ImportFromFile[orderItem](Template{FilePath: templatePath}, templatePath)
	if err != nil {
		t.Fatalf("import empty: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("want empty, got %d", len(items))
	}
}

// TestImportUnmatchedColumns 未匹配的列忽略,已匹配列正常解析。
func TestImportUnmatchedColumns(t *testing.T) {
	templatePath := buildTemplate(t)
	// 追加额外列数据(模板外的新列),导入时忽略
	f, err := excelize.OpenFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.SetCellValue("Sheet1", "A2", "SO-999")
	_ = f.SetCellValue("Sheet1", "G2", "额外列")
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	items, err := ImportFromFile[orderItem](Template{FilePath: templatePath}, templatePath)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(items) != 1 || items[0].OrderNo != "SO-999" {
		t.Fatalf("items = %+v", items)
	}
}

// TestExportPreservesTemplateStyle 导出保留模板既有单元格样式(表头样式仍在)。
func TestExportPreservesTemplateStyle(t *testing.T) {
	templatePath := buildTemplate(t)
	// 给表头加样式
	f, err := excelize.OpenFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	styleID, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FF0000"}})
	_ = f.SetCellStyle("Sheet1", "A1", "F1", styleID)
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	data := []orderItem{{OrderNo: "SO-300"}}
	outputPath := filepath.Join(t.TempDir(), "styled.xlsx")
	if err := ExportToFile(Template{FilePath: templatePath}, data, outputPath); err != nil {
		t.Fatalf("export: %v", err)
	}
	opened, err := excelize.OpenFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	style, err := opened.GetCellStyle("Sheet1", "A1")
	if err != nil || style == 0 {
		t.Fatalf("header style lost: style=%d err=%v", style, err)
	}
	_ = os.Remove(outputPath)
}
