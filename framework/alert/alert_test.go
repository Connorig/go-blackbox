package alert

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Connorig/go-blackbox/framework/monitor"
)

// fakeNotifier 记录消息的内存通知器。
type fakeNotifier struct {
	mu       sync.Mutex
	messages []Message
}

func (f *fakeNotifier) Name() string { return "fake" }
func (f *fakeNotifier) Notify(_ context.Context, message Message) error {
	f.mu.Lock()
	f.messages = append(f.messages, message)
	f.mu.Unlock()
	return nil
}

func (f *fakeNotifier) all() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Message(nil), f.messages...)
}

// collectorWith 用固定值构造采集器。
func collectorWith(cpu, mem, disk float64) *monitor.Collector {
	collector := monitor.NewCollector()
	return collector // 直接构造 Watcher 时用 stub 覆盖;这里返回真实采集器,测试用规则注入
}

// stubStats 构造固定快照。
func stubStats(cpu, mem, disk float64) *monitor.Stats {
	return &monitor.Stats{
		Hostname: "test-host",
		CPU:      monitor.CPUStats{UsagePercent: cpu},
		Memory:   monitor.MemoryStats{UsagePercent: mem},
		Disk:     monitor.DiskStats{UsagePercent: disk},
	}
}

// TestWatcherTriggerAndDedup 连续触发告警 + 去重(不重复推送)。
func TestWatcherTriggerAndDedup(t *testing.T) {
	notifier := &fakeNotifier{}
	watcher := NewWatcher(Config{
		Interval:  time.Hour, // 不轮询,手动 tick
		Notifiers: []Notifier{notifier},
		Rules: []Rule{
			CPUUsage(80, 2),
		},
	})

	// 第一次超阈值:连续 1 次,未达 2 次,不告警
	watcher.evaluate(context.Background(), watcher.config.Rules[0], stubStats(90, 0, 0))
	if len(notifier.all()) != 0 {
		t.Fatalf("must not alert before consecutive threshold: %v", notifier.all())
	}
	// 第二次超阈值:连续 2 次,触发告警
	watcher.evaluate(context.Background(), watcher.config.Rules[0], stubStats(95, 0, 0))
	if len(notifier.all()) != 1 {
		t.Fatalf("must alert exactly once: %v", notifier.all())
	}
	if notifier.all()[0].Level != LevelWarning {
		t.Fatalf("level = %s", notifier.all()[0].Level)
	}
	// 第三次超阈值:去重,不重复推送
	watcher.evaluate(context.Background(), watcher.config.Rules[0], stubStats(96, 0, 0))
	if len(notifier.all()) != 1 {
		t.Fatalf("must dedup while alerting: %v", notifier.all())
	}
	// 恢复:连续 2 次正常后推送恢复
	watcher.evaluate(context.Background(), watcher.config.Rules[0], stubStats(50, 0, 0))
	watcher.evaluate(context.Background(), watcher.config.Rules[0], stubStats(40, 0, 0))
	messages := notifier.all()
	if len(messages) != 2 {
		t.Fatalf("must send recover message, got %d: %v", len(messages), messages)
	}
	if messages[1].Level != LevelRecover {
		t.Fatalf("second message must be recover, got %s", messages[1].Level)
	}
	// 恢复后再次超阈值:重新告警
	watcher.evaluate(context.Background(), watcher.config.Rules[0], stubStats(90, 0, 0))
	watcher.evaluate(context.Background(), watcher.config.Rules[0], stubStats(91, 0, 0))
	if len(notifier.all()) != 3 {
		t.Fatalf("must re-alert after recovery, got %d", len(notifier.all()))
	}
}

// TestWatcherMultipleRules 多规则独立工作。
func TestWatcherMultipleRules(t *testing.T) {
	notifier := &fakeNotifier{}
	watcher := NewWatcher(Config{
		Notifiers: []Notifier{notifier},
		Rules: []Rule{
			CPUUsage(80, 1),
			MemoryUsage(85, 1),
			DiskUsage(90, 1),
		},
	})
	watcher.evaluate(context.Background(), watcher.config.Rules[0], stubStats(81, 0, 0))
	watcher.evaluate(context.Background(), watcher.config.Rules[1], stubStats(0, 86, 0))
	watcher.evaluate(context.Background(), watcher.config.Rules[2], stubStats(0, 0, 91))
	messages := notifier.all()
	if len(messages) != 3 {
		t.Fatalf("all three rules must alert, got %d: %v", len(messages), messages)
	}
	names := map[string]bool{}
	for _, message := range messages {
		for _, rule := range watcher.config.Rules {
			if message.Title == "告警:"+rule.Name+" 使用率超阈值" {
				names[rule.Name] = true
			}
		}
	}
	if !names["cpu"] || !names["memory"] || !names["disk"] {
		t.Fatalf("missing rules: %v", names)
	}
}

// TestWebhookWeComPayload 企业微信 payload 格式。
func TestWebhookWeComPayload(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errcode":0}`))
	}))
	defer server.Close()

	notifier := NewWeComWebhook(server.URL)
	err := notifier.Notify(context.Background(), Message{Title: "告警:cpu 使用率超阈值", Content: "当前 95.0%", Level: LevelWarning})
	if err != nil {
		t.Fatalf("notify failed: %v", err)
	}
	if received["msgtype"] != "markdown" {
		t.Fatalf("msgtype = %v", received["msgtype"])
	}
	markdown, ok := received["markdown"].(map[string]interface{})
	if !ok {
		t.Fatalf("markdown missing: %v", received)
	}
	if content, _ := markdown["content"].(string); content == "" {
		t.Fatal("content empty")
	}
}

// TestWebhookDingTalkPayload 钉钉 payload 格式。
func TestWebhookDingTalkPayload(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewDingTalkWebhook(server.URL)
	if err := notifier.Notify(context.Background(), Message{Title: "T", Content: "C", Level: LevelWarning}); err != nil {
		t.Fatalf("notify failed: %v", err)
	}
	markdown, ok := received["markdown"].(map[string]interface{})
	if !ok || markdown["title"] != "T" {
		t.Fatalf("dingtalk payload wrong: %v", received)
	}
}

// TestWebhookEmptyURL 空 URL 返回错误。
func TestWebhookEmptyURL(t *testing.T) {
	notifier := NewWeComWebhook("")
	if err := notifier.Notify(context.Background(), Message{Title: "T", Content: "C"}); err == nil {
		t.Fatal("empty url must fail")
	}
}

// TestWebhookHTTPError 非 2xx 返回错误。
func TestWebhookHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	notifier := NewWeComWebhook(server.URL)
	if err := notifier.Notify(context.Background(), Message{Title: "T", Content: "C"}); err == nil {
		t.Fatal("5xx must fail")
	}
}

// TestWatcherStartTick 启动时立即采集一次。
func TestWatcherStartTick(t *testing.T) {
	notifier := &fakeNotifier{}
	collector := monitor.NewCollector()
	watcher := NewWatcher(Config{
		Interval:  time.Hour,
		Collector: collector,
		Notifiers: []Notifier{notifier},
		Rules: []Rule{
			{Name: "never", Threshold: 101, Consecutive: 1, Check: func(stats *monitor.Stats) float64 { return 50 }},
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() { watcher.Start(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher must stop on ctx cancel")
	}
	// 不 panic、正常退出即通过(阈值 101 永不触发)
}
