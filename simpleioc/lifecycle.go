package simpleioc

import "context"

// Initializer 由需要在启动时初始化的单例实现。
// 容器 Start 时按注册顺序调用 OnInit；返回错误会终止启动流程。
// 注意：原型（多例）bean 不参与生命周期管理。
type Initializer interface {
	OnInit(ctx context.Context) error
}

// Disposer 由需要在关闭时释放资源的单例实现。
// 容器 Shutdown 时按注册逆序调用 OnDestroy；返回错误会被聚合返回。
// 注意：原型（多例）bean 不参与生命周期管理。
type Disposer interface {
	OnDestroy(ctx context.Context) error
}
