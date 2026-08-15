package id

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
)

// ErrUUIDGenerationFailed UUID 随机源读取失败。
var ErrUUIDGenerationFailed = errors.New("uuid: crypto/rand read failed")

// UUID 生成标准 UUID v4(36 字符,如 6ba7b810-9dad-11d1-80b4-00c04fd430c8)。
// 基于 crypto/rand,零第三方依赖;版本位设为 4、变体位设为 10。
// 失败时返回错误(随机源故障属于不可恢复的系统级问题)。
func UUID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", ErrUUIDGenerationFailed
	}
	// 版本 4
	buffer[6] = (buffer[6] & 0x0f) | 0x40
	// variant 10
	buffer[8] = (buffer[8] & 0x3f) | 0x80

	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], buffer[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], buffer[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], buffer[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], buffer[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], buffer[10:16])
	return string(encoded), nil
}
