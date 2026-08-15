package aop

import (
	"context"
	"time"
)

// Aspect 面向切面接口(对标 Spring AOP @Aspect):
// 实现该接口即成为可接入的切面,业务 Service 通过 aop.Proxy 自动拦截。
//
//	type LogAspect struct{}
//	func (LogAspect) Name() string { return "log" }
//	func (LogAspect) Before(ctx context.Context, jp *aop.JoinPoint) error {
//	    log.Printf("call %s params=%v", jp.Method, jp.Params)
//	    return nil
//	}
//	func (LogAspect) After(ctx context.Context, jp *aop.JoinPoint) {
//	    log.Printf("call %s cost=%s err=%v", jp.Method, jp.Cost, jp.Err)
//	}
type Aspect interface {
	// Name 切面名(日志/去重用)。
	Name() string
	// Before 前置拦截:可读/改 JoinPoint.Params;返回 error 终止调用。
	// 返回 ErrAborted 时调用被终止,不执行目标方法。
	Before(ctx context.Context, joinPoint *JoinPoint) error
	// After 后置回调:目标方法返回后执行(无论成功失败),
	// 可读取 JoinPoint.Results / Err / Cost。
	After(ctx context.Context, joinPoint *JoinPoint)
}

// JoinPoint 连接点上下文:切面中可获取/操作的全部调用信息。
// 与 Spring AOP 的 JoinPoint/ProceedingJoinPoint 对齐。
type JoinPoint struct {
	// Method 被调用方法名(如 "GetUser")。
	Method string
	// Target 目标对象(Service 实现实例)。
	Target interface{}
	// Proxy 代理对象(调用方持有的引用)。
	Proxy interface{}
	// Params 调用参数;Before 中可修改(修改会传递给目标方法与后续切面)。
	Params []interface{}
	// Results 目标方法返回值(不含 error);After 中可读取。
	Results []interface{}
	// Err 目标方法返回的错误;Before 阶段为 nil。
	Err error
	// Start 调用开始时间。
	Start time.Time
	// Cost 调用耗时(After 中有效)。
	Cost time.Duration
}

// Proxy 是面向切面的方法调用执行器:
// 业务 Service 接口的代理实现通过 Invoke 触发切面链,目标方法由 next 闭包执行。
//
//	type UserService interface { GetUser(ctx context.Context, id int64) (*User, error) }
//
//	type userServiceProxy struct {
//	    inner *userServiceImpl
//	    aop   *aop.Proxy
//	}
//	func (p *userServiceProxy) GetUser(ctx context.Context, id int64) (*User, error) {
//	    var result *User
//	    _, err := p.aop.Invoke(ctx, "GetUser", []interface{}{ctx, id}, func() ([]interface{}, error) {
//	        var callErr error
//	        result, callErr = p.inner.GetUser(ctx, id)
//	        return []interface{}{result}, callErr
//	    })
//	    return result, err
//	}
type Proxy struct {
	target  interface{}
	aspects []Aspect
}

// NewProxy 创建代理执行器。
// target 为目标对象(JoinPoint.Target);aspects 按注册顺序执行
// (Before 顺序、After 逆序,洋葱模型,与 Spring 一致)。
func NewProxy(target interface{}, aspects ...Aspect) *Proxy {
	return &Proxy{target: target, aspects: aspects}
}

// AddAspects 运行时追加切面(热插拔切面)。
func (p *Proxy) AddAspects(aspects ...Aspect) {
	p.aspects = append(p.aspects, aspects...)
}

// Invoke 执行切面链并调用目标方法。
// method 为方法名;params 为调用参数;next 为目标方法闭包。
// 任一切面 Before 返回 error 时终止调用,next 不执行,直接返回该错误。
func (p *Proxy) Invoke(ctx context.Context, method string, params []interface{}, next func() ([]interface{}, error)) ([]interface{}, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	joinPoint := &JoinPoint{
		Method: method,
		Target: p.target,
		Proxy:  p,
		Params: params,
		Start:  time.Now(),
	}

	// 前置链:任一失败终止
	for _, aspect := range p.aspects {
		if aspect == nil {
			continue
		}
		if err := aspect.Before(ctx, joinPoint); err != nil {
			joinPoint.Err = err
			joinPoint.Cost = time.Since(joinPoint.Start)
			// 后置链仍需执行(Spring 语义:异常也会走 after)
			for i := len(p.aspects) - 1; i >= 0; i-- {
				if p.aspects[i] != nil {
					p.aspects[i].After(ctx, joinPoint)
				}
			}
			return nil, err
		}
	}

	// 目标方法
	if next != nil {
		results, err := next()
		joinPoint.Results = results
		joinPoint.Err = err
	}
	joinPoint.Cost = time.Since(joinPoint.Start)

	// 后置链:逆序
	for i := len(p.aspects) - 1; i >= 0; i-- {
		if p.aspects[i] != nil {
			p.aspects[i].After(ctx, joinPoint)
		}
	}
	return joinPoint.Results, joinPoint.Err
}
