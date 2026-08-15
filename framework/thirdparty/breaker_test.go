package thirdparty

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	apperr "github.com/Connorig/go-blackbox/component/error"
	"github.com/Connorig/go-blackbox/framework/circuit"
)

// TestClientWithBreakerOpen 熔断打开后快速失败,不再发起请求。
func TestClientWithBreakerOpen(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`oops`))
	}))
	defer server.Close()

	breaker := circuit.New(circuit.Config{
		FailureThreshold: 0.5,
		MinRequests:      3,
		Window:           60 * time.Second,
		Cooldown:         50 * time.Millisecond,
	})
	client := NewClient(Config{
		BaseURL:        server.URL,
		Signer:         NewBearerSigner("t"),
		Timeout:        3 * time.Second,
		MaxRetries:     0,
		RetryBaseDelay: 5 * time.Millisecond,
		Breaker:        breaker,
	})

	// 3 次失败触发熔断(MinRequests=3,失败率 100%)
	for i := 0; i < 3; i++ {
		_ = client.Get(context.Background(), "/api/v1/fail", nil, nil)
	}
	if breaker.State() != circuit.StateOpen {
		t.Fatalf("breaker state = %s, want open", breaker.State())
	}
	before := atomic.LoadInt32(&calls)

	// 熔断打开:快速失败,不发起请求,错误码 B0200
	err := client.Get(context.Background(), "/api/v1/fail", nil, nil)
	if err == nil {
		t.Fatal("expected error while breaker open")
	}
	if !apperr.Is(err, apperr.CodeSystemDisasterTriggered) {
		t.Fatalf("expected B0200, got: %v", err)
	}
	if atomic.LoadInt32(&calls) != before {
		t.Fatalf("request must not reach server while open: calls %d -> %d", before, atomic.LoadInt32(&calls))
	}
}

// TestClientBreakerRecovery 冷却后试探成功,熔断恢复。
func TestClientBreakerRecovery(t *testing.T) {
	var calls int32
	var down atomic.Bool
	down.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if down.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`oops`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	breaker := circuit.New(circuit.Config{
		FailureThreshold: 0.5,
		MinRequests:      3,
		Window:           60 * time.Second,
		Cooldown:         50 * time.Millisecond,
	})
	client := NewClient(Config{
		BaseURL:        server.URL,
		Signer:         NewBearerSigner("t"),
		Timeout:        3 * time.Second,
		MaxRetries:     0,
		RetryBaseDelay: 5 * time.Millisecond,
		Breaker:        breaker,
	})

	for i := 0; i < 3; i++ {
		_ = client.Get(context.Background(), "/api/v1/svc", nil, nil)
	}
	if breaker.State() != circuit.StateOpen {
		t.Fatalf("state = %s, want open", breaker.State())
	}
	// 服务恢复 + 冷却结束 → 半开放行试探 → 成功恢复 closed
	down.Store(false)
	time.Sleep(60 * time.Millisecond)
	var out struct {
		Ok bool `json:"ok"`
	}
	if err := client.Get(context.Background(), "/api/v1/svc", nil, &out); err != nil {
		t.Fatalf("probe request failed: %v", err)
	}
	if !out.Ok {
		t.Fatal("probe response not decoded")
	}
	if breaker.State() != circuit.StateClosed {
		t.Fatalf("state = %s, want closed after recovery", breaker.State())
	}
}

// TestClientBreaker4xxNotCounted 4xx 业务错误不触发熔断。
func TestClientBreaker4xxNotCounted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"bad"}`))
	}))
	defer server.Close()

	breaker := circuit.New(circuit.Config{FailureThreshold: 0.5, MinRequests: 3, Window: 60 * time.Second})
	client := NewClient(Config{
		BaseURL:        server.URL,
		Signer:         NewBearerSigner("t"),
		Timeout:        3 * time.Second,
		MaxRetries:     0,
		RetryBaseDelay: 5 * time.Millisecond,
		Breaker:        breaker,
	})
	for i := 0; i < 5; i++ {
		_ = client.Get(context.Background(), "/api/v1/bad", nil, nil)
	}
	if breaker.State() != circuit.StateClosed {
		t.Fatalf("4xx must not trip breaker, state = %s", breaker.State())
	}
}

// TestClientBreakerNil 未配置熔断器时行为不变。
func TestClientBreakerNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	client := NewClient(Config{BaseURL: server.URL, Signer: NewBearerSigner("t"), Timeout: 3 * time.Second})
	var out struct {
		Ok bool `json:"ok"`
	}
	if err := client.Get(context.Background(), "/api/v1/ok", nil, &out); err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !out.Ok {
		t.Fatal("response not decoded")
	}
}

// TestBreakerClassify 分类函数边界。
func TestBreakerClassify(t *testing.T) {
	if breakerClassify(nil) {
		t.Fatal("nil must not count as failure")
	}
	// 网络错误计失败
	if !breakerClassify(errors.New("dial tcp: connection refused")) {
		t.Fatal("network error must count as failure")
	}
	// 4xx 不计
	if breakerClassify(apperr.NewWithStatus(400, apperr.CodeRequestParamError, "bad")) {
		t.Fatal("4xx must not count as failure")
	}
	// 5xx 计
	if !breakerClassify(apperr.NewWithStatus(502, apperr.CodeThirdPartyError, "bad gateway")) {
		t.Fatal("5xx must count as failure")
	}
}
