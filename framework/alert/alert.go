// Package alert 提供监控告警:轮询采集资源指标,水位超阈值时推送到
// 企业微信/钉钉/飞书机器人 webhook,支持连续触发确认、告警去重与恢复通知。
//
// 与 framework/monitor 配合:
//
//	collector := monitor.NewCollector()
//	watcher := alert.NewWatcher(alert.Config{
//	    Interval: 15 * time.Second,          // 轮询间隔
//	    Collector: collector,                // 指标来源
//	    Notifiers: []alert.Notifier{alert.NewWeComWebhook("https://qyapi.weixin.qq.com/...")},
//	    Rules: []alert.Rule{
//	        alert.CPUUsage(90, 3),     // CPU > 90% 连续 3 次告警
//	        alert.MemoryUsage(85, 3),
//	        alert.DiskUsage(85, 3),
//	    },
//	})
//	watcher.Start(ctx) // 或 go watcher.Start(ctx) 异步
package alert

import (
	"context"
	"strconv"
	"time"

	"github.com/Connorig/go-blackbox/framework/monitor"
)

// Level 告警级别。
type Level string

const (
	// LevelWarning 警告(默认阈值场景)。
	LevelWarning Level = "warning"
	// LevelCritical 严重(高阈值场景)。
	LevelCritical Level = "critical"
	// LevelRecover 恢复。
	LevelRecover Level = "recover"
)

// Message 告警消息(传给 Notifier)。
type Message struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Level   Level  `json:"level"`
}

// Notifier 通知器接口(企业微信/钉钉/飞书/自定义)。
type Notifier interface {
	// Name 通知渠道名(日志/调试用)。
	Name() string
	// Notify 发送一条告警消息。
	Notify(ctx context.Context, message Message) error
}

// Rule 告警规则。
type Rule struct {
	// Name 规则名(如 "cpu"、"memory")。
	Name string
	// Threshold 触发阈值(使用率百分比 0-100)。
	Threshold float64
	// Consecutive 连续多少次采样超阈值才触发告警(防抖动)。
	Consecutive int
	// Check 从快照提取指标值(0-100);nil 时按 Metric 字段读取。
	Check func(stats *monitor.Stats) float64
	// Level 告警级别(默认 warning)。
	Level Level
}

// CPUUsage 创建 CPU 使用率规则。
func CPUUsage(threshold float64, consecutive int) Rule {
	return Rule{
		Name:        "cpu",
		Threshold:   threshold,
		Consecutive: consecutive,
		Level:       LevelWarning,
		Check: func(stats *monitor.Stats) float64 {
			if stats == nil {
				return 0
			}
			return stats.CPU.UsagePercent
		},
	}
}

// MemoryUsage 创建内存使用率规则。
func MemoryUsage(threshold float64, consecutive int) Rule {
	return Rule{
		Name:        "memory",
		Threshold:   threshold,
		Consecutive: consecutive,
		Level:       LevelWarning,
		Check: func(stats *monitor.Stats) float64 {
			if stats == nil {
				return 0
			}
			return stats.Memory.UsagePercent
		},
	}
}

// DiskUsage 创建磁盘使用率规则。
func DiskUsage(threshold float64, consecutive int) Rule {
	return Rule{
		Name:        "disk",
		Threshold:   threshold,
		Consecutive: consecutive,
		Level:       LevelWarning,
		Check: func(stats *monitor.Stats) float64 {
			if stats == nil {
				return 0
			}
			return stats.Disk.UsagePercent
		},
	}
}

// normalize 补齐默认值。
func (r Rule) normalize() Rule {
	if r.Consecutive <= 0 {
		r.Consecutive = 1
	}
	if r.Level == "" {
		r.Level = LevelWarning
	}
	if r.Check == nil {
		r.Check = func(stats *monitor.Stats) float64 { return 0 }
	}
	return r
}

// Config 监视器配置。
type Config struct {
	// Interval 轮询间隔(默认 15 秒)。
	Interval time.Duration
	// Collector 指标采集器(必填)。
	Collector *monitor.Collector
	// Notifiers 通知器列表(至少一个)。
	Notifiers []Notifier
	// Rules 告警规则列表。
	Rules []Rule
	// OnNotify 通知回调(测试/日志用;每条消息发送前调用)。
	OnNotify func(message Message)
}

// Watcher 轮询监视器:周期采集指标,按规则触发告警与恢复通知。
// 去重语义:同一规则处于告警状态时不重复推送;恢复(低于阈值)后推送恢复消息。
type Watcher struct {
	config    Config
	ruleState map[string]*ruleState
}

type ruleState struct {
	alertConsecutive int // 连续超阈值计数
	recoverCount     int // 连续低于阈值计数(告警状态下的恢复判定)
	alerting         bool
}

// NewWatcher 创建监视器。
func NewWatcher(config Config) *Watcher {
	if config.Interval <= 0 {
		config.Interval = 15 * time.Second
	}
	normalized := make([]Rule, 0, len(config.Rules))
	for _, rule := range config.Rules {
		normalized = append(normalized, rule.normalize())
	}
	config.Rules = normalized
	return &Watcher{config: config, ruleState: make(map[string]*ruleState)}
}

// Start 启动轮询,阻塞直到 ctx 取消(业务侧 go watcher.Start(ctx))。
func (w *Watcher) Start(ctx context.Context) {
	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()
	// 首次立即采集一次
	w.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// tick 单次采集与规则评估。
func (w *Watcher) tick(ctx context.Context) {
	stats, err := w.config.Collector.Stats()
	if err != nil && stats == nil {
		return
	}
	for _, rule := range w.config.Rules {
		w.evaluate(ctx, rule, stats)
	}
}

// evaluate 评估单条规则:连续超阈值 → 告警;连续低于阈值 → 恢复。
func (w *Watcher) evaluate(ctx context.Context, rule Rule, stats *monitor.Stats) {
	value := rule.Check(stats)
	state, exists := w.ruleState[rule.Name]
	if !exists {
		state = &ruleState{}
		w.ruleState[rule.Name] = state
	}

	if value >= rule.Threshold {
		// 超阈值:重置恢复计数,累计告警计数
		state.recoverCount = 0
		state.alertConsecutive++
		if !state.alerting && state.alertConsecutive >= rule.Consecutive {
			state.alerting = true
			w.notify(ctx, Message{
				Title:   "告警:" + rule.Name + " 使用率超阈值",
				Content: formatAlert(rule, value),
				Level:   rule.Level,
			})
		}
		return
	}

	if state.alerting {
		// 告警状态下连续低于阈值达到规则次数 → 恢复
		state.alertConsecutive = 0
		state.recoverCount++
		if state.recoverCount >= rule.Consecutive {
			state.recoverCount = 0
			state.alerting = false
			w.notify(ctx, Message{
				Title:   "恢复:" + rule.Name + " 使用率已回落",
				Content: formatRecover(rule, value),
				Level:   LevelRecover,
			})
		}
		return
	}
	// 正常状态:重置计数
	state.alertConsecutive = 0
	state.recoverCount = 0
}

// notify 发送消息到全部通知器。
func (w *Watcher) notify(ctx context.Context, message Message) {
	if w.config.OnNotify != nil {
		w.config.OnNotify(message)
	}
	for _, notifier := range w.config.Notifiers {
		if notifier == nil {
			continue
		}
		if err := notifier.Notify(ctx, message); err != nil {
			// 通知失败不中断其他渠道(记录到日志由调用方处理)
			continue
		}
	}
}

func formatAlert(rule Rule, value float64) string {
	return "指标: " + rule.Name + " 使用率\n当前: " + formatPercent(value) +
		" (阈值 " + formatPercent(rule.Threshold) + ")"
}

func formatRecover(rule Rule, value float64) string {
	return "指标: " + rule.Name + " 使用率\n当前: " + formatPercent(value) +
		" (已低于阈值 " + formatPercent(rule.Threshold) + ")"
}

func formatPercent(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64) + "%"
}
