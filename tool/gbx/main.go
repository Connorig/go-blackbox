// Command gbx 是 go-blackbox 脚手架的代码生成 CLI:
// 一键生成符合泰山版规范 + 安全基线的完整业务项目骨架。
//
// 用法(两种形式等价):
//
//	gbx new -name demo                    // 在当前目录生成 ./demo
//	gbx -name demo
//	gbx new -name demo -module github.com/connor/demo
//	gbx new -name demo -dir D:\projects
//
// 生成内容:
//   - main.go:应用骨架(日志/Web/中间件安全基线/JWT 认证/健康检查/监控/Admin/迁移/数据权限)
//   - internal/model:StandardModel 业务模型示例
//   - internal/handler:业务 handler 示例(统一响应 + 错误码 + 数据权限)
//   - go.mod / README.md / .gitignore
//
// 安装:go install github.com/Connorig/go-blackbox/tool/gbx
// 或直接:go run github.com/Connorig/go-blackbox/tool/gbx new -name demo
package main

import (
	"flag"
	"fmt"
	"os"
)

const version = "v1.14.0"

// options 命令行参数。
type options struct {
	name   string
	module string
	dir    string
}

func main() {
	// 支持两种形式:gbx new -name x 与 gbx -name x
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "new" {
		args = args[1:]
	}

	var opts options
	fs := flag.NewFlagSet("gbx", flag.ExitOnError)
	fs.StringVar(&opts.name, "name", "", "项目名称(必填,同时作为目录名)")
	fs.StringVar(&opts.module, "module", "", "Go module 路径(默认 github.com/<name>/<name>)")
	fs.StringVar(&opts.dir, "dir", ".", "生成目录(默认当前目录)")
	showVersion := fs.Bool("version", false, "显示版本")
	_ = fs.Parse(args)

	if *showVersion {
		fmt.Printf("gbx %s\n", version)
		return
	}

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "gbx: %v\n", err)
		os.Exit(1)
	}
}
