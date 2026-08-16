package httpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestGetQueryHeaders GET + query + headers + JSON 解析。
func TestGetQueryHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		if r.Header.Get("X-Token") != "tok-1" || r.Header.Get("X-Default") != "def" {
			t.Errorf("headers = %v", r.Header)
		}
		_, _ = w.Write([]byte(`{"name":"connor","age":30}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL, DefaultHeaders: map[string]string{"X-Default": "def"}})
	type user struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	var out user
	response, err := client.Get(context.Background(), "/api/v1/users", Options{
		Query:   map[string]string{"page": "2"},
		Headers: map[string]string{"X-Token": "tok-1"},
	}, &out)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !response.IsSuccess() || out.Name != "connor" || out.Age != 30 {
		t.Fatalf("response = %+v out = %+v", response, out)
	}
}

// TestPostJSON POST JSON。
func TestPostJSON(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	var out struct {
		Ok bool `json:"ok"`
	}
	if _, err := client.Post(context.Background(), "/api/v1/orders", Options{
		JSON: map[string]interface{}{"order_no": "NO-1", "amount": 99},
	}, &out); err != nil {
		t.Fatalf("post failed: %v", err)
	}
	if received["order_no"] != "NO-1" || !out.Ok {
		t.Fatalf("received = %v out = %+v", received, out)
	}
}

// TestPostForm 表单提交。
func TestPostForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		_ = r.ParseForm()
		if r.FormValue("username") != "connor" {
			t.Errorf("form = %v", r.Form)
		}
		_, _ = w.Write([]byte(`{"login":true}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	var out struct {
		Login bool `json:"login"`
	}
	if _, err := client.Post(context.Background(), "/api/v1/login", Options{
		Form: map[string]string{"username": "connor", "password": "***"},
	}, &out); err != nil {
		t.Fatalf("post form failed: %v", err)
	}
	if !out.Login {
		t.Fatal("login must be true")
	}
}

// TestRawBody 原始 body + 自定义 ContentType。
func TestRawBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/xml" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		buf := make([]byte, 64)
		n, _ := r.Body.Read(buf)
		if !strings.Contains(string(buf[:n]), "<order>") {
			t.Errorf("body = %q", buf[:n])
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	response, err := client.Post(context.Background(), "/api/v1/order", Options{
		Body:        []byte("<order>1001</order>"),
		ContentType: "application/xml",
	}, nil)
	if err != nil {
		t.Fatalf("post raw failed: %v", err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

// TestPutDelete PUT/DELETE 方法。
func TestPutDelete(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL})
	_, _ = client.Put(context.Background(), "/api/v1/orders/1", Options{JSON: map[string]string{"status": "done"}}, nil)
	_, _ = client.Delete(context.Background(), "/api/v1/orders/1", Options{}, nil)
	if len(methods) != 2 || methods[0] != http.MethodPut || methods[1] != http.MethodDelete {
		t.Fatalf("methods = %v", methods)
	}
}

// TestRetryOnNetworkError 网络错误重试。
func TestRetryOnNetworkError(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 2 {
			// 模拟网络中断:关闭连接
			hijacker, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hijacker.Hijack()
				_ = conn.Close()
				return
			}
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL, MaxRetries: 3})
	var out struct {
		Ok bool `json:"ok"`
	}
	response, err := client.Get(context.Background(), "/api/v1/retry", Options{}, &out)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if !response.IsSuccess() || !out.Ok || calls != 2 {
		t.Fatalf("response = %+v calls = %d", response, calls)
	}
}

// TestTimeout 超时。
func TestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := New(Config{BaseURL: server.URL, Timeout: 100 * time.Millisecond})
	if _, err := client.Get(context.Background(), "/api/v1/slow", Options{}, nil); err == nil {
		t.Fatal("timeout must fail")
	}
}

// TestGlobalGetter 全局便捷入口。
func TestGlobalGetter(t *testing.T) {
	if Get() != nil {
		t.Fatal("global must be nil before set")
	}
	client := New(Config{})
	SetGlobal(client)
	if Get() != client {
		t.Fatal("Get must return the set client")
	}
	SetGlobal(nil)
}
