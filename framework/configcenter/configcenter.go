// Package configcenter 提供配置中心客户端(Nacos 风格 HTTP API):
// 拉取配置内容 + 轮询监听变更(回调)。
// 场景:配置热更新(灰度开关/业务参数不改代码不重启)。
// 说明:轻量实现(HTTP 拉取 + 定时轮询);长轮询与鉴权签名可在此基础上扩展。
package configcenter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client 配置中心客户端。
type Client struct {
	baseURL   string // 如 http://127.0.0.1:8848
	namespace string // 命名空间 ID(tenant,可空)
	group     string // 分组(默认 DEFAULT_GROUP)
	dataID    string
	httpc     *http.Client
}

// NewClient 创建客户端。
func NewClient(baseURL, dataID, group string) *Client {
	if group == "" {
		group = "DEFAULT_GROUP"
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		group:   group,
		dataID:  dataID,
		httpc:   &http.Client{Timeout: 5 * time.Second},
	}
}

// WithNamespace 设置命名空间(tenant)。
func (c *Client) WithNamespace(namespace string) *Client {
	if c != nil {
		c.namespace = namespace
	}
	return c
}

// Fetch 拉取配置内容(纯文本)。
func (c *Client) Fetch(ctx context.Context) (string, error) {
	if c == nil || c.baseURL == "" {
		return "", errors.New("configcenter: client is nil or base url empty")
	}
	if c.dataID == "" {
		return "", errors.New("configcenter: dataId is empty")
	}
	params := url.Values{}
	params.Set("dataId", c.dataID)
	params.Set("group", c.group)
	if c.namespace != "" {
		params.Set("tenant", c.namespace)
	}
	requestURL := c.baseURL + "/nacos/v1/cs/configs?" + params.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return "", err
	}
	response, err := c.httpc.Do(request)
	if err != nil {
		return "", fmt.Errorf("configcenter: fetch %q: %w", c.dataID, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("configcenter: config %q not found (dataId/group/namespace mismatch?)", c.dataID)
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("configcenter: fetch %q: http %d", c.dataID, response.StatusCode)
	}
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("configcenter: read response: %w", err)
	}
	return string(content), nil
}

// Watch 轮询监听配置变更:interval 为轮询间隔;onChange 收到新内容回调。
// 首次拉取即回调(初始化配置);之后内容变化才回调。
// 阻塞运行;ctx 取消后退出。
func (c *Client) Watch(ctx context.Context, interval time.Duration, onChange func(content string)) error {
	if onChange == nil {
		return errors.New("configcenter: onChange is nil")
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	last := ""
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	// 首次拉取立即回调
	content, err := c.Fetch(ctx)
	if err == nil {
		last = content
		onChange(content)
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			content, err := c.Fetch(ctx)
			if err != nil {
				continue // 网络抖动跳过,下轮重试
			}
			if content != last {
				last = content
				onChange(content)
			}
		}
	}
}
