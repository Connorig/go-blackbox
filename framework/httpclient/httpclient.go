// Package httpclient 提供通用 HTTP 请求工具(零依赖 net/http):
// GET/POST/PUT/DELETE,支持 query 参数、表单、JSON、原始 body、自定义 header。
// 与 framework/thirdparty 的区别:thirdparty 面向签名对接,本包面向通用 HTTP 调用。
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config 客户端配置。
type Config struct {
	// BaseURL 基础地址(可选,path 为绝对 URL 时忽略)。
	BaseURL string
	// Timeout 请求超时(默认 10s)。
	Timeout time.Duration
	// DefaultHeaders 默认请求头(每个请求都带,可被 Options.Headers 覆盖)。
	DefaultHeaders map[string]string
	// MaxRetries 网络错误重试次数(默认 0 不重试)。
	MaxRetries int
}

// Options 单次请求选项。
type Options struct {
	// Query 查询参数(URL 编码拼接)。
	Query map[string]string
	// Headers 请求头(合并 DefaultHeaders,同名覆盖)。
	Headers map[string]string
	// Form 表单参数(application/x-www-form-urlencoded)。
	Form map[string]string
	// JSON 对象(application/json 序列化)。
	JSON interface{}
	// Body 原始请求体(与 ContentType 配合;优先级高于 Form/JSON)。
	Body []byte
	// ContentType 请求体类型(如 text/plain、application/xml)。
	ContentType string
}

// Response 响应。
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// Text 响应体文本。
func (r *Response) Text() string {
	if r == nil {
		return ""
	}
	return string(r.Body)
}

// Unmarshal 响应体解析为 JSON(目标对象)。
func (r *Response) Unmarshal(out interface{}) error {
	if r == nil {
		return errors.New("httpclient: response is nil")
	}
	return json.Unmarshal(r.Body, out)
}

// IsSuccess 2xx 判断。
func (r *Response) IsSuccess() bool {
	return r != nil && r.StatusCode >= 200 && r.StatusCode < 300
}

// Client HTTP 客户端(并发安全)。
type Client struct {
	baseURL        string
	httpClient     *http.Client
	defaultHeaders map[string]string
	maxRetries     int
}

// New 创建客户端。
func New(config Config) *Client {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		baseURL:        strings.TrimRight(config.BaseURL, "/"),
		httpClient:     &http.Client{Timeout: timeout},
		defaultHeaders: config.DefaultHeaders,
		maxRetries:     config.MaxRetries,
	}
}

// Get GET 请求;out 非 nil 时自动 JSON 解码到 out。
func (c *Client) Get(ctx context.Context, path string, options Options, out interface{}) (*Response, error) {
	return c.do(ctx, http.MethodGet, path, options, out)
}

// Post POST 请求。
func (c *Client) Post(ctx context.Context, path string, options Options, out interface{}) (*Response, error) {
	return c.do(ctx, http.MethodPost, path, options, out)
}

// Put PUT 请求。
func (c *Client) Put(ctx context.Context, path string, options Options, out interface{}) (*Response, error) {
	return c.do(ctx, http.MethodPut, path, options, out)
}

// Delete DELETE 请求。
func (c *Client) Delete(ctx context.Context, path string, options Options, out interface{}) (*Response, error) {
	return c.do(ctx, http.MethodDelete, path, options, out)
}

// do 执行请求(含重试与响应解析)。
func (c *Client) do(ctx context.Context, method, path string, options Options, out interface{}) (*Response, error) {
	if c == nil {
		return nil, errors.New("httpclient: client is nil")
	}
	fullURL := c.baseURL + path
	if len(options.Query) > 0 {
		fullURL += "?" + encodeQuery(options.Query)
	}

	var bodyBytes []byte
	var contentType string
	switch {
	case options.Body != nil:
		bodyBytes = options.Body
		contentType = options.ContentType
	case options.Form != nil:
		bodyBytes = []byte(encodeForm(options.Form))
		contentType = "application/x-www-form-urlencoded"
	case options.JSON != nil:
		data, err := json.Marshal(options.JSON)
		if err != nil {
			return nil, fmt.Errorf("httpclient: marshal json: %w", err)
		}
		bodyBytes = data
		contentType = "application/json"
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 300 * time.Millisecond):
			}
		}
		request, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("httpclient: build request: %w", err)
		}
		for key, value := range c.defaultHeaders {
			request.Header.Set(key, value)
		}
		for key, value := range options.Headers {
			request.Header.Set(key, value)
		}
		if contentType != "" {
			request.Header.Set("Content-Type", contentType)
		}

		response, err := c.httpClient.Do(request)
		if err != nil {
			lastErr = err
			continue
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
		if err != nil {
			return nil, fmt.Errorf("httpclient: read response: %w", err)
		}
		result := &Response{StatusCode: response.StatusCode, Headers: response.Header, Body: body}
		if out != nil && len(body) > 0 {
			if err := json.Unmarshal(body, out); err != nil {
				return result, fmt.Errorf("httpclient: unmarshal response: %w", err)
			}
		}
		return result, nil
	}
	return nil, fmt.Errorf("httpclient: %s %s failed: %w", method, fullURL, lastErr)
}

// encodeQuery 查询参数编码(键排序稳定)。
func encodeQuery(query map[string]string) string {
	values := url.Values{}
	for key, value := range query {
		values.Set(key, value)
	}
	return values.Encode()
}

// encodeForm 表单编码。
func encodeForm(form map[string]string) string {
	values := url.Values{}
	for key, value := range form {
		values.Set(key, value)
	}
	return values.Encode()
}

// 全局便捷入口:New 成功后自动设置,业务直接 httpclient.Get() 获取。

var global *Client

// SetGlobal 设置全局客户端(New 成功后自动调用)。
func SetGlobal(client *Client) { global = client }

// Get 获取全局 HTTP 客户端;未初始化返回 nil。
func Get() *Client { return global }
