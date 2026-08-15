package simpleioc

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// Scope 定义 bean 的实例作用域。
type Scope uint8

const (
	// ScopeSingleton 单例：整个容器共享一个实例（默认）。
	// 单例使用懒加载，首次 Get 或 Start 时构造，之后复用。
	ScopeSingleton Scope = iota
	// ScopePrototype 原型（多例）：每次 Get 都创建新实例，容器不管理其生命周期。
	ScopePrototype
)

// beanDefinition 保存单个 bean 的注册信息与构造状态。
type beanDefinition struct {
	scope        Scope
	provider     func() interface{}
	instance     interface{}
	mu           sync.Mutex
	constructing bool
	onInitDone   bool
}

// Container 是线程安全的依赖容器。
// 支持类型注册、具名注册、单例/原型作用域、懒加载构造、循环依赖检测与生命周期钩子。
// 泛型 API 由包级函数提供（Go 不支持泛型方法）：Register/Get/GetFrom 等。
type Container struct {
	mu     sync.RWMutex
	beans  map[reflect.Type]*beanDefinition
	named  map[string]*beanDefinition
	order  []*beanDefinition // 注册顺序，用于 Start/Shutdown 编排
	closed bool
}

// NewContainer 创建独立容器，适用于测试隔离与多应用场景。
// 进程级默认容器使用包级函数访问。
func NewContainer() *Container {
	return &Container{
		beans: make(map[reflect.Type]*beanDefinition),
		named: make(map[string]*beanDefinition),
	}
}

// register 执行注册的公共路径。
// beanType 必须是非 nil 的结构体指针类型；name 非空时按名称注册，否则按类型注册。
func (c *Container) register(beanType reflect.Type, name string, scope Scope, provider func() interface{}, instance interface{}) error {
	if provider == nil && instance == nil {
		return ErrInvalidProvider
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrContainerClosed
	}

	def := &beanDefinition{scope: scope, provider: provider, instance: instance}

	if name != "" {
		if _, exists := c.named[name]; exists {
			return fmt.Errorf("%w: name %q", ErrDuplicate, name)
		}
		c.named[name] = def
	} else {
		if _, exists := c.beans[beanType]; exists {
			return fmt.Errorf("%w: %v", ErrDuplicate, beanType)
		}
		c.beans[beanType] = def
	}
	c.order = append(c.order, def)
	return nil
}

// get 查找并构造 bean 的公共路径，支持循环依赖检测。
func (c *Container) get(beanType reflect.Type, name string) (interface{}, error) {
	c.mu.RLock()
	closed := c.closed
	var def *beanDefinition
	if name != "" {
		def = c.named[name]
	} else {
		def = c.beans[beanType]
	}
	c.mu.RUnlock()
	if closed {
		return nil, ErrContainerClosed
	}
	if def == nil {
		return nil, ErrNotFound
	}

	if def.scope == ScopePrototype {
		return c.construct(def)
	}

	// 单例：懒加载 + 构造互斥 + 循环依赖检测
	// provider 必须在锁外执行，否则构造中递归获取自身会死锁；
	// 循环依赖通过 constructing 标记检测。
	def.mu.Lock()
	if def.constructing {
		def.mu.Unlock()
		return nil, ErrCircularDependency
	}
	if def.instance != nil {
		def.mu.Unlock()
		return def.instance, nil
	}
	if def.provider == nil {
		def.mu.Unlock()
		return nil, ErrInvalidProvider
	}
	def.constructing = true
	def.mu.Unlock()

	instance := def.provider()

	def.mu.Lock()
	def.constructing = false
	if instance == nil {
		def.mu.Unlock()
		return nil, ErrInvalidProvider
	}
	def.instance = instance
	def.mu.Unlock()
	return instance, nil
}

// construct 执行原型 bean 的构造，同样检测构造期间的循环依赖。
func (c *Container) construct(def *beanDefinition) (interface{}, error) {
	def.mu.Lock()
	if def.constructing {
		def.mu.Unlock()
		return nil, ErrCircularDependency
	}
	def.constructing = true
	def.mu.Unlock()

	instance := def.provider()

	def.mu.Lock()
	def.constructing = false
	def.mu.Unlock()

	if instance == nil {
		return nil, ErrInvalidProvider
	}
	return instance, nil
}

// Start 按注册顺序构造全部单例，并对实现 Initializer 的实例调用 OnInit。
// Start 幂等：重复调用不会重复初始化已初始化的实例。
func (c *Container) Start(ctx context.Context) error {
	c.mu.RLock()
	closed := c.closed
	order := append([]*beanDefinition(nil), c.order...)
	c.mu.RUnlock()
	if closed {
		return ErrContainerClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}

	for _, def := range order {
		if def.scope != ScopeSingleton {
			continue
		}
		instance, err := c.getForLifecycle(def)
		if err != nil {
			return err
		}
		if instance == nil {
			continue
		}
		initializer, ok := instance.(Initializer)
		if !ok {
			continue
		}
		def.mu.Lock()
		alreadyDone := def.onInitDone
		def.mu.Unlock()
		if alreadyDone {
			continue
		}
		if err := initializer.OnInit(ctx); err != nil {
			return fmt.Errorf("simpleioc: OnInit %T: %w", instance, err)
		}
		def.mu.Lock()
		def.onInitDone = true
		def.mu.Unlock()
	}
	return nil
}

// getForLifecycle 获取单例实例；未构造时执行构造（Start 的初始化职责）。
// provider 必须在锁外执行，避免构造期间递归获取自身造成死锁。
func (c *Container) getForLifecycle(def *beanDefinition) (interface{}, error) {
	def.mu.Lock()
	if def.instance != nil {
		def.mu.Unlock()
		return def.instance, nil
	}
	if def.provider == nil {
		def.mu.Unlock()
		return nil, ErrInvalidProvider
	}
	def.constructing = true
	def.mu.Unlock()

	instance := def.provider()

	def.mu.Lock()
	def.constructing = false
	if instance == nil {
		def.mu.Unlock()
		return nil, ErrInvalidProvider
	}
	def.instance = instance
	def.mu.Unlock()
	return instance, nil
}

// Shutdown 按注册逆序对已构造单例调用 OnDestroy，之后容器进入关闭态。
// Shutdown 幂等；多个关闭错误使用 errors.Join 聚合。
func (c *Container) Shutdown(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	order := append([]*beanDefinition(nil), c.order...)
	c.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}

	var shutdownErrors []error
	for index := len(order) - 1; index >= 0; index-- {
		def := order[index]
		def.mu.Lock()
		instance := def.instance
		def.mu.Unlock()
		if instance == nil {
			continue
		}
		disposer, ok := instance.(Disposer)
		if !ok {
			continue
		}
		if err := disposer.OnDestroy(ctx); err != nil {
			shutdownErrors = append(shutdownErrors, fmt.Errorf("simpleioc: OnDestroy %T: %w", instance, err))
		}
	}
	return errors.Join(shutdownErrors...)
}

// Reset 清空容器全部注册与实例，恢复到新建状态（测试隔离专用）。
// Reset 不会调用任何 OnDestroy。
func (c *Container) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.beans = make(map[reflect.Type]*beanDefinition)
	c.named = make(map[string]*beanDefinition)
	c.order = nil
	c.closed = false
}

// beanTypeOf 校验并返回泛型参数的指针结构体类型。
func beanTypeOf[T any]() (reflect.Type, error) {
	var zero *T
	beanType := reflect.TypeOf(zero)
	if beanType == nil || beanType.Kind() != reflect.Ptr || beanType.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w: got %v", ErrInvalidType, beanType)
	}
	return beanType, nil
}
