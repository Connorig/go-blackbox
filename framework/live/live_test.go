package live

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kataras/iris/v12"
)

// testLiveApp 构建测试 iris 应用。
func testLiveApp(t *testing.T, handlers *Handlers) *iris.Application {
	t.Helper()
	app := iris.New()
	Provide(app, Config{CallbackMount: "/api/live", Handlers: handlers})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	return app
}

// TestOnPublishAllow 推流鉴权放行。
func TestOnPublishAllow(t *testing.T) {
	app := testLiveApp(t, &Handlers{
		OnPublish: func(ctx iris.Context, info *PublishInfo) error {
			if info.Stream != "demo" {
				return errors.New("unknown stream")
			}
			if info.Param != "?key=secret" {
				return errors.New("invalid key")
			}
			return nil
		},
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/live/on_publish",
		strings.NewReader(`{"action":"on_publish","client_id":"c1","app":"live","stream":"demo","param":"?key=secret","ip":"127.0.0.1"}`))
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"code":0`) {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

// TestOnPublishDeny 推流鉴权拒绝(403 + code 1)。
func TestOnPublishDeny(t *testing.T) {
	app := testLiveApp(t, &Handlers{
		OnPublish: func(ctx iris.Context, info *PublishInfo) error {
			if info.Param != "?key=secret" {
				return errors.New("invalid stream key")
			}
			return nil
		},
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/live/on_publish",
		strings.NewReader(`{"action":"on_publish","stream":"demo","param":"?key=bad"}`))
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"code":1`) ||
		!strings.Contains(recorder.Body.String(), "invalid stream key") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

// TestOnPublishDefaultAllow 未注入 OnPublish 默认放行。
func TestOnPublishDefaultAllow(t *testing.T) {
	app := testLiveApp(t, &Handlers{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/live/on_publish",
		strings.NewReader(`{"action":"on_publish","stream":"demo"}`))
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("default must allow: %d", recorder.Code)
	}
}

// TestInvalidJSON 非 JSON body 容错不 panic。
func TestInvalidJSON(t *testing.T) {
	app := testLiveApp(t, &Handlers{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/live/on_publish", strings.NewReader("not-json"))
	app.ServeHTTP(recorder, request)
	// on_publish 解析失败 → 拒绝(403);on_connect 解析失败 → 放行
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("publish invalid json must deny: %d", recorder.Code)
	}
	recorder2 := httptest.NewRecorder()
	request2 := httptest.NewRequest(http.MethodPost, "/api/live/on_connect", strings.NewReader("not-json"))
	app.ServeHTTP(recorder2, request2)
	if recorder2.Code != http.StatusOK {
		t.Fatalf("connect invalid json must allow: %d", recorder2.Code)
	}
}

// TestNotifyCallbacks 通知型回调(下播/dvr/hls)正常 200。
func TestNotifyCallbacks(t *testing.T) {
	var unpublished, dvr bool
	app := testLiveApp(t, &Handlers{
		OnUnpublish: func(ctx iris.Context, info *UnpublishInfo) { unpublished = true },
		OnDvr:       func(ctx iris.Context, info *DvrInfo) { dvr = true },
	})
	for _, path := range []string{"/api/live/on_unpublish", "/api/live/on_dvr", "/api/live/on_hls"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"action":"x","stream":"demo"}`))
		app.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, recorder.Code)
		}
	}
	if !unpublished || !dvr {
		t.Fatalf("notify handlers not called: %v %v", unpublished, dvr)
	}
}

// ---- SRS API 客户端测试(mock SRS) ----

// mockSRS 模拟 SRS HTTP API。
func mockSRS(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/versions", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"data":{"version":"5.0.213","major":5,"minor":0,"revision":213}}`))
	})
	mux.HandleFunc("/api/v1/streams/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"streams":[{"id":"s1","name":"demo","app":"live","vhost":"__defaultVhost__",
			"url":"/live/demo","publish":{"cid":"cid-123","video":{"codec":"H264","width":1920,"height":1080},
			"audio":{"codec":"AAC"}}}]}`))
	})
	mux.HandleFunc("/api/v1/clients/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"code":0,"clients":[{"id":"cid-123","stream":"demo","app":"live","type":"publisher"}]}`))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Method Not Allowed"))
	})
	mux.HandleFunc("/api/v1/clients/cid-123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			_, _ = w.Write([]byte(`{"code":0}`))
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Method Not Allowed"))
	})
	return httptest.NewServer(mux)
}

// TestClientVersion 版本查询。
func TestClientVersion(t *testing.T) {
	server := mockSRS(t)
	defer server.Close()
	client := NewClient(server.URL, 0)
	version, err := client.Version(context.Background())
	if err != nil {
		t.Fatalf("version failed: %v", err)
	}
	if version.Version != "5.0.213" {
		t.Fatalf("version = %q", version.Version)
	}
}

// TestClientListStreams 流列表 + 字段提取。
func TestClientListStreams(t *testing.T) {
	server := mockSRS(t)
	defer server.Close()
	client := NewClient(server.URL, 0)
	streams, err := client.ListStreams(context.Background())
	if err != nil {
		t.Fatalf("list streams failed: %v", err)
	}
	if len(streams) != 1 {
		t.Fatalf("streams = %d", len(streams))
	}
	stream := streams[0]
	if stream.Name != "demo" || stream.PublishCID != "cid-123" ||
		stream.VideoCodec != "H264" || stream.Width != 1920 || stream.Height != 1080 ||
		stream.AudioCodec != "AAC" {
		t.Fatalf("stream = %+v", stream)
	}
}

// TestClientKickStream 踢流两步法。
func TestClientKickStream(t *testing.T) {
	server := mockSRS(t)
	defer server.Close()
	client := NewClient(server.URL, 0)
	if err := client.KickStream(context.Background(), "demo"); err != nil {
		t.Fatalf("kick stream failed: %v", err)
	}
	// 流不存在 → 明确错误
	if err := client.KickStream(context.Background(), "not-exist"); err == nil ||
		!strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing stream error = %v", err)
	}
}

// TestClientListClients 客户端列表。
func TestClientListClients(t *testing.T) {
	server := mockSRS(t)
	defer server.Close()
	client := NewClient(server.URL, 0)
	clients, err := client.ListClients(context.Background())
	if err != nil {
		t.Fatalf("list clients failed: %v", err)
	}
	if len(clients) != 1 || clients[0].ID != "cid-123" {
		t.Fatalf("clients = %+v", clients)
	}
}

// TestClientNonJSONTolerance 非 JSON 响应容错(不 panic)。
func TestClientNonJSONTolerance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Method Not Allowed"))
	}))
	defer server.Close()
	client := NewClient(server.URL, 0)
	if _, err := client.ListStreams(context.Background()); err == nil {
		t.Fatal("405 must return structured error")
	}
	if _, err := client.Version(context.Background()); err == nil {
		t.Fatal("405 must return structured error")
	}
}

// TestGlobalGetter live 全局入口。
func TestGlobalGetter(t *testing.T) {
	SetGlobal(nil)
	if Get() != nil {
		t.Fatal("global must be nil")
	}
	client := NewClient("http://127.0.0.1:1985", 0)
	SetGlobal(client)
	if Get() != client {
		t.Fatal("Get must return set client")
	}
	SetGlobal(nil)
}
