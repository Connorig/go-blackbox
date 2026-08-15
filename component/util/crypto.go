package util

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"

	sid "github.com/Connorig/go-blackbox/framework/database/id"
)

// 加密工具:MD5/SHA1/SHA256 摘要 + 随机串 + UUID(统一入口)。

// MD5 计算字符串的 MD5 摘要(32 位小写 hex)。
// 注意:MD5 仅用于非安全场景(签名校验、去重、缓存键),密码存储请使用 bcrypt/argon2。
func MD5(s string) string {
	return MD5Bytes([]byte(s))
}

// MD5Bytes 计算字节数据的 MD5 摘要(32 位小写 hex)。
func MD5Bytes(data []byte) string {
	digest := md5.Sum(data)
	return hex.EncodeToString(digest[:])
}

// SHA1 计算字符串的 SHA1 摘要(40 位小写 hex)。
func SHA1(s string) string {
	digest := sha1.Sum([]byte(s))
	return hex.EncodeToString(digest[:])
}

// SHA256 计算字符串的 SHA256 摘要(64 位小写 hex)。
func SHA256(s string) string {
	digest := sha256.Sum256([]byte(s))
	return hex.EncodeToString(digest[:])
}

// RandomString 生成指定长度的随机字母数字串(0-9a-zA-Z)。
// 随机源失败时返回错误(系统级故障,不建议静默降级)。
func RandomString(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	buffer := make([]byte, n)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	for i := range buffer {
		buffer[i] = alphabet[int(buffer[i])%len(alphabet)]
	}
	return string(buffer), nil
}

// RandomStringMust 生成随机串;失败时 panic(仅限启动期/非关键路径使用)。
func RandomStringMust(n int) string {
	value, err := RandomString(n)
	if err != nil {
		panic("util: crypto/rand unavailable: " + err.Error())
	}
	return value
}

// UUID 生成 UUID v4 字符串(转发 framework/database/id,统一工具入口)。
func UUID() (string, error) {
	return sid.UUID()
}

// UUIDOrEmpty 生成 UUID;失败时返回空串(调用方不关心失败场景时使用)。
func UUIDOrEmpty() string {
	value, err := sid.UUID()
	if err != nil {
		return ""
	}
	return value
}
