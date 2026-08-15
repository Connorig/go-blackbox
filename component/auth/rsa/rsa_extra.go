package rsa

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// EncryptByPublicKey 使用 PEM 公钥加密数据(公钥加密,私钥解密)。
// 与 DecodingByPrivateKey 组成完整加解密闭环(来自 sg-mes-api 实战场景)。
// 使用 PKCS1v15 填充(兼容主流实现);超长数据请先分段或使用混合加密。
func EncryptByPublicKey(publicKeyPEM []byte, plaintext []byte) ([]byte, error) {
	publicKey, err := LoadPublicKeySafe(publicKeyPEM)
	if err != nil {
		return nil, err
	}
	return rsa.EncryptPKCS1v15(rand.Reader, publicKey, plaintext)
}

// DecryptByPrivateKey 使用 PEM 私钥解密数据(与 EncryptByPublicKey 配套)。
func DecryptByPrivateKey(privateKeyPEM []byte, ciphertext []byte) ([]byte, error) {
	privateKey, err := LoadPrivateKeySafe(privateKeyPEM)
	if err != nil {
		return nil, err
	}
	return rsa.DecryptPKCS1v15(rand.Reader, privateKey, ciphertext)
}

// KeyPair 密钥对(生成结果)。
type KeyPair struct {
	PrivateKeyPEM []byte // 私钥(PKCS1 PEM)
	PublicKeyPEM  []byte // 公钥(PKIX PEM)
}

// GenerateKeyPairPEM 生成 RSA 密钥对(PEM 格式,一键产出)。
// bits 建议 2048/4096;返回的私钥/公钥可直接存储与分发。
// 对标 sg-mes-api 的 generatedKey,但纯内存生成、不落盘,更安全。
func GenerateKeyPairPEM(bits int) (*KeyPair, error) {
	if bits < 2048 {
		return nil, errors.New("rsa: bits must be at least 2048")
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, fmt.Errorf("rsa: generate key: %w", err)
	}
	publicKeyPEM := ExportPublicKeyAsPEM(&privateKey.PublicKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	return &KeyPair{PrivateKeyPEM: privateKeyPEM, PublicKeyPEM: publicKeyPEM}, nil
}
