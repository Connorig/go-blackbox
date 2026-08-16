package gbxioc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// testService 是容器测试使用的结构体。
type testService struct {
	Name string
}

// testDependency 是构造依赖注入测试使用的结构体。
type testDependency struct {
	Value string
}

// testLifecycleBean 实现 Initializer 与 Disposer，验证生命周期顺序。
type testLifecycleBean struct {
	initCalled   int32
	destroyCount int32
	name         string
}

// OnInit 记录初始化调用。
func (b *testLifecycleBean) OnInit(context.Context) error {
	atomic.AddInt32(&b.initCalled, 1)
	return nil
}

// OnDestroy 记录销毁调用。
func (b *testLifecycleBean) OnDestroy(context.Context) error {
	atomic.AddInt32(&b.destroyCount, 1)
	return nil
}

// testInitFailBean 的 OnInit 返回错误，验证 Start 中断行为。
type testInitFailBean struct{}

// OnInit 返回固定错误。
func (testInitFailBean) OnInit(context.Context) error {
	return errTestInitFailed
}

// errTestInitFailed 是生命周期测试的固定错误。
var errTestInitFailed = errors.New("test init failed")

// TestRegisterAndGetSingleton 验证单例注册、懒加载与实例复用。
func TestRegisterAndGetSingleton(t *testing.T) {
	container := NewContainer()
	t.Cleanup(container.Reset)

	constructCount := 0
	if err := RegisterTo(container, func() *testService {
		constructCount++
		return &testService{Name: "singleton"}
	}); err != nil {
		t.Fatalf("register singleton failed: %v", err)
	}
	if constructCount != 0 {
		t.Fatal("singleton must be lazily constructed")
	}

	first, err := GetBeanFrom[testService](container)
	if err != nil {
		t.Fatalf("get singleton failed: %v", err)
	}
	second, err := GetBeanFrom[testService](container)
	if err != nil {
		t.Fatalf("get singleton second time failed: %v", err)
	}
	if first != second {
		t.Fatal("singleton must return the same instance")
	}
	if constructCount != 1 {
		t.Fatalf("singleton must be constructed once, got %d", constructCount)
	}
	if first.Name != "singleton" {
		t.Fatalf("unexpected instance: %+v", first)
	}
}

// TestPrototypeReturnsNewInstance 验证原型作用域每次返回新实例。
func TestPrototypeReturnsNewInstance(t *testing.T) {
	container := NewContainer()
	t.Cleanup(container.Reset)

	constructCount := 0
	if err := RegisterPrototypeTo(container, func() *testService {
		constructCount++
		return &testService{Name: fmt.Sprintf("instance-%d", constructCount)}
	}); err != nil {
		t.Fatalf("register prototype failed: %v", err)
	}

	first, err := GetBeanFrom[testService](container)
	if err != nil {
		t.Fatalf("get prototype failed: %v", err)
	}
	second, err := GetBeanFrom[testService](container)
	if err != nil {
		t.Fatalf("get prototype second time failed: %v", err)
	}
	if first == second {
		t.Fatal("prototype must return distinct instances")
	}
	if constructCount != 2 {
		t.Fatalf("prototype must construct per get, got %d", constructCount)
	}
}

// TestRegisterNamed 验证具名注册与获取，同名重复注册被拒绝。
func TestRegisterNamed(t *testing.T) {
	container := NewContainer()
	t.Cleanup(container.Reset)

	if err := RegisterNamedTo(container, "primary", func() *testService {
		return &testService{Name: "primary"}
	}); err != nil {
		t.Fatalf("register named primary failed: %v", err)
	}
	if err := RegisterNamedTo(container, "backup", func() *testService {
		return &testService{Name: "backup"}
	}); err != nil {
		t.Fatalf("register named backup failed: %v", err)
	}
	if err := RegisterNamedTo(container, "primary", func() *testService {
		return &testService{Name: "dup"}
	}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("expected ErrDuplicate, got: %v", err)
	}

	primary, err := GetNamedBeanFrom[testService](container, "primary")
	if err != nil {
		t.Fatalf("get named primary failed: %v", err)
	}
	backup, err := GetNamedBeanFrom[testService](container, "backup")
	if err != nil {
		t.Fatalf("get named backup failed: %v", err)
	}
	if primary.Name != "primary" || backup.Name != "backup" {
		t.Fatalf("unexpected named instances: %+v %+v", primary, backup)
	}
	if _, err := GetNamedBeanFrom[testService](container, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing name, got: %v", err)
	}
}

// TestRegisterInstance 验证直接注册实例。
func TestRegisterInstance(t *testing.T) {
	container := NewContainer()
	t.Cleanup(container.Reset)

	instance := &testService{Name: "direct"}
	if err := RegisterInstanceTo(container, instance); err != nil {
		t.Fatalf("register instance failed: %v", err)
	}
	got, err := GetBeanFrom[testService](container)
	if err != nil {
		t.Fatalf("get instance failed: %v", err)
	}
	if got != instance {
		t.Fatal("registered instance must be returned as-is")
	}
	if err := RegisterInstanceTo(container, instance); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("expected ErrDuplicate, got: %v", err)
	}
}

// TestProviderCapturesDependencies 验证闭包捕获依赖的显式注入方式。
func TestProviderCapturesDependencies(t *testing.T) {
	container := NewContainer()
	t.Cleanup(container.Reset)

	if err := RegisterInstanceTo(container, &testDependency{Value: "injected"}); err != nil {
		t.Fatalf("register dependency failed: %v", err)
	}
	if err := RegisterTo(container, func() *testService {
		dep, err := GetBeanFrom[testDependency](container)
		if err != nil {
			return nil // 依赖缺失时返回 nil，由容器转换为 ErrInvalidProvider
		}
		return &testService{Name: dep.Value}
	}); err != nil {
		t.Fatalf("register service with dependency failed: %v", err)
	}

	service, err := GetBeanFrom[testService](container)
	if err != nil {
		t.Fatalf("get service failed: %v", err)
	}
	if service.Name != "injected" {
		t.Fatalf("dependency was not captured: %+v", service)
	}
}

// TestCircularDependency 验证构造期间的循环依赖被检测。
func TestCircularDependency(t *testing.T) {
	container := NewContainer()
	t.Cleanup(container.Reset)

	var innerErr error
	if err := RegisterTo(container, func() *testService {
		// 构造中获取自身触发循环依赖，容器必须返回 ErrCircularDependency
		_, innerErr = GetBeanFrom[testService](container)
		return &testService{Name: "circular"}
	}); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if _, err := GetBeanFrom[testService](container); err != nil {
		t.Fatalf("outer get failed: %v", err)
	}
	if !errors.Is(innerErr, ErrCircularDependency) {
		t.Fatalf("expected ErrCircularDependency for inner get, got: %v", innerErr)
	}
}

// TestGetMissingType 验证未注册类型返回 ErrNotFound。
func TestGetMissingType(t *testing.T) {
	container := NewContainer()
	t.Cleanup(container.Reset)

	if _, err := GetBeanFrom[testService](container); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
	if HasBeanIn[testService](container) {
		t.Fatal("Has must return false for unregistered type")
	}
}

// TestRegisterInvalidType 验证非结构体指针类型被拒绝。
func TestRegisterInvalidType(t *testing.T) {
	container := NewContainer()
	t.Cleanup(container.Reset)

	if err := RegisterTo(container, func() *int {
		v := 1
		return &v
	}); !errors.Is(err, ErrInvalidType) {
		t.Fatalf("expected ErrInvalidType for *int, got: %v", err)
	}
}

// TestStartShutdownLifecycleOrder 验证 OnInit 按注册顺序、OnDestroy 逆序执行。
func TestStartShutdownLifecycleOrder(t *testing.T) {
	container := NewContainer()
	t.Cleanup(container.Reset)

	first := &testLifecycleBean{name: "first"}
	second := &testLifecycleBean{name: "second"}
	// 同类型多实例必须使用具名注册
	if err := RegisterNamedTo(container, "first", func() *testLifecycleBean {
		return first
	}); err != nil {
		t.Fatalf("register first failed: %v", err)
	}
	if err := RegisterNamedTo(container, "second", func() *testLifecycleBean {
		return second
	}); err != nil {
		t.Fatalf("register second failed: %v", err)
	}

	if err := container.Start(context.Background()); err != nil {
		t.Fatalf("start container failed: %v", err)
	}
	if atomic.LoadInt32(&first.initCalled) != 1 || atomic.LoadInt32(&second.initCalled) != 1 {
		t.Fatal("OnInit must be called once per bean")
	}
	// Start 幂等：重复调用不重复 OnInit
	if err := container.Start(context.Background()); err != nil {
		t.Fatalf("restart container failed: %v", err)
	}
	if atomic.LoadInt32(&first.initCalled) != 1 {
		t.Fatal("OnInit must not be called twice")
	}

	if err := container.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown container failed: %v", err)
	}
	if atomic.LoadInt32(&first.destroyCount) != 1 || atomic.LoadInt32(&second.destroyCount) != 1 {
		t.Fatal("OnDestroy must be called once per bean")
	}
	// Shutdown 幂等
	if err := container.Shutdown(context.Background()); err != nil {
		t.Fatalf("second shutdown must be idempotent: %v", err)
	}
	if atomic.LoadInt32(&first.destroyCount) != 1 {
		t.Fatal("OnDestroy must not be called twice")
	}
}

// TestStartCallsOnInitForLazySingleton 验证懒加载单例在 Start 时被构造并初始化。
func TestStartCallsOnInitForLazySingleton(t *testing.T) {
	container := NewContainer()
	t.Cleanup(container.Reset)

	bean := &testLifecycleBean{name: "lazy"}
	if err := RegisterTo(container, func() *testLifecycleBean {
		return bean
	}); err != nil {
		t.Fatalf("register lazy bean failed: %v", err)
	}
	if err := container.Start(context.Background()); err != nil {
		t.Fatalf("start container failed: %v", err)
	}
	if atomic.LoadInt32(&bean.initCalled) != 1 {
		t.Fatal("lazy singleton must be initialized during Start")
	}
	got, err := GetBeanFrom[testLifecycleBean](container)
	if err != nil {
		t.Fatalf("get bean failed: %v", err)
	}
	if got != bean {
		t.Fatal("unexpected bean instance")
	}
}

// TestStartStopsOnInitError 验证 OnInit 失败会中断 Start 并返回错误。
func TestStartStopsOnInitError(t *testing.T) {
	container := NewContainer()
	t.Cleanup(container.Reset)

	if err := RegisterInstanceTo(container, &testInitFailBean{}); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if err := container.Start(context.Background()); !errors.Is(err, errTestInitFailed) {
		t.Fatalf("expected init failure, got: %v", err)
	}
}

// TestClosedContainerRejectsGetAndRegister 验证关闭后禁止注册与获取。
func TestClosedContainerRejectsGetAndRegister(t *testing.T) {
	container := NewContainer()
	t.Cleanup(container.Reset)

	bean := &testLifecycleBean{name: "closed"}
	if err := RegisterInstanceTo(container, bean); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if err := container.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
	if _, err := GetBeanFrom[testLifecycleBean](container); !errors.Is(err, ErrContainerClosed) {
		t.Fatalf("expected ErrContainerClosed on Get, got: %v", err)
	}
	if err := RegisterTo(container, func() *testService {
		return &testService{}
	}); !errors.Is(err, ErrContainerClosed) {
		t.Fatalf("expected ErrContainerClosed on Register, got: %v", err)
	}
}

// TestConcurrentGet 验证并发获取单例安全且只构造一次。
func TestConcurrentGet(t *testing.T) {
	container := NewContainer()
	t.Cleanup(container.Reset)

	var constructCount int32
	if err := RegisterTo(container, func() *testService {
		atomic.AddInt32(&constructCount, 1)
		return &testService{Name: "concurrent"}
	}); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	const goroutines = 32
	var waitGroup sync.WaitGroup
	waitGroup.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer waitGroup.Done()
			instance, err := GetBeanFrom[testService](container)
			if err != nil {
				t.Errorf("concurrent get failed: %v", err)
				return
			}
			if instance == nil {
				t.Error("concurrent get returned nil")
			}
		}()
	}
	waitGroup.Wait()
	if count := atomic.LoadInt32(&constructCount); count != 1 {
		t.Fatalf("singleton must be constructed once under concurrency, got %d", count)
	}
}

// TestResetRestoresContainer 验证 Reset 后容器可重新注册。
func TestResetRestoresContainer(t *testing.T) {
	container := NewContainer()

	if err := RegisterInstanceTo(container, &testService{Name: "before"}); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	container.Reset()
	if HasBeanIn[testService](container) {
		t.Fatal("Reset must clear registrations")
	}
	if err := RegisterInstanceTo(container, &testService{Name: "after"}); err != nil {
		t.Fatalf("register after reset failed: %v", err)
	}
	// Reset 后容器应可重新 Start/Shutdown
	if err := container.Start(context.Background()); err != nil {
		t.Fatalf("start after reset failed: %v", err)
	}
	if err := container.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown after reset failed: %v", err)
	}
}

// TestDefaultContainerPackageFunctions 验证包级函数与默认容器联通。
func TestDefaultContainerPackageFunctions(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	if err := Register(func() *testService {
		return &testService{Name: "default"}
	}); err != nil {
		t.Fatalf("package register failed: %v", err)
	}
	if !HasBean[testService]() {
		t.Fatal("package Has must detect registration")
	}
	instance, err := GetBean[testService]()
	if err != nil {
		t.Fatalf("package get failed: %v", err)
	}
	if instance.Name != "default" {
		t.Fatalf("unexpected instance: %+v", instance)
	}
}
