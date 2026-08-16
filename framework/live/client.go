package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client SRS HTTP API 客户端。
type Client struct {
	baseURL string
	httpc   *http.Client
}

// NewClient 创建 SRS API 客户端。
// baseURL 如 http://127.0.0.1:1985;timeout 默认 5s。
func NewClient(baseURL string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Client{
		baseURL: trimSlash(baseURL),
		httpc:   &http.Client{Timeout: timeout},
	}
}

// Version SRS 版本信息。
type Version struct {
	Code       int    `json:"code"`
	ServerID   string `json:"server_id"`
	ServiceID  string `json:"service_id"`
	PID        string `json:"pid"`
	Major      int    `json:"major"`
	Minor      int    `json:"minor"`
	Revision   int    `json:"revision"`
	Version    string `json:"version"`
}

// Stream 在线流信息(字段名直观化)。
type Stream struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	App        string `json:"app"`
	Vhost      string `json:"vhost"`
	URL        string `json:"url"`
	PublishCID string `json:"publish_cid"`  // 推流连接 ID(踢流关键)
	VideoCodec string `json:"video_codec"`  // 如 H264
	AudioCodec string `json:"audio_codec"`  // 如 AAC
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

// ClientInfo 客户端连接信息。
type ClientInfo struct {
	ID       string `json:"id"`
	Vhost    string `json:"vhost"`
	Stream   string `json:"stream"`
	App      string `json:"app"`
	IP       string `json:"ip"`
	PageURL  string `json:"page_url"`
	Type     string `json:"type"`   // publisher / player
	Alive    string `json:"alive"`  // 秒
	Publish  bool   `json:"publish"`
}

// Version 查询 SRS 版本(GET /api/v1/versions)。
func (c *Client) Version(ctx context.Context) (*Version, error) {
	var result struct {
		Code int     `json:"code"`
		Data Version `json:"data"`
	}
	if err := c.getJSON(ctx, "/api/v1/versions", &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// ListStreams 查询在线流(GET /api/v1/streams/,必须带尾部斜杠)。
func (c *Client) ListStreams(ctx context.Context) ([]*Stream, error) {
	var result struct {
		Code  int            `json:"code"`
		Streams []*srsStream `json:"streams"`
	}
	if err := c.getJSON(ctx, "/api/v1/streams/", &result); err != nil {
		return nil, err
	}
	streams := make([]*Stream, 0, len(result.Streams))
	for _, s := range result.Streams {
		stream := &Stream{
			ID:    s.ID,
			Name:  s.Name,
			App:   s.App,
			Vhost: s.Vhost,
			URL:   s.URL,
		}
		if s.Publish != nil {
			stream.PublishCID = s.Publish.CID
			if s.Publish.Video != nil {
				stream.VideoCodec = s.Publish.Video.Codec
				stream.Width = s.Publish.Video.Width
				stream.Height = s.Publish.Video.Height
			}
			if s.Publish.Audio != nil {
				stream.AudioCodec = s.Publish.Audio.Codec
			}
		}
		streams = append(streams, stream)
	}
	return streams, nil
}

// srsStream SRS 原始流结构(内部解析用)。
type srsStream struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	App     string `json:"app"`
	Vhost   string `json:"vhost"`
	URL     string `json:"url"`
	Publish *struct {
		CID   string `json:"cid"`
		Video *struct {
			Codec  string `json:"codec"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"video"`
		Audio *struct {
			Codec string `json:"codec"`
		} `json:"audio"`
	} `json:"publish"`
}

// KickStream 踢流(两步:①查流名取 publish_cid ②DELETE 客户端)。
// 流不存在返回明确错误;cid 缺失返回提示(流刚断/无活跃推流)。
func (c *Client) KickStream(ctx context.Context, streamName string) error {
	streams, err := c.ListStreams(ctx)
	if err != nil {
		return err
	}
	for _, stream := range streams {
		if stream.Name == streamName {
			if stream.PublishCID == "" {
				return fmt.Errorf("live: stream %q has no active publish cid", streamName)
			}
			return c.KickClient(ctx, stream.PublishCID)
		}
	}
	return fmt.Errorf("live: stream %q not found", streamName)
}

// KickClient 踢客户端(DELETE /api/v1/clients/{cid},不带尾部斜杠)。
func (c *Client) KickClient(ctx context.Context, cid string) error {
	if cid == "" {
		return errors.New("live: client id is required")
	}
	return c.deleteJSON(ctx, "/api/v1/clients/"+cid)
}

// ListClients 查询客户端连接(GET /api/v1/clients/,带尾部斜杠)。
func (c *Client) ListClients(ctx context.Context) ([]*ClientInfo, error) {
	var result struct {
		Code    int          `json:"code"`
		Clients []*ClientInfo `json:"clients"`
	}
	if err := c.getJSON(ctx, "/api/v1/clients/", &result); err != nil {
		return nil, err
	}
	return result.Clients, nil
}

// getJSON GET 请求 + JSON 解析(非 JSON 响应容错为结构化错误,不 panic)。
func (c *Client) getJSON(ctx context.Context, path string, out interface{}) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("live: build request: %w", err)
	}
	response, err := c.httpc.Do(request)
	if err != nil {
		return fmt.Errorf("live: %s: %w", path, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("live: read response: %w", err)
	}
	if response.StatusCode >= 300 {
		return fmt.Errorf("live: %s status %d: %s", path, response.StatusCode, truncateText(body, 200))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("live: %s invalid json: %w", path, err)
	}
	return nil
}

// deleteJSON DELETE 请求(SRS 删除客户端返回 code 0)。
func (c *Client) deleteJSON(ctx context.Context, path string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("live: build request: %w", err)
	}
	response, err := c.httpc.Do(request)
	if err != nil {
		return fmt.Errorf("live: %s: %w", path, err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode >= 300 {
		return fmt.Errorf("live: %s status %d: %s", path, response.StatusCode, truncateText(body, 200))
	}
	var result struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(body, &result); err == nil && result.Code != 0 {
		return fmt.Errorf("live: %s failed code=%d", path, result.Code)
	}
	return nil
}

// trimSlash 去尾部斜杠。
func trimSlash(value string) string {
	for len(value) > 0 && value[len(value)-1] == '/' {
		value = value[:len(value)-1]
	}
	return value
}

// truncateText 截断文本。
func truncateText(data []byte, limit int) string {
	text := string(data)
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
}

// 全局便捷入口:NewClient 后业务手动 SetGlobal(或 Provide 自动)。

var global *Client

// SetGlobal 设置全局客户端。
func SetGlobal(client *Client) { global = client }

// Get 获取全局 SRS 客户端;未初始化返回 nil。
func Get() *Client { return global }
