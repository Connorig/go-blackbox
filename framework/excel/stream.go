package excel

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ValidateFunc 业务行校验:返回 error 表示该行不合法。
// 典型:必填项检查、枚举值、跨字段联动校验。
type ValidateFunc[T any] func(row T, rowNum int) error

// ImportValidated 带业务校验的导入:
// 校验失败的行进入 errors(含行号与原因),合法行进 rows(部分成功语义)。
// 空行自动跳过;文件/结构错误以 Row 0 错误返回。
func ImportValidated[T any](tpl Template, inputPath string, validate ValidateFunc[T]) (rows []T, errors []RowError) {
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
		rowNum := i + 1
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
		if validate != nil {
			if validateErr := validate(item, rowNum); validateErr != nil {
				errors = append(errors, RowError{Row: rowNum, Error: validateErr.Error()})
				continue
			}
		}
		rows = append(rows, item)
	}
	return rows, errors
}

// ImportStream 流式导入(超大文件:几十万行):
// 逐行读取并回调,不一次性加载全部数据到内存。
// handler 返回 error 时中断导入并返回该错误(含行号上下文)。
func ImportStream[T any](tpl Template, inputPath string, handler func(row T, rowNum int) error) error {
	if handler == nil {
		return fmt.Errorf("excel stream: handler is nil")
	}
	tpl = tpl.normalize()
	f, err := excelize.OpenFile(inputPath)
	if err != nil {
		return fmt.Errorf("excel stream: open %q: %w", inputPath, err)
	}
	defer f.Close()
	headerMap, err := buildHeaderMap(f, tpl, typeOfT[T]())
	if err != nil {
		return err
	}
	rows, err := f.Rows(tpl.SheetName)
	if err != nil {
		return fmt.Errorf("excel stream: rows %q: %w", tpl.SheetName, err)
	}
	defer rows.Close()
	rowNum := 0
	for rows.Next() {
		rowNum++
		if rowNum < tpl.DataStartRow {
			continue
		}
		row, err := rows.Columns()
		if err != nil {
			return fmt.Errorf("excel stream: read row %d: %w", rowNum, err)
		}
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
		if err := handler(item, rowNum); err != nil {
			return fmt.Errorf("excel stream: row %d: %w", rowNum, err)
		}
	}
	return rows.Error()
}
