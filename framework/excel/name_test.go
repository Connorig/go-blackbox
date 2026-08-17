package excel

import (
	"strings"
	"testing"
	"time"
)

// TestExportNameBasic 中文名称 + 时间戳后缀 + 扩展名处理。
func TestExportNameBasic(t *testing.T) {
	name := ExportName("订单导出")
	if !strings.HasPrefix(name, "订单导出_") || !strings.HasSuffix(name, ".xlsx") {
		t.Fatalf("name = %q", name)
	}
	// 时间戳格式 yyyyMMdd_HHmmss
	stamp := strings.TrimSuffix(strings.TrimPrefix(name, "订单导出_"), ".xlsx")
	if _, err := time.Parse("20060102_150405", stamp); err != nil {
		t.Fatalf("timestamp %q invalid: %v", stamp, err)
	}
}

// TestExportNameCustomSuffix 自定义后缀。
func TestExportNameCustomSuffix(t *testing.T) {
	name := ExportName("orders", "2026-08")
	if name != "orders_2026-08.xlsx" {
		t.Fatalf("name = %q", name)
	}
}

// TestExportNameTrimsExtension 输入带扩展名不重复。
func TestExportNameTrimsExtension(t *testing.T) {
	name := ExportName("订单.xlsx")
	if strings.Contains(name, ".xlsx.xlsx") || strings.Contains(name, ".xls_") {
		t.Fatalf("name = %q", name)
	}
	if !strings.HasSuffix(name, ".xlsx") {
		t.Fatalf("name = %q", name)
	}
}

// TestExportNameEmpty 空名回退默认。
func TestExportNameEmpty(t *testing.T) {
	name := ExportName("")
	if !strings.HasPrefix(name, "export_") {
		t.Fatalf("name = %q", name)
	}
}

// TestContentDisposition 中文文件名 RFC 5987 编码。
func TestContentDisposition(t *testing.T) {
	header := ContentDisposition("订单导出_20260817.xlsx")
	if !strings.Contains(header, `filename*="UTF-8''`) && !strings.Contains(header, "filename*=UTF-8''") {
		t.Fatalf("header missing RFC 5987: %q", header)
	}
	if !strings.Contains(header, "attachment;") {
		t.Fatalf("header missing attachment: %q", header)
	}
	// 中文被 percent-encode(无原始中文字节)
	if strings.Contains(header, "订单") {
		t.Fatalf("header must encode non-ASCII: %q", header)
	}
	// ASCII fallback 存在
	if !strings.Contains(header, `filename="`) {
		t.Fatalf("header missing ascii fallback: %q", header)
	}
}

// TestPercentEncode 编码正确性。
func TestPercentEncode(t *testing.T) {
	if percentEncode("abc") != "abc" {
		t.Fatalf("ascii unchanged: %q", percentEncode("abc"))
	}
	encoded := percentEncode("订单")
	if encoded == "订单" {
		t.Fatalf("non-ascii must be encoded")
	}
	// UTF-8 字节:订 = E8 AE A2,单 = E5 8D 95
	if encoded != "%E8%AE%A2%E5%8D%95" {
		t.Fatalf("encoded = %q", encoded)
	}
}
