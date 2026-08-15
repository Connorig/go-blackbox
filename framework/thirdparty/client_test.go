package thirdparty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	apperr "github.com/Connorig/go-blackbox/component/error"
)

// TestHMACSignerDeterministic 验证相同输入签名一致,不同输入不一致。
func TestHMACSignerDeterministic(t *testing.T) {
	signer := NewHMACSigner("key-1", "secret-1")
	s1, err := signer.Sign("POST", "/openapi/v1/order", "1700000000", "nonce-1", "abc")
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	s2, err := signer.Sign("POST", "/openapi/v1/order", "1700000000", "nonce-1", "abc")
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	if s1 != s2 {
		t.Fatal("hmac sign must be deterministic")
	}
	s3, err := signer.Sign("POST", "/openapi/v1/order", "1700000000", "nonce-2", "abc")
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	if s1 == s3 {
		t.Fatal("nonce change must change signature")
	}
	if signer.HeaderValue() != "HMAC key-1" {
		t.Fatalf("unexpected header value: %s", signer.HeaderValue())
	}
}

// TestClientGetAndSign 验证请求带签名头且响应正确解析。
func TestClientGetAndSign(t *testing.T) {
	var gotHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		if r.URL.Path != "/api/v1/balance" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"balance":100,"currency":"CNY"}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL: server.URL,
		Signer:  NewHMACSigner("app-key", "app-secret"),
		Timeout: 3 * time.Second,
	})

	type balance struct {
		Balance  int    `json:"balance"`
		Currency string `json:"currency"`
	}
	var out balance
	if err := client.Get(context.Background(), "/api/v1/balance", nil, &out); err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if out.Balance != 100 || out.Currency != "CNY" {
		t.Fatalf("unexpected response: %+v", out)
	}
	if gotHeaders.Get("X-App-Key") != "HMAC app-key" {
		t.Fatalf("missing X-App-Key: %v", gotHeaders)
	}
	if gotHeaders.Get("X-Timestamp") == "" || gotHeaders.Get("X-Nonce") == "" {
		t.Fatal("timestamp/nonce headers missing")
	}
	if gotHeaders.Get("X-Signature") == "" {
		t.Fatal("signature header missing")
	}
	if gotHeaders.Get("X-Body-SHA256") == "" {
		t.Fatal("body sha256 header missing")
	}
}

// TestClientPostJSON 验证 POST 请求体序列化。
func TestClientPostJSON(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL, Signer: NewBearerSigner("tok-123"), Timeout: 3 * time.Second})
	var out struct {
		Ok bool `json:"ok"`
	}
	if err := client.Post(context.Background(), "/api/v1/order", map[string]interface{}{"order_no": "NO-1", "amount": 99}, &out); err != nil {
		t.Fatalf("post failed: %v", err)
	}
	if !out.Ok {
		t.Fatal("response not decoded")
	}
	if received["order_no"] != "NO-1" {
		t.Fatalf("body not sent correctly: %v", received)
	}
}

// TestClientRetryOn5xx 验证 5xx 自动重试,4xx 不重试。
func TestClientRetryOn5xx(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		BaseURL:   server.URL,
		Signer:    NewBearerSigner("t"),
		Timeout:   3 * time.Second,
		MaxRetries: 2,
		RetryBaseDelay: 10 * time.Millisecond,
	})
	var out struct {
		Ok bool `json:"ok"`
	}
	if err := client.Get(context.Background(), "/api/v1/retry", nil, &out); err != nil {
		t.Fatalf("get failed after retries: %v", err)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}

	// 4xx:不重试
	atomic.StoreInt32(&calls, 0)
	server4xx := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"bad request"}`))
	}))
	defer server4xx.Close()
	client4xx := NewClient(Config{BaseURL: server4xx.URL, Signer: NewBearerSigner("t"), Timeout: 3 * time.Second, MaxRetries: 3, RetryBaseDelay: 5 * time.Millisecond})
	err := client4xx.Get(context.Background(), "/api/v1/fail", nil, nil)
	if err == nil {
		t.Fatal("4xx must return error")
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("4xx must not retry, got %d calls", calls)
	}
}

// TestClientErrorMapping 验证错误映射为 C 系列码。
func TestClientErrorMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`oops`))
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL, Signer: NewBearerSigner("t"), Timeout: 3 * time.Second, MaxRetries: 0, RetryBaseDelay: 5 * time.Millisecond})
	err := client.Get(context.Background(), "/api/v1/err", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !apperr.Is(err, apperr.CodeThirdPartyError) {
		t.Fatalf("error must map to CodeThirdPartyError, got: %v", err)
	}
}

// TestClientQuerySorted 验证 query 参数拼接。
func TestClientQuerySorted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "a=1&b=2") {
			t.Fatalf("query not sorted: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL, Signer: NewBearerSigner("t"), Timeout: 3 * time.Second})
	if err := client.Get(context.Background(), "/api/v1/q", map[string]string{"b": "2", "a": "1"}, nil); err != nil {
		t.Fatalf("get failed: %v", err)
	}
}
