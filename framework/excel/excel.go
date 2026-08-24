// Package excel 提供基于模板的 Excel 导入导出:泛型列表导出/导入,
// 依赖用户预调好的 .xlsx 模板文件(样式/格式/公式),数据区域按行填充。
// 映射规则:struct tag `excel:"列名"` 匹配模板表头;无 tag 时用字段名匹配。
package excel

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Template 模板配置:用户提供的 .xlsx 文件(已调好样式/格式),gbx 填充数据区域。
type Template struct {
	// FilePath 模板文件路径(导出时打开;导入时作为表头参考)。
	FilePath string
	// SheetName 工作表名(默认 "Sheet1")。
	SheetName string
	// HeaderRow 表头行号(1-based,默认 1)。
	HeaderRow int
	// DataStartRow 数据起始行号(1-based,默认 2)。
	DataStartRow int
}

// normalize 补齐默认值。
func (t Template) normalize() Template {
	if t.SheetName == "" {
		t.SheetName = "Sheet1"
	}
	if t.HeaderRow < 1 {
		t.HeaderRow = 1
	}
	if t.DataStartRow < 1 {
		t.DataStartRow = 2
	}
	return t
}

// Export 将泛型数据列表按模板填充到 writer(返回 .xlsx 字节流)。
// 模板的数据区域从 DataStartRow 开始,每行对应 data 的一项,列序按模板表头匹配。
func Export[T any](tpl Template, data []T, writer io.Writer) error {
	tpl = tpl.normalize()
	f, err := excelize.OpenFile(tpl.FilePath)
	if err != nil {
		return fmt.Errorf("excel export: open template %q: %w", tpl.FilePath, err)
	}
	defer f.Close()
	headerMap, err := buildHeaderMap(f, tpl, typeOfT[T]())
	if err != nil {
		return err
	}
	for i, item := range data {
		row := tpl.DataStartRow + i
		val := reflect.ValueOf(item)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		if val.Kind() != reflect.Struct {
			return fmt.Errorf("excel export: element %d is not a struct", i)
		}
		for col, field := range headerMap {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			_ = f.SetCellValue(tpl.SheetName, cell, fieldValue(val, field))
		}
	}
	return f.Write(writer)
}

// ExportToFile 将泛型数据列表按模板填充到输出文件。
func ExportToFile[T any](tpl Template, data []T, outputPath string) error {
	tpl = tpl.normalize()
	f, err := excelize.OpenFile(tpl.FilePath)
	if err != nil {
		return fmt.Errorf("excel export: open template %q: %w", tpl.FilePath, err)
	}
	defer f.Close()
	headerMap, err := buildHeaderMap(f, tpl, typeOfT[T]())
	if err != nil {
		return err
	}
	for i, item := range data {
		row := tpl.DataStartRow + i
		val := reflect.ValueOf(item)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		if val.Kind() != reflect.Struct {
			return fmt.Errorf("excel export: element %d is not a struct", i)
		}
		for col, field := range headerMap {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			_ = f.SetCellValue(tpl.SheetName, cell, fieldValue(val, field))
		}
	}
	return f.SaveAs(outputPath)
}

// Import 从 reader 读取 Excel,按模板表头映射解析为泛型列表。
func Import[T any](tpl Template, reader io.Reader) ([]T, error) {
	tpl = tpl.normalize()
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, fmt.Errorf("excel import: open reader: %w", err)
	}
	defer f.Close()
	return importFromFile[T](f, tpl)
}

// ImportFromFile 从 Excel 文件按模板表头映射解析为泛型列表。
func ImportFromFile[T any](tpl Template, inputPath string) ([]T, error) {
	tpl = tpl.normalize()
	f, err := excelize.OpenFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("excel import: open %q: %w", inputPath, err)
	}
	defer f.Close()
	return importFromFile[T](f, tpl)
}

// typeOfT 返回泛型 T 的反射类型。
func typeOfT[T any]() reflect.Type {
	typ := reflect.TypeOf((*T)(nil)).Elem()
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	return typ
}

// importFromFile 内部导入实现。
func importFromFile[T any](f *excelize.File, tpl Template) ([]T, error) {
	tpl = tpl.normalize()
	headerMap, err := buildHeaderMap(f, tpl, typeOfT[T]())
	if err != nil {
		return nil, err
	}
	rows, err := f.GetRows(tpl.SheetName)
	if err != nil {
		return nil, fmt.Errorf("excel import: get rows %q: %w", tpl.SheetName, err)
	}
	if len(rows) < tpl.DataStartRow {
		return []T{}, nil
	}
	var result []T
	for i := tpl.DataStartRow - 1; i < len(rows); i++ {
		row := rows[i]
		var item T
		val := reflect.ValueOf(&item).Elem()
		if val.Kind() != reflect.Struct {
			return nil, fmt.Errorf("excel import: type %T is not a struct", item)
		}
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
		result = append(result, item)
	}
	return result, nil
}

// excelField 列映射信息。
type excelField struct {
	index int    // 结构体字段索引
	name  string // 列名(tag 或字段名)
}

// buildHeaderMap 读取模板表头行,建立列序 → 结构体字段的映射。
func buildHeaderMap(f *excelize.File, tpl Template, typ reflect.Type) ([]excelField, error) {
	rows, err := f.GetRows(tpl.SheetName)
	if err != nil {
		return nil, fmt.Errorf("excel: read sheet %q: %w", tpl.SheetName, err)
	}
	if tpl.HeaderRow-1 >= len(rows) {
		return nil, fmt.Errorf("excel: header row %d out of range (total %d rows)", tpl.HeaderRow, len(rows))
	}
	header := rows[tpl.HeaderRow-1]
	if len(header) == 0 {
		return nil, fmt.Errorf("excel: empty header row %d", tpl.HeaderRow)
	}
	fieldByName := make(map[string]int)
	fieldByTag := make(map[string]int)
	for i := 0; i < typ.NumField(); i++ {
		fld := typ.Field(i)
		if !fld.IsExported() {
			continue
		}
		fieldByName[fld.Name] = i
		if tag := strings.TrimSpace(fld.Tag.Get("excel")); tag != "" {
			fieldByTag[tag] = i
		}
	}
	result := make([]excelField, len(header))
	for col, colName := range header {
		colName = strings.TrimSpace(colName)
		if idx, ok := fieldByTag[colName]; ok {
			result[col] = excelField{index: idx, name: colName}
		} else if idx, ok := fieldByName[colName]; ok {
			result[col] = excelField{index: idx, name: colName}
		}
		// 未匹配的列保留零值(index=0),导入时跳过,导出时留空
	}
	return result, nil
}

// fieldValue 反射读取字段值。
func fieldValue(val reflect.Value, field excelField) interface{} {
	if field.index >= val.NumField() {
		return ""
	}
	return val.Field(field.index).Interface()
}

// setFieldValue 反射设置字段值(字符串到类型转换)。
func setFieldValue(val reflect.Value, field excelField, value string) {
	if field.index >= val.NumField() {
		return
	}
	f := val.Field(field.index)
	if !f.CanSet() {
		return
	}
	switch f.Kind() {
	case reflect.String:
		f.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parseIntField(f, value)
	case reflect.Float32, reflect.Float64:
		parseFloatField(f, value)
	case reflect.Bool:
		f.SetBool(strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "yes"))
	default:
	}
}
