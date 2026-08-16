package appbox

import (
	"strings"
	"testing"
)

// TestSetAppInfo 设置与读取。
func TestSetAppInfo(t *testing.T) {
	currentAppInfo = nil
	SetAppInfo("order-service", "v1.2.3")(&ApplicationBuild{})
	info := AppInfo()
	if info == nil || info.Name != "order-service" || info.Version != "v1.2.3" {
		t.Fatalf("app info wrong: %+v", info)
	}
	// 横幅文案(业务信息在 printAppInfo 中输出,验证内容)
	if !strings.Contains(info.Name, "order-service") {
		t.Fatal("name wrong")
	}
	currentAppInfo = nil
}

// TestSetAppInfoNil 未设置返回 nil。
func TestSetAppInfoNil(t *testing.T) {
	currentAppInfo = nil
	if AppInfo() != nil {
		t.Fatal("must be nil before set")
	}
}
