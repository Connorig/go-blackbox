package upload

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
)

// TestParseRequest 合法文件解析。
func TestParseRequest(t *testing.T) {
	body, contentType := buildMultipart(t, "file", "test.png", "image content")
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", contentType)
	info, err := ParseRequest(req, "file", Config{
		MaxSize:    1024,
		AllowedExt: []string{".png", ".jpg"},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if info.Name != "test.png" || info.Ext != ".png" || info.Size != 13 {
		t.Fatalf("info = %+v", info)
	}
	if string(info.Content) != "image content" {
		t.Fatalf("content = %q", info.Content)
	}
}

// TestParseRequestSizeLimit 大小超限。
func TestParseRequestSizeLimit(t *testing.T) {
	body, contentType := buildMultipart(t, "file", "big.png", strings.Repeat("x", 100))
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", contentType)
	_, err := ParseRequest(req, "file", Config{MaxSize: 50})
	if err == nil || !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("err = %v", err)
	}
}

// TestParseRequestExtBlocked 扩展名不允许。
func TestParseRequestExtBlocked(t *testing.T) {
	body, contentType := buildMultipart(t, "file", "bad.exe", "binary")
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", contentType)
	_, err := ParseRequest(req, "file", Config{
		MaxSize:    1024,
		AllowedExt: []string{".png"},
	})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("err = %v", err)
	}
}

// TestParseRequestMissingField 字段缺失。
func TestParseRequestMissingField(t *testing.T) {
	body, contentType := buildMultipart(t, "other", "file.txt", "content")
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", contentType)
	_, err := ParseRequest(req, "file", Config{MaxSize: 1024})
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("err = %v", err)
	}
}

// TestParseRequestNilSafe nil 安全。
func TestParseRequestNilSafe(t *testing.T) {
	if _, err := ParseRequest(nil, "file", Config{}); err == nil {
		t.Fatal("nil request must fail")
	}
}

// buildMultipart 构造 multipart form body。
func buildMultipart(t *testing.T, field, filename, content string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="`+field+`"; filename="`+filename+`"`)
	header.Set("Content-Type", "application/octet-stream")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(content))
	_ = writer.Close()
	return &buf, writer.FormDataContentType()
}
