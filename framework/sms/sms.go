// Package sms 提供短信服务集成,对标阿里云短信 SendSms API:
// 零第三方依赖,自实现阿里云 RPC 签名(HMAC-SHA1),适配标准接口语义。
package sms

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// 阿里云短信接口常量(对齐 dysmsapi.aliyuncs.com 2017-05-25)。
const (
	// DefaultEndpoint 阿里云短信服务地址。
	DefaultEndpoint = "https://dysmsapi.aliyuncs.com"
	// APIVersion 接口版本。
	APIVersion = "2017-05-25"
	// ActionSendSms 发送短信动作。
	ActionSendSms = "SendSms"
)

// Config 短信服务配置(阿里云 AccessKey)。
type Config struct {
	// AccessKeyID 阿里云 AccessKey ID。
	AccessKeyID string
	// AccessKeySecret 阿里云 AccessKey Secret。
	AccessKeySecret string
	// SignName 短信签名(如「公司名」)。
	SignName string
	// TemplateCode 短信模板 Code(如 SMS_123456789)。
	TemplateCode string
	// Endpoint 服务地址;空时使用默认。
	Endpoint string
	// Timeout 请求超时(默认 10s)。
	Timeout time.Duration
}

// normalize 补齐默认值。
func (c Config) normalize() Config {
	if c.Endpoint == "" {
		c.Endpoint = DefaultEndpoint
	}
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	return c
}

// SendRequest 发送短信请求参数(对齐阿里云 SendSms)。
type SendRequest struct {
	// PhoneNumbers 接收号码(单发一个;批量用逗号分隔多个,最多 1000)。
	PhoneNumbers string
	// SignName 签名(空时用 Config.SignName)。
	SignName string
	// TemplateCode 模板 Code(空时用 Config.TemplateCode)。
	TemplateCode string
	// TemplateParam 模板变量(如 {"code":"123456"});无变量可为 nil。
	TemplateParam map[string]string
	// OutId 外部流水号(回调定位用,可选)。
	OutId string
}

// SendResponse 发送结果(对齐阿里云返回)。
type SendResponse struct {
	// Code 结果码(OK 为成功;失败为阿里云错误码,如 isv.MOBILE_NUMBER_ILLEGAL)。
	Code string `json:"Code"`
	// Message 描述。
	Message string `json:"Message"`
	// RequestID 请求 ID(排查用)。
	RequestID string `json:"RequestId"`
	// BizID 发送回执 ID(查询状态用)。
	BizID string `json:"BizId"`
}

// IsSuccess 是否发送成功。
func (r *SendResponse) IsSuccess() bool {
	return r != nil && r.Code == "OK"
}

// Client 短信客户端。
type Client struct {
	config Config
	client *http.Client
}

// NewClient 创建短信客户端。
func NewClient(config Config) (*Client, error) {
	if config.AccessKeyID == "" || config.AccessKeySecret == "" {
		return nil, errors.New("sms: access key id and secret are required")
	}
	cfg := config.normalize()
	client := &Client{
		config: cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
	SetGlobal(client)
	return client, nil
}

// Send 发送短信(对标阿里云 SendSms)。
func (c *Client) Send(ctx context.Context, request SendRequest) (*SendResponse, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("sms: client is nil")
	}
	if request.PhoneNumbers == "" {
		return nil, errors.New("sms: phone numbers are required")
	}
	signName := request.SignName
	if signName == "" {
		signName = c.config.SignName
	}
	templateCode := request.TemplateCode
	if templateCode == "" {
		templateCode = c.config.TemplateCode
	}
	if signName == "" || templateCode == "" {
		return nil, errors.New("sms: sign name and template code are required")
	}

	params := map[string]string{
		"AccessKeyId":      c.config.AccessKeyID,
		"Action":           ActionSendSms,
		"Format":           "JSON",
		"PhoneNumbers":     request.PhoneNumbers,
		"RegionId":         "cn-hangzhou",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   newNonce(),
		"SignatureVersion": "1.0",
		"SignName":         signName,
		"TemplateCode":     templateCode,
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          APIVersion,
	}
	if len(request.TemplateParam) > 0 {
		paramJSON, err := json.Marshal(request.TemplateParam)
		if err != nil {
			return nil, fmt.Errorf("sms: marshal template param: %w", err)
		}
		params["TemplateParam"] = string(paramJSON)
	}
	if request.OutId != "" {
		params["OutId"] = request.OutId
	}

	// RPC 签名
	params["Signature"] = signRPC(c.config.AccessKeySecret, params)

	requestURL := c.config.Endpoint + "?" + buildQuery(params)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("sms: build request: %w", err)
	}
	response, err := c.client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("sms: send request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("sms: read response: %w", err)
	}
	var result SendResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("sms: decode response %d: %w", response.StatusCode, err)
	}
	return &result, nil
}

// signRPC 阿里云 RPC 签名:
// 参数按 Key 字典序排序,PercentEncode 拼接,HMAC-SHA1(Secret+"&")签名后 Base64。
func signRPC(accessKeySecret string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString("&")
		builder.WriteString(percentEncode(key))
		builder.WriteString("=")
		builder.WriteString(percentEncode(params[key]))
	}
	stringToSign := "GET&%2F&" + percentEncode(builder.String()[1:])
	mac := hmac.New(sha1.New, []byte(accessKeySecret+"&"))
	_, _ = mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// percentEncode 阿里云 PercentEncode 规则。
func percentEncode(value string) string {
	encoded := url.QueryEscape(value)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

// buildQuery 拼接查询串(已签名,按参数顺序输出)。
func buildQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+url.QueryEscape(params[key]))
	}
	return strings.Join(parts, "&")
}

// newNonce 生成签名随机串(时间戳+随机数)。
func newNonce() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), randInt())
}

// randInt 轻量随机(非密码学用途)。
func randInt() int64 {
	return time.Now().UnixNano() % 1000000
}
