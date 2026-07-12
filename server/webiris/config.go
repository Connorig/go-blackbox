package webiris

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultTimeFormat 是 Web 服务未指定时间格式时使用的默认格式。
	DefaultTimeFormat = "2006-01-02 15:04:05"
	// DefaultLogLevel 是 Iris 框架日志未指定级别时使用的默认级别。
	DefaultLogLevel = "info"
	// DefaultShutdownTimeout 是优雅关闭等待存量 HTTP 请求结束的默认时长。
	DefaultShutdownTimeout = 10 * time.Second
)

// supportedLogLevels 保存 Iris 支持的日志级别，用于在启动前拒绝无效配置。
var supportedLogLevels = map[string]struct{}{
	"disable": {},
	"fatal":   {},
	"error":   {},
	"warn":    {},
	"info":    {},
	"debug":   {},
}

// Config 描述 Iris Web 服务的运行参数。
// Address 必须是 host:port 格式，例如 :9528、127.0.0.1:9528 或 [::1]:9528。
type Config struct {
	Address         string        // Address 是 TCP 监听地址。
	TimeFormat      string        // TimeFormat 控制 Iris 输出时间的格式。
	LogLevel        string        // LogLevel 控制 Iris 框架日志级别。
	ShutdownTimeout time.Duration // ShutdownTimeout 限制优雅关闭的最长等待时间。
}

// normalizeConfig 清理用户输入并补充默认值。
// 地址和日志级别校验失败时直接返回错误，禁止无效配置进入启动阶段。
func normalizeConfig(config Config) (Config, error) {
	config.Address = strings.TrimSpace(config.Address)
	config.TimeFormat = strings.TrimSpace(config.TimeFormat)
	config.LogLevel = strings.ToLower(strings.TrimSpace(config.LogLevel))

	if config.Address == "" {
		return Config{}, errors.New("webiris: address is required")
	}
	if err := validateAddress(config.Address); err != nil {
		return Config{}, err
	}
	if config.TimeFormat == "" {
		config.TimeFormat = DefaultTimeFormat
	}
	if config.LogLevel == "" {
		config.LogLevel = DefaultLogLevel
	}
	if _, ok := supportedLogLevels[config.LogLevel]; !ok {
		return Config{}, fmt.Errorf("webiris: unsupported log level %q", config.LogLevel)
	}
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = DefaultShutdownTimeout
	}

	return config, nil
}

// validateAddress 验证 TCP 监听地址及端口范围。
// 端口 0 被保留给测试和临时服务，由操作系统自动分配空闲端口。
func validateAddress(address string) error {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("webiris: invalid listen address %q: %w", address, err)
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("webiris: listen port must be numeric, address=%q: %w", address, err)
	}
	if portNumber < 0 || portNumber > 65535 {
		return fmt.Errorf("webiris: listen port out of range, address=%q", address)
	}

	return nil
}
