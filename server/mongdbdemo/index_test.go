package mongdbdemo

import (
	"os"
	"testing"
)

// TestGetClient 验证 MongoDB 演示用例；需要真实 MongoDB 服务，默认跳过。
// 设置 GO_BLACKBOX_MONGO_ADDR 后才会执行（演示代码内部使用硬编码连接，仅用于人工验证）。
func TestGetClient(t *testing.T) {
	if os.Getenv("GO_BLACKBOX_MONGO_ADDR") == "" {
		t.Skip("MongoDB demo test requires GO_BLACKBOX_MONGO_ADDR environment variable")
	}

	GetClient()
	Find()
}
