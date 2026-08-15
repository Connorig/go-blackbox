// Package thirdparty 提供对接第三方系统的出站 HTTP 客户端框架。
// 脚手架负责签名、超时、重试与错误码映射,业务只关心请求与响应。
package thirdparty

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	apprsa "github.com/Connorig/go-blackbox/component/auth/rsa"
)

// Signer 定义请求签名器。
// 签名串规范(与入站 openapi 网关对齐):
//
//	StringToSign = METHOD + "\n" + Path + "\n" + Timestamp + "\n" + Nonce + "\n" + BodySHA256
//
// 业务无需直接使用 Signer,Client 内部自动完成签名。
type Signer interface {
	// Sign 计算签名;method/path 为请求方法与路径(不含 query),
	// timestamp 为 Unix 秒字符串,nonce 为随机串,bodySHA256 为请求体 SHA256 hex。
	Sign(method, path, timestamp, nonce, bodySHA256 string) (string, error)
	// HeaderName 返回签名写入的请求头名称。
	HeaderName() string
	// HeaderValue 返回签名头附带的值(如 AppKey 或 Bearer token)。
	HeaderValue() string
}

// hmacSigner HMAC-SHA256 对称签名(AppKey + AppSecret)。
type hmacSigner struct {
	appKey    string
	appSecret string
}

// NewHMACSigner 创建 HMAC-SHA256 签名器。
func NewHMACSigner(appKey, appSecret string) Signer {
	return &hmacSigner{appKey: appKey, appSecret: appSecret}
}

func (s *hmacSigner) Sign(method, path, timestamp, nonce, bodySHA256 string) (string, error) {
	stringToSign := buildStringToSign(method, path, timestamp, nonce, bodySHA256)
	mac := hmac.New(sha256.New, []byte(s.appSecret))
	_, _ = mac.Write([]byte(stringToSign))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func (s *hmacSigner) HeaderName() string { return "X-Signature" }
func (s *hmacSigner) HeaderValue() string {
	return "HMAC " + s.appKey
}

// rsaSigner RSA-SHA256 非对称签名(私钥签名,第三方公钥验签)。
type rsaSigner struct {
	privateKey *rsa.PrivateKey
	appKey     string
}

// NewRSASignerFromPEM 从 PEM 私钥创建 RSA 签名器。
func NewRSASignerFromPEM(appKey string, privateKeyPEM []byte) (Signer, error) {
	privateKey, err := apprsa.LoadPrivateKeySafe(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("load rsa private key: %w", err)
	}
	return &rsaSigner{privateKey: privateKey, appKey: appKey}, nil
}

func (s *rsaSigner) Sign(method, path, timestamp, nonce, bodySHA256 string) (string, error) {
	stringToSign := buildStringToSign(method, path, timestamp, nonce, bodySHA256)
	digest := sha256.Sum256([]byte(stringToSign))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("rsa sign: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func (s *rsaSigner) HeaderName() string { return "X-Signature" }
func (s *rsaSigner) HeaderValue() string {
	return "RSA " + s.appKey
}

// bearerSigner 无签名模式(第三方平台自有的 Bearer token 鉴权,如 GitHub OAuth)。
type bearerSigner struct {
	token string
}

// NewBearerSigner 创建 Bearer token 签名器(仅写 Authorization 头,不计算签名)。
func NewBearerSigner(token string) Signer {
	return &bearerSigner{token: token}
}

func (s *bearerSigner) Sign(_, _, _, _, _ string) (string, error) { return "", nil }
func (s *bearerSigner) HeaderName() string                        { return "Authorization" }
func (s *bearerSigner) HeaderValue() string                       { return "Bearer " + s.token }

// buildStringToSign 规范化签名串。
func buildStringToSign(method, path, timestamp, nonce, bodySHA256 string) string {
	return method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + bodySHA256
}
