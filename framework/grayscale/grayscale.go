// Package grayscale 提供灰度路由:按比例或按用户稳定分流新旧版本处理器。
// 场景:新接口灰度发版(5% → 20% → 50% → 100%)、A/B 测试。
package grayscale

import (
	"hash/fnv"
	"math/rand"
	"time"

	"github.com/kataras/iris/v12"
)

// Strategy 灰度策略。
type Strategy struct {
	// Ratio 命中新版本的比例(0~1);1 = 全量新版本,0 = 全量旧版本。
	Ratio float64
	// UserKey 提取用户标识的函数(nil = 按请求随机分流)。
	// 提供后同一用户始终命中同一版本(稳定灰度)。
	UserKey func(ctx iris.Context) string
}

// New 创建灰度策略。userKey 可选(传入则用户稳定分流)。
func New(ratio float64, userKey ...func(ctx iris.Context) string) *Strategy {
	strategy := &Strategy{Ratio: ratio}
	if len(userKey) > 0 {
		strategy.UserKey = userKey[0]
	}
	return strategy
}

// Hit 判断当前请求是否命中新版本。
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

// Route 返回分流中间件:命中走 newHandler,否则走 oldHandler。
// 用法:app.Get("/api/order", gray.Route(newOrderHandler, oldOrderHandler))
func (s *Strategy) Route(newHandler, oldHandler iris.Handler) iris.Handler {
	return func(ctx iris.Context) {
		if s.Hit(ctx) {
			newHandler(ctx)
			return
		}
		oldHandler(ctx)
	}
}

// hashUser 用户标识 FNV-1a 哈希(稳定,不受进程重启影响)。
func hashUser(key string) uint32 {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(key))
	return hasher.Sum32()
}

// 初始化随机种子(包级,确保随机分流不重复序列)。
var _ = func() int64 { rand.Seed(time.Now().UnixNano()); return 0 }()
