// Package monitor 提供服务器资源监控组件(对标阿里云 ECS 资源监控):
//   - 采集:内存/CPU/磁盘/负载/主机信息/进程与协程数,跨平台(Linux /proc + Windows x/sys)
//   - 页面:内置自包含 HTML 监控页(进度条 + 趋势曲线 + 自动刷新)
//   - API:JSON 数据接口,支持 token 校验(Auth)与限流(Limit),防接口轰炸
//
// 接入(在 EnableWeb 回调中注册路由):
//
//	builder.EnableWeb(appbox.TimeFormat, ":8080", "info", func(app *iris.Application) {
//	    monitor.Register(app, "/monitor", monitor.Config{
//	        Auth: webiris.Auth(webiris.AuthConfig{Whitelist: []string{"/monitor"}}), // 监控 API 要求登录
//	        RatePerSecond: 5, // 每 IP 5 QPS,防轰炸
//	    })
//	})
//
// 浏览器访问 http://host:8080/monitor 查看监控页;JSON 接口 /monitor/api/stats。
package monitor

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// Version 组件版本标识(随页面展示)。
const Version = "v1.13.0"

// Stats 一次采集快照。
type Stats struct {
	Hostname      string      `json:"hostname"`               // 主机名
	Platform      string      `json:"platform"`               // 平台,如 linux/amd64
	GoVersion     string      `json:"go_version"`             // Go 版本
	Version       string      `json:"version"`                // go-blackbox monitor 组件版本
	Uptime        int64       `json:"uptime_seconds"`         // 系统运行时长(秒)
	ProcessUptime int64       `json:"process_uptime_seconds"` // 进程运行时长(秒)
	Goroutines    int         `json:"goroutines"`             // 当前协程数
	Time          int64       `json:"time"`                   // 采集时间戳(Unix 秒)
	Memory        MemoryStats `json:"memory"`                 // 内存
	CPU           CPUStats    `json:"cpu"`                    // CPU
	Disk          DiskStats   `json:"disk"`                   // 磁盘(根分区)
	Load          LoadStats   `json:"load"`                   // 系统负载(非 Linux 为 0)
}

// MemoryStats 内存统计(字节)。
type MemoryStats struct {
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	UsagePercent float64 `json:"usage_percent"`
}

// CPUStats CPU 统计。
type CPUStats struct {
	UsagePercent float64 `json:"usage_percent"` // 采样间隔内平均使用率(0-100)
}

// DiskStats 磁盘统计(字节,根分区)。
type DiskStats struct {
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	UsagePercent float64 `json:"usage_percent"`
}

// LoadStats 系统负载(1/5/15 分钟)。
type LoadStats struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// percent 计算使用率,除零安全。
func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}

// Collector 资源采集器。CPU 使用率基于两次采样的差值,线程安全。
type Collector struct {
	hostname  string
	platform  string
	startTime time.Time
	cpuLock   chan struct{} // 串行化 CPU 采样缓存(简单互斥)
	lastCPU   *cpuCache
}

type cpuCache struct {
	idle, total uint64
	sampledAt   time.Time
}

// NewCollector 创建采集器。
func NewCollector() *Collector {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown"
	}
	return &Collector{
		hostname:  hostname,
		platform:  runtime.GOOS + "/" + runtime.GOARCH,
		startTime: time.Now(),
		cpuLock:   make(chan struct{}, 1),
	}
}

// Stats 采集一次全量快照。
func (c *Collector) Stats() (*Stats, error) {
	now := time.Now()
	memory, memErr := c.memoryStats()
	cpu, cpuErr := c.cpuStats()
	disk, diskErr := c.diskStats()
	load, loadErr := c.loadStats()
	uptime := c.uptime()

	stats := &Stats{
		Hostname:      c.hostname,
		Platform:      c.platform,
		GoVersion:     runtime.Version(),
		Version:       Version,
		Uptime:        uptime,
		ProcessUptime: int64(now.Sub(c.startTime).Seconds()),
		Goroutines:    runtime.NumGoroutine(),
		Time:          now.Unix(),
		Memory:        memory,
		CPU:           cpu,
		Disk:          disk,
		Load:          load,
	}
	// 任一关键采集失败时返回错误(页面据此提示部分数据不可用)
	for name, err := range map[string]error{"memory": memErr, "cpu": cpuErr, "disk": diskErr, "load": loadErr} {
		if err != nil {
			return stats, fmt.Errorf("monitor: collect %s: %w", name, err)
		}
	}
	return stats, nil
}

// sampleCPU 通用 CPU 采样缓存(平台实现提供 idle/total 原始计数)。
// 首次调用返回 0,之后返回自上次采样以来的平均使用率。线程安全。
func (c *Collector) sampleCPU(idle, total uint64) (CPUStats, error) {
	c.cpuLock <- struct{}{}
	defer func() { <-c.cpuLock }()

	if c.lastCPU == nil {
		c.lastCPU = &cpuCache{idle: idle, total: total, sampledAt: time.Now()}
		return CPUStats{UsagePercent: 0}, nil
	}
	prev := c.lastCPU
	now := time.Now()
	c.lastCPU = &cpuCache{idle: idle, total: total, sampledAt: now}

	totalDelta := total - prev.total
	idleDelta := idle - prev.idle
	usage := 0.0
	if totalDelta > 0 {
		usage = 100 * (1 - float64(idleDelta)/float64(totalDelta))
	}
	return CPUStats{UsagePercent: usage}, nil
}
