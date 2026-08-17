package excel

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

// TestImportValidated 业务校验:失败行收集错误,成功行保留。
func TestImportValidated(t *testing.T) {
	templatePath := buildTemplate(t)
	f, err := excelize.OpenFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	// 3 行数据:第 2 行数量为 0(校验不通过)
	_ = f.SetCellValue("Sheet1", "A2", "V-001")
	_ = f.SetCellValue("Sheet1", "C2", 10)
	_ = f.SetCellValue("Sheet1", "A3", "V-002")
	_ = f.SetCellValue("Sheet1", "C3", 0)
	_ = f.SetCellValue("Sheet1", "A4", "V-003")
	_ = f.SetCellValue("Sheet1", "C4", 7)
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	rows, rowErrors := ImportValidated(Template{FilePath: templatePath}, templatePath, func(row orderItem, rowNum int) error {
		if row.Quantity <= 0 {
			return errors.New("数量必须大于 0")
		}
		return nil
	})
	if len(rows) != 2 || rows[0].OrderNo != "V-001" || rows[1].OrderNo != "V-003" {
		t.Fatalf("rows = %+v", rows)
	}
	if len(rowErrors) != 1 || rowErrors[0].Row != 3 || rowErrors[0].Error != "数量必须大于 0" {
		t.Fatalf("rowErrors = %+v", rowErrors)
	}
}

// TestImportValidatedNoValidator 无校验函数时全部通过。
func TestImportValidatedNoValidator(t *testing.T) {
	templatePath := buildTemplate(t)
	f, err := excelize.OpenFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.SetCellValue("Sheet1", "A2", "N-001")
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	rows, rowErrors := ImportValidated[orderItem](Template{FilePath: templatePath}, templatePath, nil)
	if len(rows) != 1 || len(rowErrors) != 0 {
		t.Fatalf("rows=%+v errors=%+v", rows, rowErrors)
	}
}

// TestImportStream 流式导入:逐行回调,行号正确。
func TestImportStream(t *testing.T) {
	templatePath := buildTemplate(t)
	outputPath := filepath.Join(t.TempDir(), "stream.xlsx")
	// 先生成 100 行数据
	data := make([]orderItem, 100)
	for i := range data {
		data[i] = orderItem{OrderNo: "S-" + string(rune('A'+i%26)) + string(rune('0'+i/26%10)), Quantity: i}
	}
	if err := ExportToFile(Template{FilePath: templatePath}, data, outputPath); err != nil {
		t.Fatalf("export: %v", err)
	}
	count := 0
	var lastRow int
	err := ImportStream(Template{FilePath: templatePath}, outputPath, func(row orderItem, rowNum int) error {
		count++
		lastRow = rowNum
		if row.Quantity != count-1 {
			t.Errorf("row %d quantity = %d", rowNum, row.Quantity)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if count != 100 || lastRow != 101 {
		t.Fatalf("count=%d lastRow=%d", count, lastRow)
	}
}

// TestImportStreamHandlerError 回调错误中断导入。
func TestImportStreamHandlerError(t *testing.T) {
	templatePath := buildTemplate(t)
	outputPath := filepath.Join(t.TempDir(), "stream2.xlsx")
	data := []orderItem{{OrderNo: "E-001"}, {OrderNo: "E-002"}}
	if err := ExportToFile(Template{FilePath: templatePath}, data, outputPath); err != nil {
		t.Fatal(err)
	}
	count := 0
	err := ImportStream(Template{FilePath: templatePath}, outputPath, func(row orderItem, rowNum int) error {
		count++
		if rowNum == 3 {
			return errors.New("stop here")
		}
		return nil
	})
	if err == nil || count != 2 {
		t.Fatalf("err=%v count=%d", err, count)
	}
}
