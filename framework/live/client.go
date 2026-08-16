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
// SRS 5.x 的 video/audio 在流对象顶层,同时兼容旧版 publish.video/publish.audio。
type Stream struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	App        string `json:"app"`
	Vhost      string `json:"vhost"`
	URL        string `json:"url"`
	PublishCID string `json:"publish_cid"` // 推流连接 ID(踢流关键)
	VideoCodec string `json:"video_codec"` // 如 H264(平铺兼容字段)
	AudioCodec string `json:"audio_codec"` // 如 AAC(平铺兼容字段)
	Width      int    `json:"width"`       // 视频宽(平铺兼容字段)
	Height     int    `json:"height"`      // 视频高(平铺兼容字段)
	// Video 视频编码/分辨率详情(SRS 5.x 顶层 video 对象;无视频时为 nil)。
	Video *StreamVideo `json:"video,omitempty"`
	// Audio 音频编码详情(SRS 5.x 顶层 audio 对象;无音频时为 nil)。
	Audio *StreamAudio `json:"audio,omitempty"`
}

// StreamVideo 视频编码信息(json tag 对齐 SRS streams API)。
type StreamVideo struct {
	Codec   string `json:"codec"`   // 如 H264
	Profile string `json:"profile"` // 如 High
	Level   string `json:"level"`   // 如 3.1
	Width   int    `json:"width"`   // 如 1280
	Height  int    `json:"height"`  // 如 720
}

// StreamAudio 音频编码信息(json tag 对齐 SRS streams API)。
type StreamAudio struct {
	Codec      string `json:"codec"`       // 如 AAC
	SampleRate int    `json:"sample_rate"` // 如 44100
	Channel    int    `json:"channel"`     // 如 2
	Profile    string `json:"profile"`     // 如 LC
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
		// SRS 5.x 顶层 video/audio 优先;旧版 publish.video/publish.audio 兼容。
		if s.Video != nil {
			stream.Video = &StreamVideo{Codec: s.Video.Codec, Profile: s.Video.Profile, Level: s.Video.Level, Width: s.Video.Width, Height: s.Video.Height}
			stream.VideoCodec = s.Video.Codec
			stream.Width = s.Video.Width
			stream.Height = s.Video.Height
		}
		if s.Audio != nil {
			stream.Audio = &StreamAudio{Codec: s.Audio.Codec, SampleRate: s.Audio.SampleRate, Channel: s.Audio.Channel, Profile: s.Audio.Profile}
			stream.AudioCodec = s.Audio.Codec
		}
		if s.Publish != nil {
			stream.PublishCID = s.Publish.CID
			if stream.Video == nil && s.Publish.Video != nil {
				stream.Video = &StreamVideo{Codec: s.Publish.Video.Codec, Width: s.Publish.Video.Width, Height: s.Publish.Video.Height}
				stream.VideoCodec = s.Publish.Video.Codec
				stream.Width = s.Publish.Video.Width
				stream.Height = s.Publish.Video.Height
			}
			if stream.Audio == nil && s.Publish.Audio != nil {
				stream.Audio = &StreamAudio{Codec: s.Publish.Audio.Codec}
				stream.AudioCodec = s.Publish.Audio.Codec
			}
		}
		streams = append(streams, stream)
	}
	return streams, nil
}

// srsStream SRS 原始流结构(内部解析用)。
// 兼容 SRS 5.x(顶层 video/audio)与旧版(publish.video/publish.audio)。
type srsStream struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	App     string `json:"app"`
	Vhost   string `json:"vhost"`
	URL     string `json:"url"`
	Video   *struct {
		Codec   string `json:"codec"`
		Profile string `json:"profile"`
		Level   string `json:"level"`
		Width   int    `json:"width"`
		Height  int    `json:"height"`
	} `json:"video"`
	Audio *struct {
		Codec      string `json:"codec"`
		SampleRate int    `json:"sample_rate"`
		Channel    int    `json:"channel"`
		Profile    string `json:"profile"`
	} `json:"audio"`
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

// 全局便捷入口(默认/命名实例注册表,见 named.go):
// NewClient 后业务手动 SetGlobal(或 Provide 自动);多 SRS 集群用 SetNamed/GetNamed。
