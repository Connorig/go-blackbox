package openapi

import (
	"bytes"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	apperr "github.com/Connorig/go-blackbox/component/error"
	"github.com/kataras/iris/v12"
)

// hmacHeaders 构造合法 HMAC 签名请求头。
func hmacHeaders(t *testing.T, app *App, method, path string, body []byte, timestamp, nonce string) map[string]string {
	t.Helper()
	stringToSign := StringToSign(method, path, timestamp, nonce, BodySHA256(body))
	mac := hmac.New(sha256.New, []byte(app.AppSecret))
	_, _ = mac.Write([]byte(stringToSign))
	return map[string]string{
		HeaderAppKey:     app.AppKey,
		HeaderTimestamp:  timestamp,
		HeaderNonce:      nonce,
		HeaderSignature:  hex.EncodeToString(mac.Sum(nil)),
		HeaderBodySHA256: BodySHA256(body),
	}
}

// newTestApp 构建测试用 iris 应用 + 开放接口。
func newTestApp(t *testing.T, apps ...*App) (*iris.Application, *Registry, *OpenAPI) {
	t.Helper()
	app := iris.New()
	registry := NewRegistryWith(apps...)
	api := New(app, Config{Registry: registry})
	api.GET("/v1/order/query", func(ctx iris.Context) {
		_ = ctx.JSON(map[string]interface{}{"order_id": ctx.URLParam("order_id"), "app": AppKey(ctx)})
	})
	api.POST("/v1/order/update", func(ctx iris.Context) {
		var body map[string]interface{}
		if err := ctx.ReadJSON(&body); err != nil {
			ctx.StatusCode(http.StatusBadRequest)
			return
		}
		_ = ctx.JSON(map[string]interface{}{"ok": true, "received": body["order_no"]})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build iris app failed: %v", err)
	}
	return app, registry, api
}

// doRequest 执行请求并返回响应。
func doRequest(t *testing.T, app *iris.Application, method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, request)
	return recorder
}

// TestGatewayHMACSuccess 验证合法签名请求通过,handler 拿到 app 标识。
func TestGatewayHMACSuccess(t *testing.T) {
	app, _, _ := newTestApp(t, &App{AppKey: "company-001", AppSecret: "secret-1", Algorithm: AlgHMAC, Enabled: true})
	headers := hmacHeaders(t, &App{AppKey: "company-001", AppSecret: "secret-1", Algorithm: AlgHMAC}, http.MethodGet, "/openapi/v1/order/query", nil, strconv.FormatInt(time.Now().Unix(), 10), "nonce-abc")
	recorder := doRequest(t, app, http.MethodGet, "/openapi/v1/order/query?order_id=100", nil, headers)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte(`"app":"company-001"`)) {
		t.Fatalf("app key not injected: %s", recorder.Body.String())
	}
}

// TestGatewayHMACSuccessWithBody 验证带请求体的签名校验(POST)。
func TestGatewayHMACSuccessWithBody(t *testing.T) {
	appCfg := &App{AppKey: "company-001", AppSecret: "secret-1", Algorithm: AlgHMAC, Enabled: true}
	app, _, _ := newTestApp(t, appCfg)
	body := []byte(`{"order_no":"NO-1"}`)
	headers := hmacHeaders(t, appCfg, http.MethodPost, "/openapi/v1/order/update", body, strconv.FormatInt(time.Now().Unix(), 10), "nonce-post-1")
	recorder := doRequest(t, app, http.MethodPost, "/openapi/v1/order/update", body, headers)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte(`"received":"NO-1"`)) {
		t.Fatalf("handler did not receive body: %s", recorder.Body.String())
	}
}

// TestGatewaySignatureMismatch 错误签名返回 401 A0340。
func TestGatewaySignatureMismatch(t *testing.T) {
	app, _, _ := newTestApp(t, &App{AppKey: "company-001", AppSecret: "secret-1", Algorithm: AlgHMAC, Enabled: true})
	// 用错误密钥签名
	wrong := &App{AppKey: "company-001", AppSecret: "wrong-secret"}
	headers := hmacHeaders(t, wrong, http.MethodGet, "/openapi/v1/order/query", nil, strconv.FormatInt(time.Now().Unix(), 10), "nonce-bad")
	recorder := doRequest(t, app, http.MethodGet, "/openapi/v1/order/query", nil, headers)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("A0340")) {
		t.Fatalf("expected A0340, body = %s", recorder.Body.String())
	}
}

// TestGatewayMissingHeaders 缺头返回 401。
func TestGatewayMissingHeaders(t *testing.T) {
	app, _, _ := newTestApp(t, &App{AppKey: "company-001", AppSecret: "secret-1", Enabled: true})
	recorder := doRequest(t, app, http.MethodGet, "/openapi/v1/order/query", nil, nil)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}

// TestGatewayTimestampWindow 时间戳超窗返回 401 A0230。
func TestGatewayTimestampWindow(t *testing.T) {
	appCfg := &App{AppKey: "company-001", AppSecret: "secret-1", Enabled: true}
	app, _, _ := newTestApp(t, appCfg)
	old := strconv.FormatInt(time.Now().Add(-30*time.Minute).Unix(), 10)
	headers := hmacHeaders(t, appCfg, http.MethodGet, "/openapi/v1/order/query", nil, old, "nonce-old")
	recorder := doRequest(t, app, http.MethodGet, "/openapi/v1/order/query", nil, headers)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("A0230")) {
		t.Fatalf("expected A0230, body = %s", recorder.Body.String())
	}
}

// TestGatewayNonceReplay 重复 nonce 返回 A0506。
func TestGatewayNonceReplay(t *testing.T) {
	appCfg := &App{AppKey: "company-001", AppSecret: "secret-1", Enabled: true}
	app, _, _ := newTestApp(t, appCfg)
	headers := hmacHeaders(t, appCfg, http.MethodGet, "/openapi/v1/order/query", nil, strconv.FormatInt(time.Now().Unix(), 10), "nonce-replay")
	first := doRequest(t, app, http.MethodGet, "/openapi/v1/order/query", nil, headers)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d", first.Code)
	}
	second := doRequest(t, app, http.MethodGet, "/openapi/v1/order/query", nil, headers)
	if second.Code != http.StatusBadRequest {
		t.Fatalf("replay status = %d, body = %s", second.Code, second.Body.String())
	}
	if !bytes.Contains(second.Body.Bytes(), []byte("A0506")) {
		t.Fatalf("expected A0506, body = %s", second.Body.String())
	}
}

// TestGatewayDisabledApp 禁用应用返回 401。
func TestGatewayDisabledApp(t *testing.T) {
	app, _, _ := newTestApp(t, &App{AppKey: "company-001", AppSecret: "secret-1", Enabled: false})
	headers := hmacHeaders(t, &App{AppKey: "company-001", AppSecret: "secret-1"}, http.MethodGet, "/openapi/v1/order/query", nil, strconv.FormatInt(time.Now().Unix(), 10), "nonce-disabled")
	recorder := doRequest(t, app, http.MethodGet, "/openapi/v1/order/query", nil, headers)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}

// TestGatewayRSASuccess 验证 RSA 签名链路(生成密钥 → 签名 → 网关验签)。
func TestGatewayRSASuccess(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: mustMarshalPKIX(t, &privateKey.PublicKey)})
	appCfg := &App{AppKey: "rsa-app", PublicKey: string(publicKeyPEM), Algorithm: AlgRSA, Enabled: true}
	app, _, _ := newTestApp(t, appCfg)

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "nonce-rsa"
	path := "/openapi/v1/order/query"
	stringToSign := StringToSign(http.MethodGet, path, timestamp, nonce, BodySHA256(nil))
	digest := sha256.Sum256([]byte(stringToSign))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	headers := map[string]string{
		HeaderAppKey:    appCfg.AppKey,
		HeaderTimestamp: timestamp,
		HeaderNonce:     nonce,
		HeaderSignature: base64.StdEncoding.EncodeToString(signature),
	}
	recorder := doRequest(t, app, http.MethodGet, path, nil, headers)
	if recorder.Code != http.StatusOK {
		t.Fatalf("rsa status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func mustMarshalPKIX(t *testing.T, key *rsa.PublicKey) []byte {
	t.Helper()
	data, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatalf("marshal pkix failed: %v", err)
	}
	return data
}

// TestGatewayRateLimit 超限返回 429。
func TestGatewayRateLimit(t *testing.T) {
	appCfg := &App{AppKey: "company-001", AppSecret: "secret-1", Enabled: true, RatePerSecond: 2, Burst: 2}
	app, _, _ := newTestApp(t, appCfg)
	var lastCode int
	for i := 0; i < 5; i++ {
		headers := hmacHeaders(t, appCfg, http.MethodGet, "/openapi/v1/order/query", nil, strconv.FormatInt(time.Now().Unix(), 10), fmt.Sprintf("nonce-%d", i))
		recorder := doRequest(t, app, http.MethodGet, "/openapi/v1/order/query", nil, headers)
		lastCode = recorder.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on overflow, got %d", lastCode)
	}
}

// TestRegistryHotUpdate 热更新:改密钥后旧签名立即失效,新密钥生效。
func TestRegistryHotUpdate(t *testing.T) {
	registry := NewRegistryWith(&App{AppKey: "company-001", AppSecret: "old-secret", Enabled: true})
	app := iris.New()
	api := New(app, Config{Registry: registry})
	api.GET("/v1/ping", func(ctx iris.Context) { _ = ctx.JSON(map[string]string{"pong": "ok"}) })

	if err := app.Build(); err != nil {
		t.Fatalf("build iris app failed: %v", err)
	}

	// 旧密钥签名成功
	old := &App{AppKey: "company-001", AppSecret: "old-secret"}
	headers := hmacHeaders(t, old, http.MethodGet, "/openapi/v1/ping", nil, strconv.FormatInt(time.Now().Unix(), 10), "nonce-old")
	if recorder := doRequest(t, app, http.MethodGet, "/openapi/v1/ping", nil, headers); recorder.Code != http.StatusOK {
		t.Fatalf("old secret should pass: %d", recorder.Code)
	}
	// 热更新密钥
	registry.Set(&App{AppKey: "company-001", AppSecret: "new-secret", Enabled: true})
	headers2 := hmacHeaders(t, old, http.MethodGet, "/openapi/v1/ping", nil, strconv.FormatInt(time.Now().Unix(), 10), "nonce-old2")
	if recorder := doRequest(t, app, http.MethodGet, "/openapi/v1/ping", nil, headers2); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("old secret must fail after rotation: %d", recorder.Code)
	}
	updated := &App{AppKey: "company-001", AppSecret: "new-secret"}
	headers3 := hmacHeaders(t, updated, http.MethodGet, "/openapi/v1/ping", nil, strconv.FormatInt(time.Now().Unix(), 10), "nonce-new")
	if recorder := doRequest(t, app, http.MethodGet, "/openapi/v1/ping", nil, headers3); recorder.Code != http.StatusOK {
		t.Fatalf("new secret should pass: %d", recorder.Code)
	}
}

// TestAuditHook 审计回调覆盖成功与失败调用。
func TestAuditHook(t *testing.T) {
	var auditResults []bool
	appCfg := &App{AppKey: "company-001", AppSecret: "secret-1", Enabled: true}
	app := iris.New()
	registry := NewRegistryWith(appCfg)
	api := New(app, Config{Registry: registry, OnAudit: func(ctx iris.Context, appKey string, ok bool, code apperr.Code) {
		auditResults = append(auditResults, ok)
	}})
	api.GET("/v1/ping", func(ctx iris.Context) { _ = ctx.JSON(map[string]string{"pong": "ok"}) })
	if err := app.Build(); err != nil {
		t.Fatalf("build iris app failed: %v", err)
	}

	// 成功请求
	good := hmacHeaders(t, appCfg, http.MethodGet, "/openapi/v1/ping", nil, strconv.FormatInt(time.Now().Unix(), 10), "nonce-audit-1")
	if recorder := doRequest(t, app, http.MethodGet, "/openapi/v1/ping", nil, good); recorder.Code != http.StatusOK {
		t.Fatalf("good request failed: %d", recorder.Code)
	}
	// 失败请求(错误签名)
	bad := hmacHeaders(t, &App{AppKey: "company-001", AppSecret: "nope"}, http.MethodGet, "/openapi/v1/ping", nil, strconv.FormatInt(time.Now().Unix(), 10), "nonce-audit-2")
	if recorder := doRequest(t, app, http.MethodGet, "/openapi/v1/ping", nil, bad); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("bad request should fail: %d", recorder.Code)
	}

	if len(auditResults) != 2 {
		t.Fatalf("audit calls = %d, want 2: %v", len(auditResults), auditResults)
	}
	if !auditResults[0] || auditResults[1] {
		t.Fatalf("audit flags wrong: %v", auditResults)
	}
}
