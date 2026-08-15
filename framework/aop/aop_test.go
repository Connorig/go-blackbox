package aop

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestAroundLogsCost 环绕切面:耗时统计 + 参数/结果可见。
func TestAroundLogsCost(t *testing.T) {
	original := func(ctx context.Context, id int64) (string, error) {
		time.Sleep(5 * time.Millisecond)
		return "user-" + string(rune('0'+id)), nil
	}

	var cost time.Duration
	var sawParam int64
	wrapped := Around(original, func(ctx context.Context, params []interface{}, next func() ([]interface{}, error)) ([]interface{}, error) {
		start := time.Now()
		sawParam = params[1].(int64)
		results, err := next()
		cost = time.Since(start)
		return results, err
	}).(func(context.Context, int64) (string, error))

	result, err := wrapped(context.Background(), 7)
	if err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if result != "user-7" {
		t.Fatalf("result = %q", result)
	}
	if sawParam != 7 {
		t.Fatalf("param = %d", sawParam)
	}
	if cost < 5*time.Millisecond {
		t.Fatalf("cost too small: %s", cost)
	}
}

// TestBeforeAborts 前置切面返回 error 终止调用。
func TestBeforeAborts(t *testing.T) {
	called := false
	original := func(ctx context.Context, id int64) (string, error) {
		called = true
		return "ok", nil
	}
	wrapped := Before(original, func(ctx context.Context, params []interface{}) error {
		if params[1].(int64) <= 0 {
			return errors.New("invalid id")
		}
		return nil
	}).(func(context.Context, int64) (string, error))

	if _, err := wrapped(context.Background(), 0); err == nil {
		t.Fatal("must be aborted")
	}
	if called {
		t.Fatal("target must not be called when before hook fails")
	}
	// 合法参数放行
	result, err := wrapped(context.Background(), 5)
	if err != nil || result != "ok" {
		t.Fatalf("valid call failed: %v %q", err, result)
	}
	if !called {
		t.Fatal("target must be called")
	}
}

// TestAfterSeesResults 后置切面:看到结果与错误。
func TestAfterSeesResults(t *testing.T) {
	original := func(ctx context.Context, name string) (int, error) {
		return len(name), nil
	}
	var sawResult int
	var sawErr error
	wrapped := After(original, func(ctx context.Context, params []interface{}, results []interface{}, err error) {
		if len(results) > 0 {
			sawResult = results[0].(int)
		}
		sawErr = err
	}).(func(context.Context, string) (int, error))

	result, err := wrapped(context.Background(), "hello")
	if err != nil || result != 5 {
		t.Fatalf("call failed: %v %d", err, result)
	}
	if sawResult != 5 {
		t.Fatalf("after hook saw result = %d", sawResult)
	}
	if sawErr != nil {
		t.Fatalf("after hook saw err = %v", sawErr)
	}
}

// TestChain 多个切面组合:Before + Around + After。
func TestChain(t *testing.T) {
	order := []string{}
	original := func(ctx context.Context, x int) (int, error) {
		order = append(order, "target")
		return x * 2, nil
	}
	var fn interface{} = original
	fn = Before(fn, func(ctx context.Context, params []interface{}) error {
		order = append(order, "before")
		return nil
	})
	fn = Around(fn, func(ctx context.Context, params []interface{}, next func() ([]interface{}, error)) ([]interface{}, error) {
		order = append(order, "around-in")
		results, err := next()
		order = append(order, "around-out")
		return results, err
	})
	fn = After(fn, func(ctx context.Context, params []interface{}, results []interface{}, err error) {
		order = append(order, "after")
	})
	wrapped := fn.(func(context.Context, int) (int, error))

	result, err := wrapped(context.Background(), 21)
	if err != nil || result != 42 {
		t.Fatalf("call failed: %v %d", err, result)
	}
	expected := []string{"around-in", "before", "target", "around-out", "after"}
	// 语义:After(Around(Before(target))),最外层先执行
	if len(order) != len(expected) {
		t.Fatalf("order = %v", order)
	}
	for i := range expected {
		if order[i] != expected[i] {
			t.Fatalf("order = %v, want %v", order, expected)
		}
	}
}

// TestNoErrorReturn 无 error 返回的函数。
func TestNoErrorReturn(t *testing.T) {
	original := func(name string) string {
		return "hi " + name
	}
	wrapped := Around(original, func(ctx context.Context, params []interface{}, next func() ([]interface{}, error)) ([]interface{}, error) {
		return next()
	}).(func(string) string)
	if result := wrapped("go"); result != "hi go" {
		t.Fatalf("result = %q", result)
	}
}

// TestConcurrentSafety 并发调用安全。
func TestConcurrentSafety(t *testing.T) {
	original := func(ctx context.Context, n int) (int, error) {
		return n + 1, nil
	}
	wrapped := Around(original, func(ctx context.Context, params []interface{}, next func() ([]interface{}, error)) ([]interface{}, error) {
		return next()
	}).(func(context.Context, int) (int, error))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if result, err := wrapped(context.Background(), i); err != nil || result != i+1 {
				t.Errorf("call %d failed: %v %d", i, err, result)
			}
		}(i)
	}
	wg.Wait()
}
