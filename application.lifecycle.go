package appbox

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const defaultShutdownTimeout = 15 * time.Second

// applicationState 表示单个 Application 实例当前所处的生命周期阶段。
// 状态只允许从 created 进入 starting、running、stopping，最终进入 stopped。
type applicationState uint8

const (
	// applicationStateCreated 表示应用尚未开始执行 Builder。
	applicationStateCreated applicationState = iota
	// applicationStateStarting 表示应用正在初始化组件。
	applicationStateStarting
	// applicationStateRunning 表示组件已初始化完成，应用正在等待退出信号。
	applicationStateRunning
	// applicationStateStopping 表示应用正在执行逆序资源关闭。
	applicationStateStopping
	// applicationStateStopped 表示应用已经完成关闭，不允许再次启动。
	applicationStateStopped
)

// ShutdownFunc 定义应用退出时执行的资源关闭函数。
// 关闭函数必须响应传入 Context 的超时或取消，并返回资源释放过程中发生的错误。
type ShutdownFunc func(context.Context) error

// shutdownHook 保存关闭功能点名称和对应函数，名称用于错误定位和安全日志输出。
type shutdownHook struct {
	name string
	stop ShutdownFunc
}

// applicationLifecycle 维护应用状态和成功初始化资源的关闭栈。
// 所有状态和关闭栈访问均受互斥锁保护，避免组件异常退出与系统信号并发触发重复关闭。
type applicationLifecycle struct {
	mu            sync.Mutex
	state         applicationState
	shutdownHooks []shutdownHook
}

// beginStart 将新建应用切换到启动中状态。
// 同一 Application 实例只允许启动一次，避免复用全局组件配置和已关闭资源。
func (lifecycle *applicationLifecycle) beginStart() error {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()

	if lifecycle.state != applicationStateCreated {
		return fmt.Errorf("application cannot start from lifecycle state %d", lifecycle.state)
	}
	lifecycle.state = applicationStateStarting
	return nil
}

// markRunning 将完成组件初始化的应用切换到运行状态。
func (lifecycle *applicationLifecycle) markRunning() error {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()

	if lifecycle.state != applicationStateStarting {
		return fmt.Errorf("application cannot become ready from lifecycle state %d", lifecycle.state)
	}
	lifecycle.state = applicationStateRunning
	return nil
}

// registerShutdown 将成功初始化资源的关闭函数加入栈顶。
// name 不能为空且 stop 必须有效；调用方应在资源完全可用后立即注册。
func (lifecycle *applicationLifecycle) registerShutdown(name string, stop ShutdownFunc) error {
	if name == "" {
		return errors.New("shutdown hook name is empty")
	}
	if stop == nil {
		return fmt.Errorf("shutdown hook %q function is nil", name)
	}

	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if lifecycle.state != applicationStateStarting {
		return fmt.Errorf("shutdown hook %q cannot be registered in lifecycle state %d", name, lifecycle.state)
	}
	lifecycle.shutdownHooks = append(lifecycle.shutdownHooks, shutdownHook{name: name, stop: stop})
	return nil
}

// shutdown 只执行一次关闭流程，并按照资源注册顺序的逆序调用关闭函数。
// 每个函数共享同一关闭期限；发生多个错误时使用 errors.Join 聚合并保留错误链。
func (lifecycle *applicationLifecycle) shutdown(parent context.Context, timeout time.Duration) error {
	lifecycle.mu.Lock()
	if lifecycle.state == applicationStateStopping || lifecycle.state == applicationStateStopped {
		lifecycle.mu.Unlock()
		return nil
	}
	lifecycle.state = applicationStateStopping
	hooks := append([]shutdownHook(nil), lifecycle.shutdownHooks...)
	lifecycle.shutdownHooks = nil
	lifecycle.mu.Unlock()

	if parent == nil {
		parent = context.Background()
	}
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}
	shutdownCtx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	var shutdownErrors []error
	for index := len(hooks) - 1; index >= 0; index-- {
		hook := hooks[index]
		if err := hook.stop(shutdownCtx); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("shutdown %s: %w", hook.name, err))
		}
	}

	lifecycle.mu.Lock()
	lifecycle.state = applicationStateStopped
	lifecycle.mu.Unlock()
	return errors.Join(shutdownErrors...)
}
