package simpleioc

import (
	"context"
	"fmt"
	"reflect"
)

// 泛型 API 层：Go 不支持泛型方法，因此容器能力通过包级泛型函数暴露。
// 默认容器（defaultContainer）使用 Register/GetBean 系列；显式容器使用 RegisterTo/GetBeanFrom 系列。
// 注意：旧兼容入口 Get[T](bean T) T 占用 Get 名称，因此新 API 使用 GetBean 命名。

// Register 在默认容器注册类型单例 provider。
// provider 在首次 GetBean 或 Start 时执行；同类型重复注册返回 ErrDuplicate。
func Register[T any](provider func() *T) error {
	return RegisterTo(defaultContainer, provider)
}

// RegisterNamed 在默认容器注册具名单例 provider。
func RegisterNamed[T any](name string, provider func() *T) error {
	return RegisterNamedTo(defaultContainer, name, provider)
}

// RegisterPrototype 在默认容器注册原型（多例）provider，每次获取返回新实例。
func RegisterPrototype[T any](provider func() *T) error {
	return RegisterPrototypeTo(defaultContainer, provider)
}

// RegisterInstance 在默认容器直接注册已构造实例（等价旧 Set 语义）。
func RegisterInstance[T any](instance *T) error {
	return RegisterInstanceTo(defaultContainer, instance)
}

// GetBean 从默认容器按类型获取 bean。
func GetBean[T any]() (*T, error) {
	return GetBeanFrom[T](defaultContainer)
}

// GetNamedBean 从默认容器按名称获取 bean。
func GetNamedBean[T any](name string) (*T, error) {
	return GetNamedBeanFrom[T](defaultContainer, name)
}

// MustGetBean 从默认容器按类型获取 bean，失败时 panic（仅限启动期使用）。
func MustGetBean[T any]() *T {
	return MustGetBeanFrom[T](defaultContainer)
}

// HasBean 判断默认容器是否已注册该类型（不触发构造）。
func HasBean[T any]() bool {
	return HasBeanIn[T](defaultContainer)
}

// Start 启动默认容器：按注册顺序构造单例并执行 OnInit。
func Start(ctx context.Context) error {
	return defaultContainer.Start(ctx)
}

// Shutdown 关闭默认容器：逆序执行 OnDestroy，之后容器不可再注册/获取。
func Shutdown(ctx context.Context) error {
	return defaultContainer.Shutdown(ctx)
}

// Reset 重置默认容器（测试隔离专用）。
func Reset() {
	defaultContainer.Reset()
}

// RegisterTo 在指定容器注册类型单例 provider。
func RegisterTo[T any](container *Container, provider func() *T) error {
	if container == nil {
		return fmt.Errorf("%w: container is nil", ErrContainerClosed)
	}
	beanType, err := beanTypeOf[T]()
	if err != nil {
		return err
	}
	if provider == nil {
		return ErrInvalidProvider
	}
	return container.register(beanType, "", ScopeSingleton, func() interface{} {
		return provider()
	}, nil)
}

// RegisterNamedTo 在指定容器注册具名单例 provider。
func RegisterNamedTo[T any](container *Container, name string, provider func() *T) error {
	if container == nil {
		return fmt.Errorf("%w: container is nil", ErrContainerClosed)
	}
	beanType, err := beanTypeOf[T]()
	if err != nil {
		return err
	}
	if provider == nil {
		return ErrInvalidProvider
	}
	return container.register(beanType, name, ScopeSingleton, func() interface{} {
		return provider()
	}, nil)
}

// RegisterPrototypeTo 在指定容器注册原型（多例）provider。
func RegisterPrototypeTo[T any](container *Container, provider func() *T) error {
	if container == nil {
		return fmt.Errorf("%w: container is nil", ErrContainerClosed)
	}
	beanType, err := beanTypeOf[T]()
	if err != nil {
		return err
	}
	if provider == nil {
		return ErrInvalidProvider
	}
	return container.register(beanType, "", ScopePrototype, func() interface{} {
		return provider()
	}, nil)
}

// RegisterInstanceTo 在指定容器直接注册已构造实例。
func RegisterInstanceTo[T any](container *Container, instance *T) error {
	if container == nil {
		return fmt.Errorf("%w: container is nil", ErrContainerClosed)
	}
	beanType, err := beanTypeOf[T]()
	if err != nil {
		return err
	}
	if instance == nil {
		return ErrInvalidProvider
	}
	return container.register(beanType, "", ScopeSingleton, nil, instance)
}

// GetBeanFrom 从指定容器按类型获取 bean。
func GetBeanFrom[T any](container *Container) (*T, error) {
	if container == nil {
		return nil, fmt.Errorf("%w: container is nil", ErrContainerClosed)
	}
	beanType, err := beanTypeOf[T]()
	if err != nil {
		return nil, err
	}
	instance, err := container.get(beanType, "")
	if err != nil {
		return nil, err
	}
	return instance.(*T), nil
}

// GetNamedBeanFrom 从指定容器按名称获取 bean。
func GetNamedBeanFrom[T any](container *Container, name string) (*T, error) {
	if container == nil {
		return nil, fmt.Errorf("%w: container is nil", ErrContainerClosed)
	}
	beanType, err := beanTypeOf[T]()
	if err != nil {
		return nil, err
	}
	instance, err := container.get(beanType, name)
	if err != nil {
		return nil, err
	}
	return instance.(*T), nil
}

// MustGetBeanFrom 从指定容器按类型获取 bean，失败时 panic（仅限启动期使用）。
func MustGetBeanFrom[T any](container *Container) *T {
	instance, err := GetBeanFrom[T](container)
	if err != nil {
		panic(err)
	}
	return instance
}

// HasBeanIn 判断指定容器是否已注册该类型（不触发构造）。
func HasBeanIn[T any](container *Container) bool {
	if container == nil {
		return false
	}
	beanType, err := beanTypeOf[T]()
	if err != nil {
		return false
	}
	container.mu.RLock()
	defer container.mu.RUnlock()
	_, exists := container.beans[beanType]
	return exists
}

// compatGet 是旧 Get 兼容入口使用的按类型查找。
// Deprecated: 仅内部兼容层调用。
func compatGet(beanType reflect.Type) (interface{}, error) {
	return defaultContainer.get(beanType, "")
}
