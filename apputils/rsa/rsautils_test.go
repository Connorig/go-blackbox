package rsa

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"testing"
)

// TestLoadPrivateKeySafeRejectsInvalidPEM 验证无效 PEM 返回错误而不是 panic。
func TestLoadPrivateKeySafeRejectsInvalidPEM(t *testing.T) {
	if _, err := LoadPrivateKeySafe([]byte("not-a-pem")); err == nil {
		t.Fatal("invalid private key PEM must return an error")
	}
	if _, err := LoadPublicKeySafe([]byte("not-a-pem")); err == nil {
		t.Fatal("invalid public key PEM must return an error")
	}
}

// TestLegacyLoadersReturnNilOnInvalidPEM 验证旧版加载器在失败时返回 nil 而不是 panic。
func TestLegacyLoadersReturnNilOnInvalidPEM(t *testing.T) {
	if LoadPrivateKey([]byte("invalid")) != nil {
		t.Fatal("legacy LoadPrivateKey must return nil for invalid PEM")
	}
	if LoadPublicKey([]byte("invalid")) != nil {
		t.Fatal("legacy LoadPublicKey must return nil for invalid PEM")
	}
	if GetPublicKeyFromPriKey([]byte("invalid")) != nil {
		t.Fatal("GetPublicKeyFromPriKey must return nil for invalid private key")
	}
}

// TestGenerateEncryptDecryptRoundTrip 验证密钥生成、PEM 导出加载与加解密往返。
func TestGenerateEncryptDecryptRoundTrip(t *testing.T) {
	privateKey, publicKey := GenerateRSAKey()
	if privateKey == nil || publicKey == nil {
		t.Fatal("generate RSA key pair failed")
	}

	privatePEM := ExportPrivateKeyAsPEM(privateKey)
	if len(privatePEM) == 0 {
		t.Fatal("export private key PEM failed")
	}
	loadedPrivate, err := LoadPrivateKeySafe(privatePEM)
	if err != nil {
		t.Fatalf("load private key failed: %v", err)
	}

	// 使用 Base64 编码的私钥走 DecodingByPrivateKey 路径
	encoded := Base64EncodeString(privatePEM)
	plaintext := []byte("secret-message")
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, publicKey, plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	decrypted, err := DecodingByPrivateKey(encoded, ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("round trip mismatch: got %q", decrypted)
	}
	if loadedPrivate == nil {
		t.Fatal("loaded private key is nil")
	}
}
