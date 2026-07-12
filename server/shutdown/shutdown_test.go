package shutdown

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// TestWaitExitReturnsComponentRequest 验证组件主动退出原因会返回并触发退出前回调。
func TestWaitExitReturnsComponentRequest(t *testing.T) {
	signalChannel := make(chan os.Signal, 1)
	requestChannel := make(chan exitRequest, 1)
	requestChannel <- exitRequest{message: "worker stopped"}

	callbackMessage := ""
	message := waitExit(&Configuration{BeforeExit: func(value string) {
		callbackMessage = value
	}}, signalChannel, requestChannel)

	if message != "worker stopped" {
		t.Fatalf("unexpected exit message: %s", message)
	}
	if callbackMessage != message {
		t.Fatalf("unexpected callback message: %s", callbackMessage)
	}
}

// TestWaitExitReturnsSystemSignal 验证系统信号能够结束等待且不会依赖真实进程信号。
func TestWaitExitReturnsSystemSignal(t *testing.T) {
	signalChannel := make(chan os.Signal, 1)
	requestChannel := make(chan exitRequest)
	signalChannel <- syscall.SIGTERM

	message := waitExit(nil, signalChannel, requestChannel)
	if message != syscall.SIGTERM.String() {
		t.Fatalf("unexpected signal message: %s", message)
	}
}

// TestExitDoesNotBlockWhenRequestIsPending 验证重复主动退出不会阻塞故障处理 goroutine。
func TestExitDoesNotBlockWhenRequestIsPending(t *testing.T) {
	previousChannel := exitChan
	exitChan = make(chan exitRequest, 1)
	t.Cleanup(func() {
		exitChan = previousChannel
	})

	Exit("first")
	completed := make(chan struct{})
	go func() {
		Exit("second")
		close(completed)
	}()

	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("repeated Exit call blocked")
	}

	request := <-exitChan
	if request.message != "first" {
		t.Fatalf("first exit request was not preserved: %s", request.message)
	}
}

// TestConfiguredSignalsDoesNotMutateDefaults 验证自定义信号配置不会污染后续默认监听集合。
func TestConfiguredSignalsDoesNotMutateDefaults(t *testing.T) {
	custom := configuredSignals(&Configuration{Signals: []os.Signal{syscall.SIGHUP}})
	custom[0] = syscall.SIGQUIT

	defaults := configuredSignals(nil)
	if len(defaults) != 2 || defaults[0] != syscall.SIGINT || defaults[1] != syscall.SIGTERM {
		t.Fatalf("default signals were mutated: %v", defaults)
	}
}
