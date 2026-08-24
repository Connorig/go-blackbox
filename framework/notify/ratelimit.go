package notify

import (
	"context"
	"errors"
	"sync"
	"time"
)

// RateLimiter 通知频控器:按 key(如 channel:target)限制发送频率,
// 用于防短信轰炸、验证码刷接口等场景。基于进程内滑动窗口;
// 多实例部署如需全局频控,建议在 Sender 内接入 Redis 计数(可扩展)。
type RateLimiter struct {
	mu     sync.Mutex
	window time.Duration
	max    int
	hits   map[string][]time.Time
}

// NewRateLimiter 创建频控器:window 窗口内同一 key 最多 max 次。
// window<=0 或 max<=0 时频控关闭(Allow 恒 true)。
func NewRateLimiter(window time.Duration, max int) *RateLimiter {
	return &RateLimiter{
		window: window,
		max:    max,
		hits:   make(map[string][]time.Time),
	}
}

// Allow 判断 key 是否允许发送(滑动窗口内未超限)。
// key 建议格式 "channel:target"(如 "sms:13800138000"),由调用方拼接。
func (r *RateLimiter) Allow(key string) bool {
	if r == nil || r.window <= 0 || r.max <= 0 {
		return true
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := now.Add(-r.window)
	history := r.hits[key]
	kept := history[:0]
	for _, timestamp := range history {
		if timestamp.After(cutoff) {
			kept = append(kept, timestamp)
		}
	}
	if len(kept) >= r.max {
		r.hits[key] = kept
		return false
	}
	r.hits[key] = append(kept, now)
	return true
}

// AllowSend 频控包装:不通过时返回明确错误(业务可识别并提示"发送过于频繁")。
func (r *RateLimiter) AllowSend(ctx context.Context, channel, target string, content Content) error {
	if r == nil {
		return nil
	}
	if !r.Allow(channel + ":" + target) {
		return errors.New("notify: rate limit exceeded for " + channel + ":" + target)
	}
	return nil
}

// Clean 清理全部 key 的过期记录(窗口滑动后自动失效;可周期性调用)。
func (r *RateLimiter) Clean() {
	if r == nil {
		return
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := now.Add(-r.window)
	for key, history := range r.hits {
		kept := history[:0]
		for _, timestamp := range history {
			if timestamp.After(cutoff) {
				kept = append(kept, timestamp)
			}
		}
		if len(kept) == 0 {
			delete(r.hits, key)
		} else {
			r.hits[key] = kept
		}
	}
}
