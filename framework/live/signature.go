package live

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/kataras/iris/v12"
)

// 回调签名验签:SRS 回调不可信来源防护。
// 方案:HMAC-SHA256 签名——SRS 配置 http_hooks 时带自定义 header,
// 业务在 gbx Provide 时注入共享密钥,gbx 在中间件层统一验签。
// 伪造回调(未带签名/签名不匹配)直接 403 拒绝,不进入业务 handler。

// CallbackSignature 回调签名中间件。
type CallbackSignature struct {
	secret    string
	headerKey string
}

// NewCallbackSignature 创建签名中间件。
// secret 为共享密钥;headerKey 为签名所在请求头(默认 X-SRS-Signature)。
func NewCallbackSignature(secret, headerKey string) *CallbackSignature {
	if headerKey == "" {
		headerKey = "X-SRS-Signature"
	}
	return &CallbackSignature{secret: secret, headerKey: headerKey}
}

// Wrap 返回 iris 中间件:验签失败直接 403,成功放行。
// 用法:app.Use(verify.Wrap()) 或 party.Use(verify.Wrap())。
func (s *CallbackSignature) Wrap() iris.Handler {
	return func(ctx iris.Context) {
		if !s.Verify(ctx) {
			ctx.StatusCode(iris.StatusForbidden)
			_, _ = ctx.WriteString(`{"code":1,"msg":"invalid signature"}`)
			return
		}
		ctx.Next()
	}
}

// Verify 校验当前请求签名。
func (s *CallbackSignature) Verify(ctx iris.Context) bool {
	if s == nil || s.secret == "" {
		return true // 未配置密钥:验签关闭(对接期友好)
	}
	if ctx == nil {
		return false
	}
	provided := ctx.GetHeader(s.headerKey)
	if provided == "" {
		return false
	}
	expected := s.Sign(ctx)
	return hmac.Equal([]byte(expected), []byte(provided))
}

// Sign 计算当前请求的 HMAC-SHA256 签名(hex)。
// 签名内容:方法 + 路径 + 原始 body(按字节)。
func (s *CallbackSignature) Sign(ctx iris.Context) string {
	body, _ := ctx.GetBody()
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte(strings.ToUpper(ctx.Method())))
	mac.Write([]byte(ctx.Path()))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// SignPayload 对任意 payload 计算签名(业务/SRS 侧工具函数)。
// 供测试与文档示例:与 Verify 的算法保持一致。
func SignPayload(secret, method, path string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.ToUpper(method)))
	mac.Write([]byte(path))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
