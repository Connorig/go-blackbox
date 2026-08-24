package webiris

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Connorig/go-blackbox/component/i18n"
	"github.com/kataras/iris/v12"
)

// TestLanguageMiddleware 验证 Accept-Language 检测写入上下文。
func TestLanguageMiddleware(t *testing.T) {
	bundle := i18n.NewBundle()
	bundle.Register("zh-CN", map[string]string{"greet": "你好"})
	bundle.Register("en-US", map[string]string{"greet": "Hello"})

	app := iris.New()
	app.Use(Language(bundle))
	app.Get("/greet", func(ctx iris.Context) {
		lang := Lang(ctx)
		OK(ctx, map[string]interface{}{"lang": lang, "message": bundle.T(lang, "greet")})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	server := httptest.NewServer(app)
	defer server.Close()

	// zh-CN 请求
	response := doRequest(t, server.URL+"/greet", "zh-CN,zh;q=0.9")
	if response["lang"] != "zh-CN" || response["message"] != "你好" {
		t.Fatalf("unexpected zh response: %v", response)
	}
	// en-US 请求
	response = doRequest(t, server.URL+"/greet", "en-US,en;q=0.9")
	if response["lang"] != "en-US" || response["message"] != "Hello" {
		t.Fatalf("unexpected en response: %v", response)
	}
	// 无头部 → 默认
	response = doRequest(t, server.URL+"/greet", "")
	if response["lang"] != "zh-CN" {
		t.Fatalf("no header must fall back to default, got %v", response)
	}
}

// TestLanguageMiddlewareNilBundle 验证 nil bundle 安全(恒默认语言)。
func TestLanguageMiddlewareNilBundle(t *testing.T) {
	app := iris.New()
	app.Use(Language(nil))
	app.Get("/lang", func(ctx iris.Context) {
		OK(ctx, map[string]interface{}{"lang": Lang(ctx)})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	server := httptest.NewServer(app)
	defer server.Close()

	response := doRequest(t, server.URL+"/lang", "en-US")
	if response["lang"] != "zh-CN" {
		t.Fatalf("nil bundle must use default lang, got %v", response)
	}
}

// TestLangNilSafe 验证 Lang 对 nil ctx 安全。
func TestLangNilSafe(t *testing.T) {
	if Lang(nil) != i18n.DefaultLang {
		t.Fatal("Lang(nil) must return default")
	}
}

// doRequest 发起请求并解析统一响应。
func doRequest(t *testing.T, url, acceptLanguage string) map[string]interface{} {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	data, _ := body["data"].(map[string]interface{})
	return data
}
