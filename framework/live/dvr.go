package live

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// 录制(DVR)客户端封装:启动/停止录制、查询录制文件、触发回放。
// SRS DVR HTTP API 约定(自建 hook 之外的轻量接口,需 SRS 配置 http_hooks on_dvr
// 或 vhost 开启 dvr 后使用)。

// StartRecord 启动录制(stream 为流名,output 为录制文件路径)。
// 返回任务 ID(空表示 SRS 未返回)。
func (c *Client) StartRecord(ctx context.Context, stream, output string) (string, error) {
	params := url.Values{}
	params.Set("stream", stream)
	params.Set("output", output)
	body, err := c.postForm(ctx, "/api/v1/dvr/start", params)
	if err != nil {
		return "", fmt.Errorf("live: start record: %w", err)
	}
	var result struct {
		Code int    `json:"code"`
		Data string `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("live: start record: parse response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("live: start record: srs code %d", result.Code)
	}
	return result.Data, nil
}

// StopRecord 停止录制。
func (c *Client) StopRecord(ctx context.Context, stream string) error {
	params := url.Values{}
	params.Set("stream", stream)
	body, err := c.postForm(ctx, "/api/v1/dvr/stop", params)
	if err != nil {
		return fmt.Errorf("live: stop record: %w", err)
	}
	var result struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("live: stop record: parse response: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("live: stop record: srs code %d", result.Code)
	}
	return nil
}

// postForm 内部:POST form 到 SRS API。
func (c *Client) postForm(ctx context.Context, path string, params url.Values) ([]byte, error) {
	if c == nil || c.httpc == nil {
		return nil, fmt.Errorf("live: client is nil")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.httpc.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	return readBodyLimited(response.Body, 1<<20)
}

// readBodyLimited 读取响应体(限制大小)。
func readBodyLimited(reader io.Reader, limit int64) ([]byte, error) {
	buffer := make([]byte, 0, 4096)
	chunk := make([]byte, 4096)
	var total int64
	for {
		n, err := reader.Read(chunk)
		if n > 0 {
			total += int64(n)
			if total > limit {
				return nil, fmt.Errorf("response too large (>%d)", limit)
			}
			buffer = append(buffer, chunk[:n]...)
		}
		if err == io.EOF {
			return buffer, nil
		}
		if err != nil {
			return nil, err
		}
	}
}
