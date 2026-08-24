package eventbus

import (
	"context"
	"time"
)

// SubscribeRetry 注册带失败重试的订阅(同步模式):
// handler 失败时按指数退避(backoff × 2^attempt)重试,最多 maxRetries 次
// (总尝试 maxRetries+1),超过后返回最后一次错误。
// 适用:依赖外部资源(DB/HTTP)的订阅者,瞬态失败自动恢复。
// 注意:异步总线(Publish 异步投递)下重试语义不适用,退化为普通订阅。
func (b *Bus) SubscribeRetry(event string, handler Handler, maxRetries int, backoff time.Duration) func() {
	if b == nil || handler == nil {
		return func() {}
	}
	if maxRetries < 0 {
		maxRetries = 0
	}
	if backoff <= 0 {
		backoff = 100 * time.Millisecond
	}
	wrapped := func(ctx context.Context, event Event) error {
		var err error
		for attempt := 0; attempt <= maxRetries; attempt++ {
			err = handler(ctx, event)
			if err == nil {
				return nil
			}
			if attempt < maxRetries {
				delay := backoff * time.Duration(1<<attempt)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}
			}
		}
		return err
	}
	return b.Subscribe(event, wrapped)
}
