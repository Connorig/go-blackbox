package rsa

import (
	"bytes"
	"testing"
)

// TestEncryptDecryptRoundTrip 公钥加密 → 私钥解密闭环。
func TestEncryptDecryptRoundTrip(t *testing.T) {
	keyPair, err := GenerateKeyPairPEM(2048)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	plaintext := []byte("hello sg-mes, hello gbx")

	ciphertext, err := EncryptByPublicKey(keyPair.PublicKeyPEM, plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext must differ")
	}
	decrypted, err := DecryptByPrivateKey(keyPair.PrivateKeyPEM, ciphertext)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("round trip mismatch: %q", decrypted)
	}
}

// TestDecodingByPrivateKeyCompat 兼容既有 DecodingByPrivateKey 路径。
func TestDecodingByPrivateKeyCompat(t *testing.T) {
	keyPair, _ := GenerateKeyPairPEM(2048)
	plaintext := []byte("compat check")
	ciphertext, err := EncryptByPublicKey(keyPair.PublicKeyPEM, plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	// DecodingByPrivateKey 期望 base64 编码的私钥(与 Base64EncodeString 配套)
	decrypted, err := DecodingByPrivateKey(Base64EncodeString(keyPair.PrivateKeyPEM), ciphertext)
	if err != nil {
		t.Fatalf("legacy decrypt failed: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatal("legacy decrypt mismatch")
	}
}

// TestEncryptInvalidKey 非法公钥报错。
func TestEncryptInvalidKey(t *testing.T) {
	if _, err := EncryptByPublicKey([]byte("invalid"), []byte("data")); err == nil {
		t.Fatal("invalid public key must fail")
	}
}

// TestGenerateKeyPairBits 位数校验。
func TestGenerateKeyPairBits(t *testing.T) {
	if _, err := GenerateKeyPairPEM(1024); err == nil {
		t.Fatal("bits < 2048 must fail")
	}
	keyPair, err := GenerateKeyPairPEM(2048)
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if len(keyPair.PrivateKeyPEM) == 0 || len(keyPair.PublicKeyPEM) == 0 {
		t.Fatal("PEM outputs must not be empty")
	}
}
