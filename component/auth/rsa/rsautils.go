package rsa

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
)

const BITS = 2048 // 密钥长度 1024,2048

// GenerateRSAKey 生成 RSA 密钥对
// 生成失败时返回 nil,nil，调用方必须检查后再使用。
func GenerateRSAKey() (*rsa.PrivateKey, *rsa.PublicKey) {
	privateKey, err := rsa.GenerateKey(rand.Reader, BITS)
	if err != nil {
		log.Printf("生成RSA密钥对失败 %s", err)
		return nil, nil
	}
	return privateKey, &privateKey.PublicKey
}

// DecodingByPrivateKey 私钥解密
// privateKey 为 Base64 编码的 PEM 私钥；密钥格式或解密失败都会返回错误。
func DecodingByPrivateKey(privateKey string, result []byte) (decodeStr []byte, err error) {
	// base64解码密钥
	decodeString, err1 := Base64DecodeString(privateKey)
	if err1 != nil {
		return nil, fmt.Errorf("base64 decode private key: %w", err1)
	}
	// 通过pem加载密钥
	loadPrivateKey, err := LoadPrivateKeySafe(decodeString)
	if err != nil {
		return nil, err
	}
	// rsa 解码
	decodeStr, err = rsa.DecryptPKCS1v15(rand.Reader, loadPrivateKey, result)
	if err != nil {
		return nil, fmt.Errorf("rsa decrypt: %w", err)
	}
	return decodeStr, nil
}

// ExportPublicKeyAsPEM 将 RSA 公钥导出为 PEM 格式
func ExportPublicKeyAsPEM(publicKey *rsa.PublicKey) []byte {
	pubBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		log.Printf("将 RSA 公钥导出为 PEM 格式失败: %v", err)
		return nil
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	return pubPEM
}

// ExportPrivateKeyAsPEM 将 RSA 私钥导出为 PEM 格式
func ExportPrivateKeyAsPEM(privateKey *rsa.PrivateKey) []byte {
	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes})
	return privPEM
}

// LoadPrivateKey 加载 RSA 私钥；解析失败时返回 nil 并记录日志，不再 panic。
// 新代码建议使用 LoadPrivateKeySafe 获取具体错误。
func LoadPrivateKey(privPEM []byte) *rsa.PrivateKey {
	privateKey, err := LoadPrivateKeySafe(privPEM)
	if err != nil {
		log.Printf("加载 RSA 私钥失败: %v", err)
		return nil
	}
	return privateKey
}

// LoadPrivateKeySafe 加载 RSA 私钥并返回解析错误。
func LoadPrivateKeySafe(privPEM []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(privPEM)
	if block == nil {
		return nil, errors.New("解析 PEM block 私钥失败")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析 PKCS1 私钥失败: %w", err)
	}
	return privateKey, nil
}

// LoadPublicKey 加载 RSA 公钥；解析失败时返回 nil 并记录日志，不再 panic。
// 新代码建议使用 LoadPublicKeySafe 获取具体错误。
func LoadPublicKey(pubPEM []byte) *rsa.PublicKey {
	publicKey, err := LoadPublicKeySafe(pubPEM)
	if err != nil {
		log.Printf("加载 RSA 公钥失败: %v", err)
		return nil
	}
	return publicKey
}

// LoadPublicKeySafe 加载 RSA 公钥并返回解析错误。
func LoadPublicKeySafe(pubPEM []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pubPEM)
	if block == nil {
		return nil, errors.New("解析 PEM block 公钥失败")
	}
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析 PKIX 公钥失败: %w", err)
	}
	rsaKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("PEM 公钥不是 RSA 类型: %T", publicKey)
	}
	return rsaKey, nil
}

// GetPublicKeyFromPriKey 通过密钥获取公钥；私钥无效时返回 nil。
func GetPublicKeyFromPriKey(privKey []byte) *rsa.PublicKey {
	privateKey := LoadPrivateKey(privKey)
	if privateKey == nil {
		return nil
	}
	return &privateKey.PublicKey
}

// Base64EncodeString BASE64编码
func Base64EncodeString(pubPEM []byte) (basePubKey string) {
	return base64.StdEncoding.EncodeToString(pubPEM)
}

// Base64DecodeString BASE64解码
func Base64DecodeString(encode string) (pubKey []byte, err error) {
	return base64.StdEncoding.DecodeString(encode)
}
