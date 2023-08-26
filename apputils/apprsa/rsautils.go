package apprsa

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"log"
)

const BITS = 2048 // 密钥长度 1024,2048

// GenerateRSAKey 生成 RSA 密钥对
func GenerateRSAKey() (*rsa.PrivateKey, *rsa.PublicKey) {
	privateKey, err := rsa.GenerateKey(rand.Reader, BITS)
	if err != nil {
		log.Printf("生成RSA密钥对失败 %s", err)
	}

	return privateKey, &privateKey.PublicKey
}

// DecodingByPrivateKey 私钥解密
func DecodingByPrivateKey(privateKey string, result []byte) (decodeStr []byte, err error) {
	// base64解码密钥
	decodeString, err1 := Base64DecodeString(privateKey)
	if err1 != nil {
		log.Printf("base decoding error %s", err1.Error())
	}

	// 通过pem加载密钥
	loadPrivateKey := LoadPrivateKey(decodeString)
	// rsa 解码
	decodeStr, err = rsa.DecryptPKCS1v15(rand.Reader, loadPrivateKey, result)
	if err != nil {
		log.Printf("encoding error %s", err.Error())
	}
	return
}

// ExportPublicKeyAsPEM 将 RSA 公钥导出为 PEM 格式
func ExportPublicKeyAsPEM(publicKey *rsa.PublicKey) []byte {
	pubBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {

		log.Printf("将 RSA 公钥导出为 PEM 格式失败")
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

// LoadPrivateKey 加载 RSA 私钥
func LoadPrivateKey(privPEM []byte) *rsa.PrivateKey {
	block, _ := pem.Decode(privPEM)
	if block == nil {
		//panic("failed to parse PEM block containing the key")
		log.Printf("解析 PEM block 私钥失败")
	}
	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		log.Printf("ParsePKCS1PrivateKey 私钥失败")
	}
	return privKey
}

// LoadPublicKey 加载 RSA 公钥
func LoadPublicKey(pubPEM []byte) *rsa.PublicKey {
	block, _ := pem.Decode(pubPEM)
	if block == nil {
		//panic("failed to parse PEM block containing the key")
		log.Printf("解析 PEM block 公钥失败")
	}
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		log.Printf("解析 ParsePKIXPublicKey 公钥失败")
	}
	return pubKey.(*rsa.PublicKey)
}

// GetPublicKeyFromPriKey 通过密钥获取公钥
func GetPublicKeyFromPriKey(privKey []byte) *rsa.PublicKey {
	privateKey := LoadPrivateKey(privKey)
	return &privateKey.PublicKey
}

// Base64EncodeString BASE64编码
func Base64EncodeString(pubPEM []byte) (basePubKey string) {
	basePubKey = base64.StdEncoding.EncodeToString([]byte(pubPEM))
	return
}

// Base64DecodeString BASE64解码
func Base64DecodeString(encode string) (pubKey []byte, err error) {
	pubKey, err = base64.StdEncoding.DecodeString(encode)
	return
}
