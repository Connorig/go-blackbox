// Package safe 提供 goroutine panic 治理工具:
// 业务/回调在独立 goroutine 中 panic 时,默认行为是崩溃整个进程;
// safe.Go 捕获 panic 并记录日志与堆栈,保证服务继续运行。
package safe

import (
	"fmt"
	"runtime/debug"

	zaplog "github.com/Connorig/go-blackbox/framework/log"
)

// Go 启动带 panic 恢复的 goroutine:fn 内部 panic 会被捕获,
// 记录组件名、错误与堆栈后继续运行(不崩溃进程)。
// 用法:
//
//	safe.Go("redqueue consume", func() { ... })
func Go(name string, fn func()) {
	if fn == nil {
		return
	}
	go func() {
		defer Recover(name)
		fn()
	}()
}

// Recover 供 defer 使用:捕获当前 goroutine 的 panic 并记录日志。
//
//	defer safe.Recover("my goroutine")
func Recover(name string) {
	if r := recover(); r != nil {
		zaplog.SugaredLogger.Errorw("goroutine panic recovered",
			"name", name,
			"panic", fmt.Sprintf("%v", r),
			"stack", string(debug.Stack()),
		)
	}
}
