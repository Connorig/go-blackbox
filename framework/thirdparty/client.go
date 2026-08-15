package thirdparty

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	apperr "github.com/Connorig/go-blackbox/component/error"
)

// DefaultTimeout 默认总超时。
const DefaultTimeout = 10 * time.Second

// Config 第三方客户端配置。
type Config struct {
	// BaseURL 第三方服务根地址(如 https://sms.partner.com)。
	BaseURL string
	// Signer 签名器(NewHMACSigner / NewRSASignerFromPEM / NewBearerSigner)。
	Signer Signer
	// Timeout 单次请求总超时;非正数时使用 DefaultTimeout。
	Timeout time.Duration
	// MaxRetries 失败重试次数(默认 2;网络错误与 5xx 重试,4xx 不重试)。
	MaxRetries int
	// RetryBaseDelay 重试基础退避(默认 200ms,指数增长 + 抖动)。
	RetryBaseDelay time.Duration
	// ExtraHeaders 附加固定请求头(如 Accept-Language)。
	ExtraHeaders map[string]string
	// NonceFunc 自定义 nonce 生成;nil 时使用 crypto/rand 随机 hex。
	NonceFunc func() (string, error)
}

// Client 第三方 HTTP 客户端。
// 使用前必须 NewClient 构造;所有方法并发安全。
type Client struct {
	baseURL      string
	signer       Signer
	httpClient   *http.Client
	maxRetries   int
	retryBase    time.Duration
	extraHeaders map[string]string
	nonceFunc    func() (string, error)
}

// NewClient 构造第三方客户端。
func NewClient(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	retries := cfg.MaxRetries
	if retries < 0 {
		retries = 0
	} else if retries == 0 {
		retries = 2
	}
	retryBase := cfg.RetryBaseDelay
	if retryBase <= 0 {
		retryBase = 200 * time.Millisecond
	}
	nonceFunc := cfg.NonceFunc
	if nonceFunc == nil {
		nonceFunc = defaultNonce
	}
	return &Client{
		baseURL:      cfg.BaseURL,
		signer:       cfg.Signer,
		httpClient:   &http.Client{Timeout: timeout},
		maxRetries:   retries,
		retryBase:    retryBase,
		extraHeaders: cfg.ExtraHeaders,
		nonceFunc:    nonceFunc,
	}
}

// Get 发起 GET 请求,响应 JSON 解码到 out。
// query 为查询参数(可为 nil);out 为 nil 时只校验状态码。
func (c *Client) Get(ctx context.Context, path string, query map[string]string, out interface{}) error {
	return c.Do(ctx, http.MethodGet, path, query, nil, out)
}

// Post 发起 POST 请求,body 序列化为 JSON,响应解码到 out。
func (c *Client) Post(ctx context.Context, path string, body, out interface{}) error {
	return c.Do(ctx, http.MethodPost, path, nil, body, out)
}

// Do 发起任意方法请求;query 为查询参数,body 为 JSON 负载(可为 nil)。
func (c *Client) Do(ctx context.Context, method, path string, query map[string]string, body, out interface{}) error {
	if c.baseURL == "" {
		return apperr.New(apperr.CodeThirdPartyError, "thirdparty: base url is empty")
	}
	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return apperr.Wrap(err, apperr.CodeThirdPartyError, "thirdparty: marshal request body")
		}
	}

	url := c.baseURL + path
	if len(query) > 0 {
		url += "?" + buildQuery(query)
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return apperr.Wrap(ctx.Err(), apperr.CodeThirdPartyTimeout, "thirdparty: request cancelled")
			case <-time.After(backoff(c.retryBase, attempt)):
			}
		}
		status, respBody, err := c.roundTrip(ctx, method, url, bodyBytes)
		if err != nil {
			lastErr = err
			// 网络层错误可重试
			continue
		}
		if status >= 200 && status < 300 {
			if out != nil && len(respBody) > 0 {
				if err := json.Unmarshal(respBody, out); err != nil {
					return apperr.Wrap(err, apperr.CodeThirdPartyError, "thirdparty: unmarshal response")
				}
			}
			return nil
		}
		lastErr = apperr.NewWithStatus(status, apperr.CodeThirdPartyError,
			fmt.Sprintf("thirdparty: unexpected status %d: %s", status, truncate(respBody, 200)))
		// 4xx 不重试(客户端错误,重试无意义)
		if status >= 400 && status < 500 {
			return lastErr
		}
	}
	return lastErr
}

// roundTrip 执行单次请求并完成签名。
func (c *Client) roundTrip(ctx context.Context, method, url string, body []byte) (int, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("thirdparty: build request: %w", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Accept", "application/json")
	for key, value := range c.extraHeaders {
		request.Header.Set(key, value)
	}

	// 签名头(签名器非 nil 时)
	if c.signer != nil {
		nonce, err := c.nonceFunc()
		if err != nil {
			return 0, nil, fmt.Errorf("thirdparty: generate nonce: %w", err)
		}
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		bodySHA256 := sha256Hex(body)
		signature, err := c.signer.Sign(method, request.URL.Path, timestamp, nonce, bodySHA256)
		if err != nil {
			return 0, nil, fmt.Errorf("thirdparty: sign request: %w", err)
		}
		request.Header.Set("X-Timestamp", timestamp)
		request.Header.Set("X-Nonce", nonce)
		request.Header.Set("X-Body-SHA256", bodySHA256)
		if signature != "" {
			request.Header.Set(c.signer.HeaderName(), signature)
		}
		request.Header.Set("X-App-Key", c.signer.HeaderValue())
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("thirdparty: %w", err)
	}
	defer response.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20)) // 4MB 上限
	if err != nil {
		return 0, nil, fmt.Errorf("thirdparty: read response: %w", err)
	}
	return response.StatusCode, respBody, nil
}

// buildQuery 拼接查询参数(按 key 排序,保证签名可复现)。
func buildQuery(query map[string]string) string {
	if len(query) == 0 {
		return ""
	}
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	// 简单排序(签名规范要求参数有序)
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	result := ""
	for i, key := range keys {
		if i > 0 {
			result += "&"
		}
		result += key + "=" + query[key]
	}
	return result
}

// backoff 指数退避 + 抖动。
func backoff(base time.Duration, attempt int) time.Duration {
	delay := base * time.Duration(1<<(attempt-1))
	if delay > 5*time.Second {
		delay = 5 * time.Second
	}
	return delay
}

// sha256Hex 计算字节 SHA256 的 hex 表示。
func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// defaultNonce 生成 16 字节随机 hex。
func defaultNonce() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

// truncate 截断长文本用于错误信息。
func truncate(data []byte, limit int) string {
	text := string(data)
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
}

// IsTimeout 判断错误是否为第三方超时(可用于业务重试决策)。
func IsTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return appErr.Code == apperr.CodeThirdPartyTimeout
	}
	return false
}
