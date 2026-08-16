package appbox

import (
	"fmt"
	"strings"
)

// Version 脚手架版本(发版时与 git tag 同步更新)。
const Version = "1.52.0"

// Author 作者签名。
const Author = "Connor"

// Organization 组织域名。
const Organization = "nexaaico.com"

// BannerText 启动横幅(GBX 大字,Spring Boot 风格)。
const BannerText = `

   ___  ____  __  __
  / __| __ ) \ \/ /
 | | |_|  _ \  \  /
 | |___| |_) | /  \
  \____|____/ /_/\_\

      GBX.APP
   ALL IN GBX.APP

 go-blackbox v%s · %s · %s
`

// bannerEnabled 控制横幅打印(WithoutBanner 关闭)。
var bannerEnabled = true

// WithoutBanner 关闭启动横幅(生产日志干净)。
func WithoutBanner() Option {
	return func(app *ApplicationBuild) {
		bannerEnabled = false
	}
}

// Option 应用构建选项。
type Option func(*ApplicationBuild)

// printBanner 打印启动横幅与版本信息(Spring Boot 风格)。
func printBanner() {
	if !bannerEnabled {
		return
	}
	fmt.Printf(BannerText, Version, Organization, Author)
}

// BannerString 返回横幅文本(测试/文档用)。
func BannerString() string {
	return strings.TrimLeft(fmt.Sprintf(BannerText, Version, Organization, Author), "\n")
}
