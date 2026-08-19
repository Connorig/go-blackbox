// Package grayscale 提供灰度路由:按比例或按用户稳定分流新旧版本处理器。
// 场景:新接口灰度发版(5% → 20% → 50% → 100%)、A/B 测试。
// 支持灰度可观测:响应头标记命中版本(X-Gray-Version)与命中统计(Stats)。
package grayscale

import (
	"hash/fnv"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/kataras/iris/v12"
)

// 默认灰度标记响应头。
const DefaultHeaderName = "X-Gray-Version"

// Strategy 灰度策略。
type Strategy struct {
	// Ratio 命中新版本的比例(0~1);1 = 全量新版本,0 = 全量旧版本。
	Ratio float64
	// UserKey 提取用户标识的函数(nil = 按请求随机分流)。
	// 提供后同一用户始终命中同一版本(稳定灰度)。
	UserKey func(ctx iris.Context) string
	// HeaderName 响应头标记名("" 关闭标记;默认 X-Gray-Version)。
	HeaderName string

	newHits uint64
	oldHits uint64
}

// New 创建灰度策略。userKey 可选(传入则用户稳定分流)。
func New(ratio float64, userKey ...func(ctx iris.Context) string) *Strategy {
	strategy := &Strategy{Ratio: ratio, HeaderName: DefaultHeaderName}
	if len(userKey) > 0 {
		strategy.UserKey = userKey[0]
	}
	return strategy
}

// WithHeaderName 设置响应头标记名("" 关闭标记)。链式调用。
func (s *Strategy) WithHeaderName(name string) *Strategy {
	if s != nil {
		s.HeaderName = name
	}
	return s
}

// Hit 判断当前请求是否命中新版本(不计入统计;Route 内部统计)。
func (s *Strategy) Hit(ctx iris.Context) bool {
	if s == nil || s.Ratio <= 0 {
		return false
	}
	if s.Ratio >= 1 {
		return true
	}
	if s.UserKey != nil {
		if key := s.UserKey(ctx); key != "" {
			return float64(hashUser(key)%10000)/10000 < s.Ratio
		}
	}
	return rand.Float64() < s.Ratio
}

// Route 返回分流处理器:命中走 newHandler,否则走 oldHandler。
// 响应头标记命中版本(X-Gray-Version: new|old,可 WithHeaderName 关闭),并计入统计。
// 用法:app.Get("/api/order", gray.Route(newOrderHandler, oldOrderHandler))
func (s *Strategy) Route(newHandler, oldHandler iris.Handler) iris.Handler {
	return func(ctx iris.Context) {
		if s.Hit(ctx) {
			s.markHeader(ctx, "new")
			atomic.AddUint64(&s.newHits, 1)
			newHandler(ctx)
			return
		}
		s.markHeader(ctx, "old")
		atomic.AddUint64(&s.oldHits, 1)
		oldHandler(ctx)
	}
}

// markHeader 写入灰度标记响应头(关闭时跳过)。
func (s *Strategy) markHeader(ctx iris.Context, version string) {
	if s == nil || s.HeaderName == "" {
		return
	}
	ctx.Header(s.HeaderName, version)
}

// Stats 灰度统计快照。
type Stats struct {
	Total   uint64  // 总请求数
	NewHits uint64  // 命中新版本数
	OldHits uint64  // 命中旧版本数
	Ratio   float64 // 实际新版本占比(0~1;无请求时为 0)
}

// Stats 返回灰度命中统计(原子读取)。
func (s *Strategy) Stats() Stats {
	if s == nil {
		return Stats{}
	}
	newHits := atomic.LoadUint64(&s.newHits)
	oldHits := atomic.LoadUint64(&s.oldHits)
	total := newHits + oldHits
	stats := Stats{Total: total, NewHits: newHits, OldHits: oldHits}
	if total > 0 {
		stats.Ratio = float64(newHits) / float64(total)
	}
	return stats
}

// hashUser 用户标识 FNV-1a 哈希(稳定,不受进程重启影响)。
func hashUser(key string) uint32 {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(key))
	return hasher.Sum32()
}

// 初始化随机种子(包级,确保随机分流不重复序列)。
var _ = func() int64 { rand.Seed(time.Now().UnixNano()); return 0 }()
