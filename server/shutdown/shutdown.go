package shutdown

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

type Configuration struct {
	// 退出前回调操作
	BeforeExit func(string)
	// 定义要接受的系统信号
	Signals []os.Signal
}

/*
系统信号
	SIGHUP 	终端控制进程结束(终端连接断开
	SIGINT 	用户发送INTR字符(Ctrl+C)触发
	SIGQUIT 用户发送QUIT字符(Ctrl+/触发
	SIGKILL 无条件结束程序(不能被捕获、阻塞或忽略
	SIGUSR1 用户保留
	SIGUSR2 用户保留
	SIGPIPE 消息管道损坏(FIFO/Socket通信时，管道未打开而进行写操作)
	SIGALRM 时钟定时信号
	SIGTERM 结束程序(可以被捕获、阻塞或忽略
*/

// 定义通道-接收系统信号值类型
var defaultSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}

// 退出通道-用于系统进程退出前的回调
var exitChan = make(chan struct{ message string })

// 全局上下文
var ctx context.Context

// context包提供上下文机制在 goroutine 之间传递 deadline、取消信号（cancellation signals）或者其他请求相关的信息
var cancel context.CancelFunc

func init() {
	ctx, cancel = context.WithCancel(context.Background())
}

func Context() context.Context {
	return ctx
}

func WaitExit(config *Configuration) {
	// 创建系统信号通道
	sigChan := make(chan os.Signal, 1)

	if config != nil {
		if len(config.Signals) > 0 {
			defaultSignals = config.Signals
		}
	}
	// 监听 defaultSignals 系统默认信号，并通知 sigChan
	signal.Notify(sigChan, defaultSignals...)

	select {
	// 结束自定义退出信号
	case s := <-exitChan:
		onExit(s.message, config)
		// 接收系统退出信号
	case s := <-sigChan:
		onExit(s.String(), config)
	}
}

// 退出context上下文时的回调
func onExit(s string, config *Configuration) {

	defer func() {
		// 捕捉异常
		_ = recover()
	}()

	// 结束该上下文
	cancel()
	if config != nil && config.BeforeExit != nil {
		// 结束钱执行回调
		config.BeforeExit(s)
	}
}

// Exit 发送退出进程信号
func Exit(msg string) {
	exitChan <- struct{ message string }{msg}
}
