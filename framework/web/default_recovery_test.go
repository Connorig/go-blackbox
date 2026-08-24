package webiris

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kataras/iris/v12"
)

// TestDefaultApplicationHasPanicRecovery 验证 newApplication 默认安装
// PanicRecovery:业务 handler panic 返回统一 500 + B0001,无需业务显式 Use。
func TestDefaultApplicationHasPanicRecovery(t *testing.T) {
	app := newApplication("error", func(app *iris.Application) {
		app.Get("/boom", func(ctx iris.Context) {
			panic("default application panic")
		})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	server := httptest.NewServer(app)
	defer server.Close()

	response, err := http.Get(server.URL + "/boom")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("panic must map to 500 by default, got %d", response.StatusCode)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if body["code"] != "B0001" {
		t.Fatalf("default panic response must carry B0001, got %v", body["code"])
	}
}
