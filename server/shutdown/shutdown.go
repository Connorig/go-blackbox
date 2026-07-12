package shutdown

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Configuration 定义应用退出信号和收到退出请求后的通知回调。
type Configuration struct {
	// BeforeExit 在取消全局 Context 后同步执行，用于记录退出原因等轻量操作。
	BeforeExit func(string)
	// Signals 指定需要监听的系统信号；为空时监听 SIGINT 和 SIGTERM。
	Signals []os.Signal
}

// exitRequest 保存组件主动请求退出时提供的非敏感原因。
type exitRequest struct {
	message string
}

// defaultSignals 是每次 WaitExit 未显式配置时使用的系统信号集合。
var defaultSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}

// exitChan 保存首个组件主动退出请求，缓冲区避免发送方在监听启动前永久阻塞。
var exitChan = make(chan exitRequest, 1)

// 全局 Context 兼容现有调用方，用于把首次退出请求广播给后台组件。
var (
	ctx        context.Context
	cancel     context.CancelFunc
	cancelOnce sync.Once
)

func init() {
	ctx, cancel = context.WithCancel(context.Background())
}

// Context 返回进程级退出 Context。
// 首次收到系统信号或 Exit 请求时该 Context 会被取消，调用方不得尝试复用已退出的进程状态。
func Context() context.Context {
	return ctx
}

// WaitExit 阻塞等待系统信号或组件主动退出，并返回触发退出的原因。
// 函数返回前会停止本次 signal.Notify 注册，避免重复调用累积信号订阅资源。
func WaitExit(config *Configuration) string {
	signalChannel := make(chan os.Signal, 1)
	signals := configuredSignals(config)
	signal.Notify(signalChannel, signals...)
	defer signal.Stop(signalChannel)

	return waitExit(config, signalChannel, exitChan)
}

// configuredSignals 为单次等待复制有效信号配置，避免修改包级默认切片影响后续调用。
func configuredSignals(config *Configuration) []os.Signal {
	if config != nil && len(config.Signals) > 0 {
		return append([]os.Signal(nil), config.Signals...)
	}
	return append([]os.Signal(nil), defaultSignals...)
}

// waitExit 执行可测试的退出等待逻辑，系统信号和主动退出只会消费其中一个。
func waitExit(config *Configuration, signalChannel <-chan os.Signal, requests <-chan exitRequest) string {
	var message string
	select {
	case request := <-requests:
		message = request.message
	case receivedSignal := <-signalChannel:
		message = receivedSignal.String()
	}

	cancelOnce.Do(cancel)
	if config != nil && config.BeforeExit != nil {
		config.BeforeExit(message)
	}
	return message
}

// Exit 请求应用结束运行。
// 仅保留首个尚未消费的退出原因；后续重复请求直接返回，避免故障 goroutine 永久阻塞。
func Exit(message string) {
	select {
	case exitChan <- exitRequest{message: message}:
	default:
	}
}
