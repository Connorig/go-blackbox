package appbox

import (
	"fmt"
)

// 应用信息(业务项目自身版本,横幅追加打印)。

// appInfo 业务应用信息(SetAppInfo 设置)。
type appInfo struct {
	Name    string
	Version string
}

var currentAppInfo *appInfo

// SetAppInfo 设置业务应用信息(名称 + 版本),启动横幅会追加打印:
//
//	app: order-service v1.2.3
//
// 用法:
//
//	app := appbox.New(appbox.SetAppInfo("order-service", "v1.2.3"))
//
// 说明:appbox.Version 是脚手架版本(随 tag 自动维护,业务不用管);
// 本函数注入业务项目自身的版本,两者互不干扰。
func SetAppInfo(name, version string) Option {
	return func(app *ApplicationBuild) {
		currentAppInfo = &appInfo{Name: name, Version: version}
	}
}

// AppInfo 返回业务应用信息(测试/日志用);未设置返回 nil。
func AppInfo() *appInfo {
	return currentAppInfo
}

// printAppInfo 横幅下方追加打印业务应用信息。
func printAppInfo() {
	if currentAppInfo == nil {
		return
	}
	if currentAppInfo.Name != "" {
		fmt.Printf(" app: %s %s\n\n", currentAppInfo.Name, currentAppInfo.Version)
	}
}
