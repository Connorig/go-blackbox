package shutdown

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

type Configuration struct {
	BeforeExit func(string)
	Signals    []os.Signal
}

/*

信号
发
SIGHUP
SIGINT
SIGQUIT
SIGKILL
SIGUSR1
SIGUSR2
SIGPIPE
SIGALRM
SIGTERM
说明
终端控制进程结束(终端连接断开
用户发送INTR字符(Ctrl+C)触发
用户发送QUIT字符(Ctrl+/触发
无条件结束程序(不能被捕获、阻塞或忽略
用户保留
用户保留
消息管道损坏(FIFO/Socket通信时，管道未打开而进行写操作)
时钟定时信号
结束程序(可以被捕获、阻塞或忽略*/
var defaultSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
var exitChan = make(chan struct{ message string })

var ctx context.Context
var cancel context.CancelFunc

func init() {
	ctx, cancel = context.WithCancel(context.Background())
}

func Context() context.Context {
	return ctx
}

func WaitExit(config *Configuration) {

	sigChan := make(chan os.Signal, 1)

	if config != nil {
		if len(config.Signals) > 0 {
			defaultSignals = config.Signals
		}
	}

	signal.Notify(sigChan, defaultSignals...)

	select {
	case s := <-exitChan:
		onExit(s.message, config)
	case s := <-sigChan:
		onExit(s.String(), config)
	}
}

func onExit(s string, config *Configuration) {

	defer func() {
		_ = recover()
	}()

	cancel()
	if config != nil && config.BeforeExit != nil {
		config.BeforeExit(s)
	}
}

func Exit(msg string) {
	exitChan <- struct{ message string }{msg}
}
