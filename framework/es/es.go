// Package es 提供 ElasticSearch 集成(对标 Spring Data Elasticsearch 的
// ElasticsearchOperations/Template):常用操作封装 + 原生客户端暴露。
package es

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// Config ES 客户端配置。
type Config struct {
	// Addresses 节点地址列表(如 http://127.0.0.1:9200)。
	Addresses []string
	// Username/Password 认证(可选)。
	Username string
	Password string
	// Timeout 请求超时(默认 10s)。
	Timeout time.Duration
	// CloudID 弹性云 ID(可选,优先于 Addresses)。
	CloudID string
	// APIKey API Key 认证(可选)。
	APIKey string
}

// normalize 补齐默认值。
func (c Config) normalize() Config {
	if len(c.Addresses) == 0 {
		c.Addresses = []string{"http://127.0.0.1:9200"}
	}
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	return c
}

// EsTemplate 高频操作层:索引/查询/获取/删除/批量 + 原生客户端。
type EsTemplate interface {
	// Ping 健康检查。
	Ping(ctx context.Context) error
	// Index 写入文档(指定 ID;存在即覆盖)。
	Index(ctx context.Context, index, id string, document interface{}) error
	// IndexWithRefresh 写入文档并强制 refresh(测试/低延迟场景)。
	IndexWithRefresh(ctx context.Context, index, id string, document interface{}) error
	// Get 按 ID 获取文档原始 JSON。
	Get(ctx context.Context, index, id string) ([]byte, error)
	// Delete 按 ID 删除文档。
	Delete(ctx context.Context, index, id string) error
	// Search 执行查询(DSL 原始字节);返回解析后的搜索响应。
	Search(ctx context.Context, index string, query []byte) (*SearchResponse, error)
	// ExistsIndex 判断索引是否存在。
	ExistsIndex(ctx context.Context, index string) (bool, error)
	// CreateIndex 创建索引(带 settings/mappings JSON;已存在时返回 false 不报错)。
	CreateIndex(ctx context.Context, index string, settingsJSON []byte) (bool, error)
	// Client 返回底层原生客户端(高级操作入口)。
	Client() *elasticsearch.Client
}

// Client 是 ES 模板实现。
type Client struct {
	es *elasticsearch.Client
}

// NewClient 创建 ES 客户端并验证连通性。
func NewClient(config Config) (*Client, error) {
	cfg := config.normalize()
	esConfig := elasticsearch.Config{
		Addresses: cfg.Addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
		CloudID:   cfg.CloudID,
		APIKey:    cfg.APIKey,
	}
	esClient, err := elasticsearch.NewClient(esConfig)
	if err != nil {
		return nil, fmt.Errorf("es: create client: %w", err)
	}
	client := &Client{es: esClient}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		return nil, fmt.Errorf("es: ping %v: %w", cfg.Addresses, err)
	}
	SetGlobal(client)
	return client, nil
}

// Ping 健康检查。
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.es == nil {
		return errors.New("es: client is nil")
	}
	response, err := c.es.Ping(c.es.Ping.WithContext(ctx))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.IsError() {
		return fmt.Errorf("es: ping failed: %s", response.String())
	}
	return nil
}

// Index 写入文档。
func (c *Client) Index(ctx context.Context, index, id string, document interface{}) error {
	body, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("es: marshal document: %w", err)
	}
	request := esapi.IndexRequest{
		Index:      index,
		DocumentID: id,
		Body:       bytes.NewReader(body),
		Refresh:    "false",
	}
	return c.do(ctx, request)
}

// IndexWithRefresh 写入并强制 refresh。
func (c *Client) IndexWithRefresh(ctx context.Context, index, id string, document interface{}) error {
	body, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("es: marshal document: %w", err)
	}
	request := esapi.IndexRequest{
		Index:      index,
		DocumentID: id,
		Body:       bytes.NewReader(body),
		Refresh:    "true",
	}
	return c.do(ctx, request)
}

// Get 按 ID 获取文档原始 JSON。
func (c *Client) Get(ctx context.Context, index, id string) ([]byte, error) {
	if c == nil || c.es == nil {
		return nil, errors.New("es: client is nil")
	}
	response, err := c.es.Get(index, id, c.es.Get.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.IsError() {
		if response.StatusCode == 404 {
			return nil, nil // 不存在返回 nil,nil
		}
		return nil, fmt.Errorf("es: get %s/%s: %s", index, id, response.String())
	}
	return io.ReadAll(response.Body)
}

// Delete 删除文档。
func (c *Client) Delete(ctx context.Context, index, id string) error {
	request := esapi.DeleteRequest{
		Index:      index,
		DocumentID: id,
	}
	return c.do(ctx, request)
}

// Search 执行查询。
func (c *Client) Search(ctx context.Context, index string, query []byte) (*SearchResponse, error) {
	if c == nil || c.es == nil {
		return nil, errors.New("es: client is nil")
	}
	response, err := c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex(index),
		c.es.Search.WithBody(bytes.NewReader(query)),
	)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.IsError() {
		return nil, fmt.Errorf("es: search %s: %s", index, response.String())
	}
	var result SearchResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("es: decode search response: %w", err)
	}
	return &result, nil
}

// ExistsIndex 索引是否存在。
func (c *Client) ExistsIndex(ctx context.Context, index string) (bool, error) {
	if c == nil || c.es == nil {
		return false, errors.New("es: client is nil")
	}
	response, err := c.es.Indices.Exists([]string{index}, c.es.Indices.Exists.WithContext(ctx))
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	return response.StatusCode == 200, nil
}

// CreateIndex 创建索引;已存在返回 (false, nil)。
func (c *Client) CreateIndex(ctx context.Context, index string, settingsJSON []byte) (bool, error) {
	if c == nil || c.es == nil {
		return false, errors.New("es: client is nil")
	}
	response, err := c.es.Indices.Create(
		index,
		c.es.Indices.Create.WithContext(ctx),
		c.es.Indices.Create.WithBody(bytes.NewReader(settingsJSON)),
	)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode == 400 && strings.Contains(response.String(), "resource_already_exists_exception") {
		return false, nil
	}
	if response.IsError() {
		return false, fmt.Errorf("es: create index %s: %s", index, response.String())
	}
	return true, nil
}

// Client 返回原生客户端。
func (c *Client) Client() *elasticsearch.Client {
	if c == nil {
		return nil
	}
	return c.es
}

// do 执行单请求并统一错误处理。
func (c *Client) do(ctx context.Context, request esapi.Request) error {
	if c == nil || c.es == nil {
		return errors.New("es: client is nil")
	}
	response, err := request.Do(ctx, c.es)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.IsError() {
		return fmt.Errorf("es: %s", response.String())
	}
	return nil
}

// ---- 响应结构(常用子集) ----

// SearchResponse 搜索响应(常用字段)。
type SearchResponse struct {
	Took     int `json:"took"`
	TimedOut bool `json:"timed_out"`
	Hits     struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
		Hits []Hit `json:"hits"`
	} `json:"hits"`
}

// Hit 单条命中。
type Hit struct {
	Index  string          `json:"_index"`
	ID     string          `json:"_id"`
	Score  float64         `json:"_score"`
	Source json.RawMessage `json:"_source"`
}
