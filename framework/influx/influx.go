// Package influx 提供 InfluxDB 时序数据库集成(对标 Spring Data InfluxDB 的
// InfluxDBTemplate):写入/查询封装 + 原生客户端暴露。支持 InfluxDB 2.x
// (及 1.8+ 的兼容模式:ServerURL 指向 /api/v2 或 8086 根地址 + 1.x token)。
package influx

import (
	"context"
	"errors"
	"fmt"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

// Config InfluxDB 客户端配置。
type Config struct {
	// ServerURL 服务地址(如 http://127.0.0.1:8086)。
	ServerURL string
	// Token 访问令牌(InfluxDB 2.x Token;1.x 兼容模式用 user:password 的 base64)。
	Token string
	// Org 组织(InfluxDB 2.x 概念)。
	Org string
	// Bucket 默认桶(可空,操作时显式指定)。
	Bucket string
	// Timeout 请求超时(默认 10s)。
	Timeout time.Duration
}

// normalize 补齐默认值。
func (c Config) normalize() Config {
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	return c
}

// InfluxTemplate 高频操作层:写入(点/行协议)/查询(Flux)/健康检查 + 原生客户端。
type InfluxTemplate interface {
	// Ping 健康检查。
	Ping(ctx context.Context) error
	// Write 写入一个数据点(measurement + tags + fields + 时间戳)。
	// timestamp 为零值时使用当前时间。
	Write(ctx context.Context, bucket, measurement string,
		tags, fields map[string]interface{}, timestamp time.Time) error
	// WritePoint 写入预构造的数据点。
	WritePoint(ctx context.Context, bucket string, point *write.Point) error
	// WriteRaw 写入行协议(Line Protocol)原始字符串。
	WriteRaw(ctx context.Context, bucket, lineProtocol string) error
	// Query 执行 Flux 查询,返回记录列表(每行 map:列名 → 值)。
	Query(ctx context.Context, bucket, flux string) ([]map[string]interface{}, error)
	// QueryRaw 执行 Flux 查询,返回原始 CSV 文本。
	QueryRaw(ctx context.Context, flux string) (string, error)
	// Buckets 列出全部桶(管理用)。
	Buckets(ctx context.Context) ([]string, error)
	// Client 返回原生客户端(高级操作入口)。
	Client() influxdb2.Client
}

// Client 是 InfluxDB 模板实现。
type Client struct {
	client influxdb2.Client
	org    string
	bucket string
}

// NewClient 创建 InfluxDB 客户端并验证连通性。
func NewClient(config Config) (*Client, error) {
	cfg := config.normalize()
	if cfg.ServerURL == "" {
		return nil, errors.New("influx: server url is required")
	}
	influxClient := influxdb2.NewClient(cfg.ServerURL, cfg.Token)
	client := &Client{client: influxClient, org: cfg.Org, bucket: cfg.Bucket}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		influxClient.Close()
		return nil, fmt.Errorf("influx: ping %s: %w", cfg.ServerURL, err)
	}
	SetGlobal(client)
	return client, nil
}

// Ping 健康检查。
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.client == nil {
		return errors.New("influx: client is nil")
	}
	ready, err := c.client.Ready(ctx)
	if err != nil {
		return err
	}
	if ready == nil {
		return errors.New("influx: not ready")
	}
	return nil
}

// Write 写入数据点。
func (c *Client) Write(ctx context.Context, bucket, measurement string,
	tags, fields map[string]interface{}, timestamp time.Time) error {
	if c == nil || c.client == nil {
		return errors.New("influx: client is nil")
	}
	if bucket == "" {
		bucket = c.bucket
	}
	if bucket == "" {
		return errors.New("influx: bucket is required")
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	point := write.NewPoint(measurement, stringMap(tags), fields, timestamp)
	return c.WritePoint(ctx, bucket, point)
}

// WritePoint 写入预构造点。
func (c *Client) WritePoint(ctx context.Context, bucket string, point *write.Point) error {
	if c == nil || c.client == nil {
		return errors.New("influx: client is nil")
	}
	if bucket == "" {
		bucket = c.bucket
	}
	if point == nil {
		return errors.New("influx: point is nil")
	}
	return c.client.WriteAPIBlocking(c.org, bucket).WritePoint(ctx, point)
}

// WriteRaw 写入行协议。
func (c *Client) WriteRaw(ctx context.Context, bucket, lineProtocol string) error {
	if c == nil || c.client == nil {
		return errors.New("influx: client is nil")
	}
	if bucket == "" {
		bucket = c.bucket
	}
	if lineProtocol == "" {
		return errors.New("influx: line protocol is empty")
	}
	return c.client.WriteAPIBlocking(c.org, bucket).WriteRecord(ctx, lineProtocol)
}

// Query 执行 Flux 查询。
func (c *Client) Query(ctx context.Context, bucket, flux string) ([]map[string]interface{}, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("influx: client is nil")
	}
	if flux == "" {
		return nil, errors.New("influx: flux query is empty")
	}
	queryAPI := c.client.QueryAPI(c.org)
	result, err := queryAPI.Query(ctx, flux)
	if err != nil {
		return nil, fmt.Errorf("influx: query: %w", err)
	}
	defer result.Close()

	var records []map[string]interface{}
	for result.Next() {
		record := result.Record()
		row := make(map[string]interface{})
		row["_time"] = record.Time()
		row["_measurement"] = record.Measurement()
		row["_field"] = record.Field()
		row["_value"] = record.Value()
		for key, value := range record.Values() {
			row[key] = value
		}
		records = append(records, row)
	}
	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("influx: query result: %w", err)
	}
	return records, nil
}

// QueryRaw 执行 Flux 查询返回原始 CSV。
func (c *Client) QueryRaw(ctx context.Context, flux string) (string, error) {
	if c == nil || c.client == nil {
		return "", errors.New("influx: client is nil")
	}
	queryAPI := c.client.QueryAPI(c.org)
	table, err := queryAPI.QueryRaw(ctx, flux, api.DefaultDialect())
	if err != nil {
		return "", fmt.Errorf("influx: query raw: %w", err)
	}
	return table, nil
}

// Buckets 列出全部桶(管理用)。
func (c *Client) Buckets(ctx context.Context) ([]string, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("influx: client is nil")
	}
	bucketsAPI := c.client.BucketsAPI()
	buckets, err := bucketsAPI.GetBuckets(ctx)
	if err != nil {
		return nil, fmt.Errorf("influx: list buckets: %w", err)
	}
	names := make([]string, 0, len(*buckets))
	for _, bucket := range *buckets {
		if bucket.Name != "" {
			names = append(names, bucket.Name)
		}
	}
	return names, nil
}

// Client 返回原生客户端。
func (c *Client) Client() influxdb2.Client {
	if c == nil {
		return nil
	}
	return c.client
}

// Close 关闭客户端(释放连接池)。
func (c *Client) Close() {
	if c != nil && c.client != nil {
		c.client.Close()
	}
}

// stringMap 将 interface{} 值转为字符串标签(map[string]string)。
func stringMap(values map[string]interface{}) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = fmt.Sprintf("%v", value)
	}
	return result
}

// EnsureBucket 确保桶存在(不存在则创建;管理/初始化用)。
// 需要 orgID:从组织名解析。
func (c *Client) EnsureBucket(ctx context.Context, bucket string) error {
	if c == nil || c.client == nil {
		return errors.New("influx: client is nil")
	}
	orgAPI := c.client.OrganizationsAPI()
	org, err := orgAPI.FindOrganizationByName(ctx, c.org)
	if err != nil {
		return fmt.Errorf("influx: find org: %w", err)
	}
	bucketsAPI := c.client.BucketsAPI()
	if _, err := bucketsAPI.FindBucketByName(ctx, bucket); err == nil {
		return nil // 已存在
	}
	_, err = bucketsAPI.CreateBucketWithName(ctx, org, bucket, domain.RetentionRule{EverySeconds: 0})
	if err != nil {
		return fmt.Errorf("influx: create bucket %s: %w", bucket, err)
	}
	return nil
}
