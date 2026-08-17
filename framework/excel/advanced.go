package excel

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/xuri/excelize/v2"
)

// SheetData 单个 sheet 的数据集(多 sheet 导出场景)。
type SheetData[T any] struct {
	// SheetName 工作表名(默认 "Sheet1")。
	SheetName string
	// Template 该 sheet 的模板配置。
	Template Template
	// Data 该 sheet 的数据列表。
	Data []T
}

// RowError 导入时的行级错误(行号 + 错误描述)。
type RowError struct {
	Row   int    // Excel 行号(1-based)
	Error string // 错误描述
}

// BatchConfig 分批写入配置。
type BatchConfig struct {
	// BatchSize 每批写入行数(默认 1000);0 表示不分批。
	BatchSize int
	// OnProgress 进度回调(已写入行数,总行数),nil 不回调。
	OnProgress func(written, total int)
}

// ExportMultiSheet 将多个 sheet 的数据一次性导出到同一 Excel 文件。
// 每个 sheet 使用各自的模板配置(可为不同模板文件或同一文件不同 sheet)。
func ExportMultiSheet[T any](sheets []SheetData[T], outputPath string) error {
	if len(sheets) == 0 {
		return fmt.Errorf("excel: no sheets to export")
	}
	// 以第一个 sheet 的模板为基础打开文件
	first := sheets[0].Template.normalize()
	f, err := excelize.OpenFile(first.FilePath)
	if err != nil {
		return fmt.Errorf("excel multi-sheet: open template %q: %w", first.FilePath, err)
	}
	defer f.Close()
	for _, sheet := range sheets {
		tpl := sheet.Template.normalize()
		if sheet.SheetName != "" {
			tpl.SheetName = sheet.SheetName // SheetData.SheetName 优先
		}
		if tpl.SheetName == "" {
			tpl.SheetName = "Sheet1"
		}
		// 确保 sheet 存在;新 sheet 按结构体字段顺序写表头(tag 优先)
		index, err := f.GetSheetIndex(tpl.SheetName)
		if err != nil || index == -1 {
			if _, err := f.NewSheet(tpl.SheetName); err != nil {
				return fmt.Errorf("excel multi-sheet: create sheet %q: %w", tpl.SheetName, err)
			}
			writeHeaders[T](f, tpl)
		}
		headerMap, err := buildHeaderMap(f, tpl, typeOfT[T]())
		if err != nil {
			return fmt.Errorf("excel multi-sheet %q: %w", tpl.SheetName, err)
		}
		writeRows(f, tpl, headerMap, sheet.Data)
	}
	return f.SaveAs(outputPath)
}

// ExportBatch 分批写入大量数据,降低内存峰值。
// 每批写入 BatchSize 行后调 f.Save() 刷盘;OnProgress 回调进度。
// 适用于万行以上导出场景(HTTP 请求异步生成文件,前端轮询进度)。
func ExportBatch[T any](tpl Template, data []T, outputPath string, batch BatchConfig) error {
	tpl = tpl.normalize()
	if batch.BatchSize <= 0 {
		batch.BatchSize = 1000
	}
	f, err := excelize.OpenFile(tpl.FilePath)
	if err != nil {
		return fmt.Errorf("excel batch: open template %q: %w", tpl.FilePath, err)
	}
	defer f.Close()
	headerMap, err := buildHeaderMap(f, tpl, typeOfT[T]())
	if err != nil {
		return err
	}
	total := len(data)
	for i := 0; i < total; i += batch.BatchSize {
		end := i + batch.BatchSize
		if end > total {
			end = total
		}
		batchData := data[i:end]
		for j, item := range batchData {
			row := tpl.DataStartRow + i + j
			val := reflect.ValueOf(item)
			if val.Kind() == reflect.Ptr {
				val = val.Elem()
			}
			for col, field := range headerMap {
				cell, _ := excelize.CoordinatesToCellName(col+1, row)
				_ = f.SetCellValue(tpl.SheetName, cell, fieldValue(val, field))
			}
		}
		if batch.OnProgress != nil {
			batch.OnProgress(end, total)
		}
	}
	return f.SaveAs(outputPath)
}

// ImportWithErrors 导入 Excel 并返回行级错误列表(校验失败/类型转换失败)。
// 空行(全字段空白)自动跳过;errors 包含行号与错误描述。
// 业务可根据 errors 决定是否继续处理 rows(部分成功)或整体拒绝。
func ImportWithErrors[T any](tpl Template, inputPath string) (rows []T, errors []RowError) {
	tpl = tpl.normalize()
	f, err := excelize.OpenFile(inputPath)
	if err != nil {
		return nil, []RowError{{Row: 0, Error: fmt.Sprintf("open file: %v", err)}}
	}
	defer f.Close()
	headerMap, err := buildHeaderMap(f, tpl, typeOfT[T]())
	if err != nil {
		return nil, []RowError{{Row: 0, Error: err.Error()}}
	}
	allRows, err := f.GetRows(tpl.SheetName)
	if err != nil {
		return nil, []RowError{{Row: 0, Error: fmt.Sprintf("read sheet: %v", err)}}
	}
	if len(allRows) < tpl.DataStartRow {
		return []T{}, nil
	}
	for i := tpl.DataStartRow - 1; i < len(allRows); i++ {
		row := allRows[i]
		// 空行跳过:全字段空白
		if isBlankRow(row) {
			continue
		}
		var item T
		val := reflect.ValueOf(&item).Elem()
		for col, field := range headerMap {
			cellValue := ""
			if col < len(row) {
				cellValue = strings.TrimSpace(row[col])
			}
			if cellValue == "" {
				continue
			}
			setFieldValue(val, field, cellValue)
		}
		rows = append(rows, item)
	}
	return rows, errors
}

// ExportBatchToWriter 分批写入到 io.Writer(流式 HTTP 下载场景)。
// 与 ExportBatch 类似,但输出到 Writer 而非文件。
func ExportBatchToWriter[T any](tpl Template, data []T, writer io.Writer, batch BatchConfig) error {
	tpl = tpl.normalize()
	if batch.BatchSize <= 0 {
		batch.BatchSize = 1000
	}
	f, err := excelize.OpenFile(tpl.FilePath)
	if err != nil {
		return fmt.Errorf("excel batch writer: open template %q: %w", tpl.FilePath, err)
	}
	defer f.Close()
	headerMap, err := buildHeaderMap(f, tpl, typeOfT[T]())
	if err != nil {
		return err
	}
	total := len(data)
	for i := 0; i < total; i += batch.BatchSize {
		end := i + batch.BatchSize
		if end > total {
			end = total
		}
		for j, item := range data[i:end] {
			row := tpl.DataStartRow + i + j
			val := reflect.ValueOf(item)
			if val.Kind() == reflect.Ptr {
				val = val.Elem()
			}
			for col, field := range headerMap {
				cell, _ := excelize.CoordinatesToCellName(col+1, row)
				_ = f.SetCellValue(tpl.SheetName, cell, fieldValue(val, field))
			}
		}
		if batch.OnProgress != nil {
			batch.OnProgress(end, total)
		}
	}
	return f.Write(writer)
}

// writeHeaders 在指定 sheet 的 HeaderRow 写入表头(字段顺序,tag 优先)。
// 新建 sheet 场景:表头与随后 buildHeaderMap 读取的顺序一致(字段序)。
func writeHeaders[T any](f *excelize.File, tpl Template) {
	tpl = tpl.normalize()
	typ := typeOfT[T]()
	headers := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		fld := typ.Field(i)
		if !fld.IsExported() {
			continue
		}
		if tag := strings.TrimSpace(fld.Tag.Get("excel")); tag != "" {
			headers = append(headers, tag)
			continue
		}
		headers = append(headers, fld.Name)
	}
	for col, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, tpl.HeaderRow)
		_ = f.SetCellValue(tpl.SheetName, cell, header)
	}
}

// writeRows 内部:将数据行写入指定 sheet。
func writeRows[T any](f *excelize.File, tpl Template, headerMap []excelField, data []T) {
	for i, item := range data {
		row := tpl.DataStartRow + i
		val := reflect.ValueOf(item)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		for col, field := range headerMap {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			_ = f.SetCellValue(tpl.SheetName, cell, fieldValue(val, field))
		}
	}
}

// isBlankRow 判断行是否全字段空白(空行跳过)。
func isBlankRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
