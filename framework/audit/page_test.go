package oplog

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kataras/iris/v12"
)

// TestRegisterPage 验证审计页面可访问且注入 prefix。
func TestRegisterPage(t *testing.T) {
	app := iris.New()
	Register(app, "/ops/audit", nil, "any-key", Config{})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	server := httptest.NewServer(app)
	defer server.Close()

	// 无斜杠访问
	response, err := http.Get(server.URL + "/ops/audit")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	content := readAll(response)
	if response.StatusCode != 200 {
		t.Fatalf("page must be 200, got %d", response.StatusCode)
	}
	if !strings.Contains(content, "操作审计查询") {
		t.Fatal("page must contain title")
	}
	if !strings.Contains(content, `var prefix = "/ops/audit"`) {
		t.Fatalf("page must inject prefix into script, got: %.200s", content)
	}
	if strings.Contains(content, "{{PREFIX}}") {
		t.Fatal("placeholder must be replaced")
	}
}

// TestRegisterPageSlash 验证带斜杠访问同样可用。
func TestRegisterPageSlash(t *testing.T) {
	app := iris.New()
	Register(app, "/ops/audit", nil, "any-key", Config{})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	server := httptest.NewServer(app)
	defer server.Close()

	response, err := http.Get(server.URL + "/ops/audit/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	content := readAll(response)
	if response.StatusCode != 200 || !strings.Contains(content, "操作审计查询") {
		t.Fatalf("slash page failed: %d", response.StatusCode)
	}
}

// TestRegisterNilApp 验证 nil app 防御。
func TestRegisterNilApp(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("nil app must panic")
		}
	}()
	Register(nil, "/ops/audit", nil, "key", Config{})
}

// readAll 读取响应体。
func readAll(response *http.Response) string {
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	return string(data)
}
