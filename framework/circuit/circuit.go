// Package circuit 提供熔断器(参考 gobreaker/业界通用设计):
//
//	closed(正常) → open(熔断) → half-open(试探) → closed(恢复)
//
// 用于保护对第三方/下游服务的调用:失败率达到阈值时快速失败,
// 避免故障扩散(雪崩);冷却期后放行试探请求,成功即恢复。
//
// 与 thirdparty 客户端集成:
//
//	client := thirdparty.NewClient(thirdparty.Config{
//	    BaseURL: "https://sms.partner.com",
//	    Signer:  thirdparty.NewHMACSigner("key", "secret"),
//	    Breaker: circuit.New(circuit.DefaultConfig()), // 熔断保护
//	})
package circuit

import (
	"errors"
	"sync"
	"time"
)

// State 熔断器状态。
type State int

const (
	// StateClosed 关闭:请求正常放行,统计失败率。
	StateClosed State = iota
	// StateOpen 打开:快速失败,不发起真实请求。
	StateOpen
	// StateHalfOpen 半开:放行少量试探请求,判断服务是否恢复。
	StateHalfOpen
)

// String 状态名。
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrOpen 熔断器打开时快速失败返回的错误。
var ErrOpen = errors.New("circuit breaker is open")

// Config 熔断器配置。
type Config struct {
	// FailureThreshold 失败率阈值(0-1):窗口内失败请求占比达到该值触发熔断。
	// 默认 0.5。
	FailureThreshold float64
	// MinRequests 触发评估的最小请求数:窗口内请求数不足时不触发熔断。
	// 默认 10。
	MinRequests int
	// Window 统计窗口时长:每窗口评估一次失败率。
	// 默认 10 秒。
	Window time.Duration
	// Cooldown 熔断打开后保持时长,之后进入半开试探。
	// 默认 10 秒。
	Cooldown time.Duration
	// HalfOpenMaxRequests 半开状态放行的试探请求数。
	// 默认 1。
	HalfOpenMaxRequests int
}

// DefaultConfig 返回推荐配置(失败率 50%、最小 10 请求、10s 窗口、10s 冷却)。
func DefaultConfig() Config {
	return Config{
		FailureThreshold:    0.5,
		MinRequests:         10,
		Window:              10 * time.Second,
		Cooldown:            10 * time.Second,
		HalfOpenMaxRequests: 1,
	}
}

// normalize 补齐默认值。
func (c Config) normalize() Config {
	def := DefaultConfig()
	if c.FailureThreshold <= 0 || c.FailureThreshold > 1 {
		c.FailureThreshold = def.FailureThreshold
	}
	if c.MinRequests <= 0 {
		c.MinRequests = def.MinRequests
	}
	if c.Window <= 0 {
		c.Window = def.Window
	}
	if c.Cooldown <= 0 {
		c.Cooldown = def.Cooldown
	}
	if c.HalfOpenMaxRequests <= 0 {
		c.HalfOpenMaxRequests = def.HalfOpenMaxRequests
	}
	return c
}

// Breaker 熔断器(并发安全)。
type Breaker struct {
	config Config

	mu       sync.Mutex
	state    State
	windowAt time.Time // 当前统计窗口起点
	success  int
	failure  int
	openedAt time.Time // 进入 open 的时间
	halfOK   int       // 半开期试探请求计数
}

// New 创建熔断器;config 零值时使用默认配置。
func New(config Config) *Breaker {
	return &Breaker{
		config:   config.normalize(),
		state:    StateClosed,
		windowAt: time.Now(),
	}
}

// State 返回当前状态。
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Stats 返回当前统计(诊断/监控用)。
type Stats struct {
	State       State   `json:"state"`
	Success     int     `json:"success"`
	Failure     int     `json:"failure"`
	FailureRate float64 `json:"failure_rate"`
}

// Execute 执行受熔断保护的操作。
// classify 决定错误是否计入失败(nil 时 err != nil 即失败;
// 如 HTTP 4xx 业务错误不应触发熔断,可传入分类函数)。
// 熔断打开时返回 ErrOpen,不执行 fn。
func (b *Breaker) Execute(fn func() error, classify func(err error) bool) error {
	if !b.allow() {
		return ErrOpen
	}
	err := fn()
	if err == nil {
		b.record(true)
		return nil
	}
	fails := classify == nil || classify(err)
	b.record(!fails)
	if fails {
		return err
	}
	// 业务类失败(如 4xx)不触发熔断,但返回值
	return err
}

// allow 判断是否放行请求,并处理状态迁移。
func (b *Breaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	switch b.state {
	case StateClosed:
		// 窗口过期则滚动
		if now.Sub(b.windowAt) >= b.config.Window {
			b.rollWindow(now)
		}
		return true
	case StateOpen:
		if now.Sub(b.openedAt) >= b.config.Cooldown {
			// 冷却结束:进入半开
			b.state = StateHalfOpen
			b.halfOK = 0
			b.windowAt = now
			b.success, b.failure = 0, 0
			return true
		}
		return false
	case StateHalfOpen:
		if b.halfOK < b.config.HalfOpenMaxRequests {
			b.halfOK++
			return true
		}
		return false
	default:
		return false
	}
}

// record 记录结果并评估状态迁移。
func (b *Breaker) record(success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		if success {
			b.success++
		} else {
			b.failure++
		}
		// 窗口期满或请求数达标时评估
		now := time.Now()
		if now.Sub(b.windowAt) >= b.config.Window {
			b.evaluateLocked(now)
		} else if b.success+b.failure >= b.config.MinRequests {
			b.evaluateLocked(now)
		}
	case StateHalfOpen:
		if success {
			// 试探成功:恢复 closed
			b.state = StateClosed
			b.windowAt = time.Now()
			b.success, b.failure = 0, 0
		} else {
			// 试探失败:回到 open,重置冷却
			b.state = StateOpen
			b.openedAt = time.Now()
		}
	}
}

// evaluateLocked 评估窗口失败率,决定是否熔断。
func (b *Breaker) evaluateLocked(now time.Time) {
	requests := b.success + b.failure
	if requests < b.config.MinRequests {
		b.rollWindow(now)
		return
	}
	failureRate := float64(b.failure) / float64(requests)
	if failureRate >= b.config.FailureThreshold {
		b.state = StateOpen
		b.openedAt = now
		return
	}
	b.rollWindow(now)
}

// rollWindow 滚动统计窗口。
func (b *Breaker) rollWindow(now time.Time) {
	b.windowAt = now
	b.success, b.failure = 0, 0
}
