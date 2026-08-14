package simpleioc

import "errors"

// 容器错误定义。调用方应使用 errors.Is 判断具体失败原因。
var (
	// ErrNotFound 表示请求的类型或名称未注册。
	ErrNotFound = errors.New("simpleioc: bean not found")
	// ErrDuplicate 表示同类型或同名称的 bean 已存在。
	ErrDuplicate = errors.New("simpleioc: bean already registered")
	// ErrInvalidProvider 表示 provider 为 nil 或返回 nil 实例。
	ErrInvalidProvider = errors.New("simpleioc: provider is nil or returned nil")
	// ErrInvalidType 表示注册/获取的泛型类型不是结构体指针。
	ErrInvalidType = errors.New("simpleioc: type must be a pointer to struct")
	// ErrCircularDependency 表示构造过程中递归获取同一个 bean。
	ErrCircularDependency = errors.New("simpleioc: circular dependency detected")
	// ErrContainerClosed 表示容器已关闭，禁止继续注册或获取。
	ErrContainerClosed = errors.New("simpleioc: container is closed")
)
