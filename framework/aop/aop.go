// Package aop 提供轻量级方法级切面(面向切面编程),对标 Spring AOP 的
// @Before / @After / @Around,用于 Service 层:
//   - 方法调用前后拦截(日志、耗时统计)
//   - 前置校验(权限、参数),返回 error 终止调用
//   - 环绕包装(事务、重试、熔断、上下文注入)
//
// Web 层请使用 webiris 中间件(等同环绕切面:AccessLog/Auth/Limit/ErrorHandler)。
// 本包面向非 Web 场景(Service/Repository 方法)。
//
// 用法:
//
//	// 原始方法(任意签名)
//	getUser := func(ctx context.Context, id int64) (*User, error) { ... }
//
//	// 环绕切面:耗时 + 日志
//	getUser = aop.Around(getUser, func(ctx context.Context, params []interface{},
//	    next func() ([]interface{}, error)) ([]interface{}, error) {
//	    start := time.Now()
//	    results, err := next()
//	    log.Printf("GetUser cost=%s err=%v", time.Since(start), err)
//	    return results, err
//	}).(func(context.Context, int64) (*User, error))
//
//	// 前置校验:返回 error 终止调用
//	getUser = aop.Before(getUser, func(ctx context.Context, params []interface{}) error {
//	    if id := params[1].(int64); id <= 0 {
//	        return errors.New("invalid user id")
//	    }
//	    return nil
//	}).(func(context.Context, int64) (*User, error))
//
// 与 simpleioc 配合:将装饰后的 Service 结构注册进容器,
// 业务调用方无感知(接口签名不变)。
package aop

import (
	"context"
	"errors"
	"reflect"
)

// Context 携带调用信息。
type Context struct {
	// Method 方法名(反射推断,无则空)。
	Method string
	// Params 调用参数(只读,修改不生效)。
	Params []interface{}
}

// BeforeHook 前置切面:在目标方法调用前执行;返回 error 终止调用。
type BeforeHook func(ctx context.Context, params []interface{}) error

// AfterHook 后置切面:在目标方法返回后执行(无论成功失败)。
type AfterHook func(ctx context.Context, params []interface{}, results []interface{}, err error)

// AroundHook 环绕切面:包裹目标方法调用;调用 next() 执行目标方法。
type AroundHook func(ctx context.Context, params []interface{}, next func() ([]interface{}, error)) ([]interface{}, error)

// Before 为目标函数附加前置切面,返回与原函数同签名的新函数(需类型断言使用)。
// hook 返回 error 时终止调用,直接返回该错误(结果切片为空)。
func Before(fn interface{}, hooks ...BeforeHook) interface{} {
	return decorate(fn, func(ctx context.Context, params []interface{}, next func() ([]interface{}, error)) ([]interface{}, error) {
		for _, hook := range hooks {
			if hook == nil {
				continue
			}
			if err := hook(ctx, params); err != nil {
				return nil, err
			}
		}
		return next()
	})
}

// After 为目标函数附加后置切面(记录日志/耗时/结果审计)。
func After(fn interface{}, hooks ...AfterHook) interface{} {
	return decorate(fn, func(ctx context.Context, params []interface{}, next func() ([]interface{}, error)) ([]interface{}, error) {
		results, err := next()
		for _, hook := range hooks {
			if hook == nil {
				continue
			}
			hook(ctx, params, results, err)
		}
		return results, err
	})
}

// Around 为目标函数附加环绕切面(事务/重试/熔断/耗时统计)。
func Around(fn interface{}, hook AroundHook) interface{} {
	return decorate(fn, hook)
}

// decorate 用反射将 fn 包装为同签名函数,调用时执行 hook 链。
func decorate(fn interface{}, hook AroundHook) interface{} {
	if fn == nil {
		return nil
	}
	fnValue := reflect.ValueOf(fn)
	if fnValue.Kind() != reflect.Func {
		panic("aop: target must be a function")
	}
	fnType := fnValue.Type()
	methodName := fnType.Name()
	if methodName == "" {
		methodName = "func"
	}

	return reflect.MakeFunc(fnType, func(args []reflect.Value) []reflect.Value {
		// 参数转换:第一个 context.Context 提取;其余转 interface{}
		params := make([]interface{}, 0, len(args))
		var callCtx context.Context = context.Background()
		for i, arg := range args {
			if i == 0 {
				if ctx, ok := arg.Interface().(context.Context); ok {
					callCtx = ctx
				}
			}
			params = append(params, arg.Interface())
		}

		results, err := hook(callCtx, params, func() ([]interface{}, error) {
			callResults := fnValue.Call(args)
			if len(callResults) == 0 {
				return nil, nil
			}
			// 最后一个返回值是 error 时单独提取
			converted := make([]interface{}, 0, len(callResults))
			var callErr error
			for i, result := range callResults {
				if i == len(callResults)-1 {
					if errValue, ok := result.Interface().(error); ok {
						callErr = errValue
						continue
					}
				}
				converted = append(converted, result.Interface())
			}
			return converted, callErr
		})
		_ = methodName

		// 结果转换回原签名:results 对齐 fnType 的非 error 返回
		return rebuildResults(fnType, results, err)
	}).Interface()
}

// rebuildResults 把 (results []interface{}, err) 还原为原函数返回值列表。
func rebuildResults(fnType reflect.Type, results []interface{}, err error) []reflect.Value {
	outCount := fnType.NumOut()
	out := make([]reflect.Value, outCount)
	resultIndex := 0
	for i := 0; i < outCount; i++ {
		outType := fnType.Out(i)
		if outType == reflect.TypeOf((*error)(nil)).Elem() {
			// error 返回位
			if err != nil {
				out[i] = reflect.ValueOf(err)
			} else {
				out[i] = reflect.Zero(outType)
			}
			continue
		}
		// 数据返回位:results 按序填充,不足补零值
		if resultIndex < len(results) && results[resultIndex] != nil {
			value := reflect.ValueOf(results[resultIndex])
			if value.Type().AssignableTo(outType) {
				out[i] = value
			} else if value.Type().ConvertibleTo(outType) {
				out[i] = value.Convert(outType)
			} else {
				out[i] = reflect.Zero(outType)
			}
		} else {
			out[i] = reflect.Zero(outType)
		}
		resultIndex++
	}
	return out
}

// ErrAborted 由前置切面返回时,调用被终止的通用错误。
var ErrAborted = errors.New("aop: call aborted by before hook")
