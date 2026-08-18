package live

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
)

// doRequest 通过真实 HTTP server 执行请求(iris 需经 Serve 初始化)。
func doRequest(app *iris.Application, method, path, body, signature string) *http.Response {
	_ = app.Build() // iris 12.2.5 需 Build 后 ServeHTTP
	server := httptest.NewServer(app)
	defer server.Close()
	req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
	if signature != "" {
		req.Header.Set("X-SRS-Signature", signature)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	return response
}

// TestSignVerify 签名生成与校验往返。
func TestSignVerify(t *testing.T) {
	secret := "shared-secret-123"
	verify := NewCallbackSignature(secret, "")
	app := iris.New()
	var verified bool
	app.Post("/api/live/on_publish", verify.Wrap(), func(ctx iris.Context) {
		verified = true
		_, _ = ctx.WriteString("ok")
	})

	body := "{\"action\":\"on_publish\"}"
	signature := SignPayload(secret, "POST", "/api/live/on_publish", []byte(body))
	response := doRequest(app, http.MethodPost, "/api/live/on_publish", body, signature)
	defer response.Body.Close()
	if !verified || response.StatusCode != http.StatusOK {
		t.Fatalf("verified=%v code=%d", verified, response.StatusCode)
	}
}

// TestSignVerifyReject 无签名/错误签名拒绝 403。
func TestSignVerifyReject(t *testing.T) {
	verify := NewCallbackSignature("secret-1", "")
	app := iris.New()
	app.Post("/cb", verify.Wrap(), func(ctx iris.Context) { _, _ = ctx.WriteString("ok") })

	response := doRequest(app, http.MethodPost, "/cb", "{}", "")
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("no signature must be 403, got %d", response.StatusCode)
	}
	response = doRequest(app, http.MethodPost, "/cb", "{}", "wrong")
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong signature must be 403, got %d", response.StatusCode)
	}
}

// TestSignEmptySecret 空密钥时验签关闭(对接期友好)。
func TestSignEmptySecret(t *testing.T) {
	verify := NewCallbackSignature("", "")
	app := iris.New()
	app.Post("/cb", verify.Wrap(), func(ctx iris.Context) { _, _ = ctx.WriteString("ok") })
	response := doRequest(app, http.MethodPost, "/cb", "{}", "")
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("empty secret must pass, got %d", response.StatusCode)
	}
}

// TestDvrClient 录制 API 客户端(httptest 模拟 SRS)。
func TestDvrClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/dvr/start":
			_, _ = w.Write([]byte(`{"code":0,"data":"rec-001"}`))
		case "/api/v1/dvr/stop":
			_, _ = w.Write([]byte(`{"code":0}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	ctx := context.Background()
	taskID, err := client.StartRecord(ctx, "test-stream", "/data/rec.mp4")
	if err != nil || taskID != "rec-001" {
		t.Fatalf("start: id=%q err=%v", taskID, err)
	}
	if err := client.StopRecord(ctx, "test-stream"); err != nil {
		t.Fatalf("stop: %v", err)
	}
}
