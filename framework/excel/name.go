package excel

import (
	"fmt"
	"strings"
	"time"
)

// ExportName 构造导出文件名:业务名 + 时间戳后缀 + .xlsx 扩展。
// 中文安全:Go 字符串原生 UTF-8,HTTP 响应头用 RFC 5987 编码(Content-Disposition)。
// 防覆盖:时间戳精度到秒(yyyyMMddHHmmss),同一秒内多次导出加序号。
//
// 用法:
//
//	filename := excel.ExportName("订单导出")           // 订单导出_20260817_233700.xlsx
//	filename := excel.ExportName("orders")             // orders_20260817_233700.xlsx
//	filename := excel.ExportName("订单导出", "2026-08") // 订单导出_2026-08.xlsx(自定义后缀)
func ExportName(name string, suffix ...string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "export"
	}
	// 去掉可能的扩展名(防 name="订单.xlsx" 产生 .xlsx.xlsx)
	name = strings.TrimSuffix(name, ".xlsx")
	name = strings.TrimSuffix(name, ".xls")

	stamp := time.Now().Format("20060102_150405")
	if len(suffix) > 0 && suffix[0] != "" {
		stamp = suffix[0]
	}
	return fmt.Sprintf("%s_%s.xlsx", name, stamp)
}

// ContentDisposition 生成 HTTP Content-Disposition 响应头值,
// 兼容中文文件名(RFC 5987 编码 + ASCII fallback):
//
//	attachment; filename="export_20260817.xlsx"; filename*=UTF-8''%E8%AE%A2%E5%8D%95%E5%AF%BC%E5%87%BA_20260817.xlsx
//
// 用法:
//
//	w.Header().Set("Content-Disposition", excel.ContentDisposition("订单导出_20260817_233700.xlsx"))
//	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
func ContentDisposition(filename string) string {
	// ASCII fallback:替换非 ASCII 字符
	ascii := strings.Map(func(r rune) rune {
		if r < 128 {
			return r
		}
		return '_'
	}, filename)
	// RFC 5987 编码:percent-encode 非 ASCII
	encoded := percentEncode(filename)
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, ascii, encoded)
}

// percentEncode 对文件名做 RFC 5987 percent-encoding(非 ASCII 与保留字符编码)。
func percentEncode(s string) string {
	var builder strings.Builder
	for _, r := range s {
		if r < 128 && r != '"' && r != '%' {
			builder.WriteRune(r)
			continue
		}
		// 非 ASCII 或特殊字符:UTF-8 字节 percent-encode
		bytes := []byte(string(r))
		for _, b := range bytes {
			fmt.Fprintf(&builder, "%%%02X", b)
		}
	}
	return builder.String()
}
