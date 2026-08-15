package openapi

import (
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// 签名请求头名称(与出站 thirdparty 客户端对齐)。
const (
	HeaderAppKey     = "X-App-Key"     // 应用标识
	HeaderTimestamp  = "X-Timestamp"   // Unix 秒
	HeaderNonce      = "X-Nonce"       // 随机串(防重放)
	HeaderSignature  = "X-Signature"   // 签名值
	HeaderBodySHA256 = "X-Body-SHA256" // 请求体 SHA256 hex
)

// StringToSign 构建规范化签名串(防篡改:方法/路径/时间戳/随机串/请求体摘要全部参与)。
// 与出站 thirdparty 的 buildStringToSign 保持一致。
func StringToSign(method, path, timestamp, nonce, bodySHA256 string) string {
	return method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + bodySHA256
}

// VerifySignature 按应用配置的算法校验签名。
// app 提供密钥/公钥;签名头值格式:
//
//	HMAC:  hex(HMAC-SHA256(appSecret, StringToSign))
//	RSA:   base64(RSA-SHA256(publicKey, StringToSign))
func VerifySignature(app *App, method, path, timestamp, nonce, bodySHA256, signature string) error {
	if app == nil {
		return errors.New("openapi: app is nil")
	}
	if signature == "" {
		return errors.New("openapi: signature header is empty")
	}
	algorithm := app.Algorithm
	if algorithm == "" {
		algorithm = AlgHMAC
	}
	stringToSign := StringToSign(method, path, timestamp, nonce, bodySHA256)

	switch algorithm {
	case AlgHMAC:
		if app.AppSecret == "" {
			return errors.New("openapi: app secret is empty")
		}
		mac := hmac.New(sha256.New, []byte(app.AppSecret))
		_, _ = mac.Write([]byte(stringToSign))
		expected := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected)) {
			return errors.New("openapi: hmac signature mismatch")
		}
		return nil
	case AlgRSA:
		if app.PublicKey == "" {
			return errors.New("openapi: app public key is empty")
		}
		publicKey, err := parseRSAPublicKey([]byte(app.PublicKey))
		if err != nil {
			return fmt.Errorf("openapi: parse public key: %w", err)
		}
		raw, err := base64.StdEncoding.DecodeString(signature)
		if err != nil {
			return errors.New("openapi: rsa signature is not valid base64")
		}
		digest := sha256.Sum256([]byte(stringToSign))
		if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], raw); err != nil {
			return errors.New("openapi: rsa signature mismatch")
		}
		return nil
	default:
		return fmt.Errorf("openapi: unsupported algorithm %q", algorithm)
	}
}

// BodySHA256 计算请求体 SHA256 hex(空体返回空串对应的摘要)。
func BodySHA256(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

// parseRSAPublicKey 解析 PEM 或裸 DER 格式的 RSA 公钥。
func parseRSAPublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block != nil {
		data = block.Bytes
	}
	// PKIX(最常见)
	if key, err := x509.ParsePKIXPublicKey(data); err == nil {
		if rsaKey, ok := key.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
		return nil, errors.New("openapi: public key is not RSA")
	}
	// PKCS1 旧格式
	if key, err := x509.ParsePKCS1PublicKey(data); err == nil {
		return key, nil
	}
	return nil, errors.New("openapi: unsupported public key format")
}
