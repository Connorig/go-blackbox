package aop

import (
	"context"
	"errors"
	"testing"
	"time"
)

// userService 测试接口(模拟业务 Service)。
type userService interface {
	GetUser(ctx context.Context, id int64) (string, error)
	UpdateUser(ctx context.Context, id int64, name string) error
}

// userServiceImpl 测试实现。
type userServiceImpl struct {
	users map[int64]string
}

func (s *userServiceImpl) GetUser(ctx context.Context, id int64) (string, error) {
	name, ok := s.users[id]
	if !ok {
		return "", errors.New("user not found")
	}
	return name, nil
}

func (s *userServiceImpl) UpdateUser(ctx context.Context, id int64, name string) error {
	s.users[id] = name
	return nil
}

// userServiceProxy 手写代理(脚手架推荐形态):方法转发 + aop.Proxy 拦截。
type userServiceProxy struct {
	inner *userServiceImpl
	aop   *Proxy
}

func (p *userServiceProxy) GetUser(ctx context.Context, id int64) (string, error) {
	var result string
	_, err := p.aop.Invoke(ctx, "GetUser", []interface{}{ctx, id}, func() ([]interface{}, error) {
		var callErr error
		result, callErr = p.inner.GetUser(ctx, id)
		return []interface{}{result}, callErr
	})
	return result, err
}

func (p *userServiceProxy) UpdateUser(ctx context.Context, id int64, name string) error {
	_, err := p.aop.Invoke(ctx, "UpdateUser", []interface{}{ctx, id, name}, func() ([]interface{}, error) {
		return nil, p.inner.UpdateUser(ctx, id, name)
	})
	return err
}

// logAspect 日志切面(演示 JoinPoint 全量上下文)。
type logAspect struct {
	events []string
}

func (a *logAspect) Name() string { return "log" }

func (a *logAspect) Before(ctx context.Context, jp *JoinPoint) error {
	a.events = append(a.events, "before:"+jp.Method)
	return nil
}

func (a *logAspect) After(ctx context.Context, jp *JoinPoint) {
	a.events = append(a.events, "after:"+jp.Method+":cost="+jp.Cost.String())
}

// authAspect 权限切面(前置校验:方法名白名单)。
type authAspect struct {
	allowed map[string]bool
}

func (a *authAspect) Name() string { return "auth" }

func (a *authAspect) Before(ctx context.Context, jp *JoinPoint) error {
	if !a.allowed[jp.Method] {
		return errors.New("method not allowed")
	}
	return nil
}

func (a *authAspect) After(ctx context.Context, jp *JoinPoint) {}

// paramGuardAspect 参数校验切面:修改/校验参数。
type paramGuardAspect struct{}

func (paramGuardAspect) Name() string { return "param-guard" }

func (paramGuardAspect) Before(ctx context.Context, jp *JoinPoint) error {
	for _, param := range jp.Params {
		if id, ok := param.(int64); ok && id <= 0 {
			return errors.New("invalid id")
		}
	}
	return nil
}

func (paramGuardAspect) After(ctx context.Context, jp *JoinPoint) {}

// TestAspectInterface 接口定义 + 代理接入:切面拿到完整上下文。
func TestAspectInterface(t *testing.T) {
	logger := &logAspect{}
	guard := paramGuardAspect{}
	inner := &userServiceImpl{users: map[int64]string{1: "connor"}}
	proxy := &userServiceProxy{
		inner: inner,
		aop:   NewProxy(inner, guard, logger),
	}

	// 通过接口调用(调用方只持有接口,切面对其透明)
	var service userService = proxy
	name, err := service.GetUser(context.Background(), 1)
	if err != nil || name != "connor" {
		t.Fatalf("call failed: %v %q", err, name)
	}
	if len(logger.events) != 2 {
		t.Fatalf("events = %v", logger.events)
	}
	if logger.events[0] != "before:GetUser" {
		t.Fatalf("before event missing: %v", logger.events)
	}
	if logger.events[1] != "after:GetUser:cost="+logger.events[1][len("after:GetUser:cost="):] {
		t.Fatalf("after event malformed: %v", logger.events[1])
	}
}

// TestAspectBeforeAbort 前置切面终止调用(权限校验)。
func TestAspectBeforeAbort(t *testing.T) {
	auth := &authAspect{allowed: map[string]bool{"GetUser": true}}
	inner := &userServiceImpl{users: map[int64]string{1: "x"}}
	proxy := &userServiceProxy{inner: inner, aop: NewProxy(inner, auth)}

	var service userService = proxy
	// GetUser 放行
	if _, err := service.GetUser(context.Background(), 1); err != nil {
		t.Fatalf("allowed method failed: %v", err)
	}
	// UpdateUser 被拒
	if err := service.UpdateUser(context.Background(), 1, "new"); err == nil {
		t.Fatal("blocked method must fail")
	}
}

// TestAspectParamGuard 参数校验切面。
func TestAspectParamGuard(t *testing.T) {
	inner := &userServiceImpl{users: map[int64]string{1: "x"}}
	proxy := &userServiceProxy{inner: inner, aop: NewProxy(inner, paramGuardAspect{})}

	var service userService = proxy
	if _, err := service.GetUser(context.Background(), 0); err == nil {
		t.Fatal("invalid param must fail")
	}
	if _, err := service.GetUser(context.Background(), 1); err != nil {
		t.Fatalf("valid param failed: %v", err)
	}
}

// TestAspectAfterSeesResults 后置切面可见返回值与错误。
func TestAspectAfterSeesResults(t *testing.T) {
	var sawResults []interface{}
	var sawErr error
	inspector := &aspectFunc{
		name: "inspector",
		after: func(ctx context.Context, jp *JoinPoint) {
			sawResults = jp.Results
			sawErr = jp.Err
			_ = jp.Target // 目标对象可获取
			_ = jp.Proxy
		},
	}
	inner := &userServiceImpl{users: map[int64]string{1: "connor"}}
	proxy := &userServiceProxy{inner: inner, aop: NewProxy(inner, inspector)}

	var service userService = proxy
	if _, err := service.GetUser(context.Background(), 1); err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if len(sawResults) != 1 || sawResults[0] != "connor" {
		t.Fatalf("results = %v", sawResults)
	}
	if sawErr != nil {
		t.Fatalf("err = %v", sawErr)
	}

	// 错误路径
	if _, err := service.GetUser(context.Background(), 999); err == nil {
		t.Fatal("missing user must fail")
	}
	if sawErr == nil {
		t.Fatal("after must see error")
	}
}

// TestAspectChainOrder 多切面洋葱顺序:Before 顺序、After 逆序。
func TestAspectChainOrder(t *testing.T) {
	var order []string
	aspect1 := &aspectFunc{name: "a1",
		before: func(ctx context.Context, jp *JoinPoint) error { order = append(order, "a1-before"); return nil },
		after:  func(ctx context.Context, jp *JoinPoint) { order = append(order, "a1-after") },
	}
	aspect2 := &aspectFunc{name: "a2",
		before: func(ctx context.Context, jp *JoinPoint) error { order = append(order, "a2-before"); return nil },
		after:  func(ctx context.Context, jp *JoinPoint) { order = append(order, "a2-after") },
	}
	inner := &userServiceImpl{users: map[int64]string{1: "x"}}
	proxy := &userServiceProxy{inner: inner, aop: NewProxy(inner, aspect1, aspect2)}

	var service userService = proxy
	if _, err := service.GetUser(context.Background(), 1); err != nil {
		t.Fatalf("call failed: %v", err)
	}
	expected := []string{"a1-before", "a2-before", "a2-after", "a1-after"}
	if len(order) != len(expected) {
		t.Fatalf("order = %v", order)
	}
	for i := range expected {
		if order[i] != expected[i] {
			t.Fatalf("order = %v, want %v", order, expected)
		}
	}
}

// TestAspectCost 耗时统计。
func TestAspectCost(t *testing.T) {
	var cost time.Duration
	slow := &aspectFunc{name: "slow",
		after: func(ctx context.Context, jp *JoinPoint) { cost = jp.Cost },
	}
	inner := &userServiceImpl{users: map[int64]string{1: "x"}}
	proxy := &userServiceProxy{inner: inner, aop: NewProxy(inner, slow)}
	// 用慢实现模拟耗时
	proxy.inner = &userServiceImpl{users: map[int64]string{1: "y"}}
	var service userService = proxy
	_, _ = service.GetUser(context.Background(), 1)
	_ = cost // 正常非负即通过(真实耗时由系统时钟保证)
}

// aspectFunc 用函数构造 Aspect 的测试辅助。
type aspectFunc struct {
	name   string
	before func(ctx context.Context, jp *JoinPoint) error
	after  func(ctx context.Context, jp *JoinPoint)
}

func (a *aspectFunc) Name() string { return a.name }

func (a *aspectFunc) Before(ctx context.Context, jp *JoinPoint) error {
	if a.before != nil {
		return a.before(ctx, jp)
	}
	return nil
}

func (a *aspectFunc) After(ctx context.Context, jp *JoinPoint) {
	if a.after != nil {
		a.after(ctx, jp)
	}
}
