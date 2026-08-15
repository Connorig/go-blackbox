package influx

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

// influxEnv 返回测试 InfluxDB 配置;未配置时跳过。
func influxEnv(t *testing.T) Config {
	t.Helper()
	serverURL := os.Getenv("GO_BLACKBOX_INFLUX_URL")
	token := os.Getenv("GO_BLACKBOX_INFLUX_TOKEN")
	org := os.Getenv("GO_BLACKBOX_INFLUX_ORG")
	if serverURL == "" {
		t.Skip("influx not configured: set GO_BLACKBOX_INFLUX_URL to run")
	}
	if token == "" {
		token = "test-token"
	}
	if org == "" {
		org = "test"
	}
	return Config{ServerURL: serverURL, Token: token, Org: org, Bucket: "gbx-test"}
}

// testClient 连接测试 InfluxDB。
func testClient(t *testing.T) *Client {
	t.Helper()
	client, err := NewClient(influxEnv(t))
	if err != nil {
		t.Fatalf("new influx client failed: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

// TestNewClientInvalid 非法配置报错。
func TestNewClientInvalid(t *testing.T) {
	if _, err := NewClient(Config{}); err == nil {
		t.Fatal("empty server url must fail")
	}
	if _, err := NewClient(Config{ServerURL: "http://127.0.0.1:1"}); err == nil {
		t.Fatal("unreachable server must fail")
	}
}

// TestWriteAndQuery 写入/查询闭环(需真实 InfluxDB)。
func TestWriteAndQuery(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()
	bucket := "gbx-test"

	if err := client.EnsureBucket(ctx, bucket); err != nil {
		t.Fatalf("ensure bucket failed: %v", err)
	}

	// 写入 3 个温度点
	base := time.Now().Add(-2 * time.Hour)
	for i := 0; i < 3; i++ {
		err := client.Write(ctx, bucket, "temperature",
			map[string]interface{}{"sensor": "s1", "room": "r1"},
			map[string]interface{}{"value": float64(20 + i)},
			base.Add(time.Duration(i)*time.Minute))
		if err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}
	// 行协议写入
	if err := client.WriteRaw(ctx, bucket, "temperature,sensor=s2,room=r1 value=25.5"); err != nil {
		t.Fatalf("write raw failed: %v", err)
	}

	// Flux 查询
	flux := `from(bucket:"` + bucket + `") |> range(start: -24h) |> filter(fn: (r) => r._measurement == "temperature")`
	records, err := client.Query(ctx, bucket, flux)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(records) < 4 {
		t.Fatalf("expected >= 4 records, got %d", len(records))
	}

	// 原始 CSV
	raw, err := client.QueryRaw(ctx, flux)
	if err != nil || raw == "" {
		t.Fatalf("query raw failed: %v %q", err, raw)
	}

	// 桶列表
	buckets, err := client.Buckets(ctx)
	if err != nil || len(buckets) == 0 {
		t.Fatalf("buckets = %v, %v", buckets, err)
	}
}

// TestWritePoint 预构造点写入。
func TestWritePoint(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()
	bucket := "gbx-test"

	if err := client.EnsureBucket(ctx, bucket); err != nil {
		t.Fatalf("ensure bucket failed: %v", err)
	}
	point := write.NewPoint("cpu_usage",
		map[string]string{"host": "h1"},
		map[string]interface{}{"value": 42.5},
		time.Now())
	if err := client.WritePoint(ctx, bucket, point); err != nil {
		t.Fatalf("write point failed: %v", err)
	}
}
