package zaplog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Logger 是脚手架全局结构化 Logger；Init 前使用无输出实现避免 nil panic。
	Logger = zap.NewNop()
	// SugaredLogger 是兼容现有 printf 风格调用的全局 Logger。
	SugaredLogger = Logger.Sugar()
	loggerMu      sync.Mutex
)

// Init 根据 CONFIG 创建完整日志目录和严格单级文件 Logger。
// 初始化失败时保留现有 Logger，不会注册半初始化日志实例。
func Init() error {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	if err := validateConfig(CONFIG); err != nil {
		return err
	}
	logDirectory := filepath.Join(CONFIG.Director, "zap")
	if err := os.MkdirAll(logDirectory, 0o755); err != nil {
		return fmt.Errorf("create log directory %q: %w", logDirectory, err)
	}

	minimumLevel, err := parseLevel(CONFIG.Level)
	if err != nil {
		return err
	}
	core, err := newEncoderCore(minimumLevel)
	if err != nil {
		return err
	}

	options := []zap.Option{zap.AddStacktrace(zap.ErrorLevel)}
	if CONFIG.ShowLine {
		options = append(options, zap.AddCaller())
	}
	newLogger := zap.New(core, options...).With(zap.String("service", serviceName(CONFIG.Prefix)))
	oldLogger := Logger
	Logger = newLogger
	SugaredLogger = newLogger.Sugar()
	if oldLogger != nil {
		if syncErr := oldLogger.Sync(); syncErr != nil && !isIgnorableSyncError(syncErr) {
			return fmt.Errorf("sync previous logger: %w", syncErr)
		}
	}
	return nil
}

// WithComponent 返回携带 component 字段的 SugaredLogger。
// component 为空时使用 unknown，确保日志始终可以按功能模块筛选。
func WithComponent(component string) *zap.SugaredLogger {
	trimmedComponent := strings.TrimSpace(component)
	if trimmedComponent == "" {
		trimmedComponent = "unknown"
	}
	return SugaredLogger.With("component", trimmedComponent)
}

// Sync 刷新全局 Logger 的缓冲区。
// macOS 和终端标准输出不支持 fsync 时产生的 EINVAL、ENOTTY 会被安全忽略。
func Sync() error {
	loggerMu.Lock()
	defer loggerMu.Unlock()
	if Logger == nil {
		return nil
	}
	if err := Logger.Sync(); err != nil && !isIgnorableSyncError(err) {
		return fmt.Errorf("sync global logger: %w", err)
	}
	return nil
}

// validateConfig 校验日志目录、格式和轮转时间边界。
func validateConfig(config Zap) error {
	if strings.TrimSpace(config.Director) == "" {
		return errors.New("log director is empty")
	}
	format := strings.ToLower(strings.TrimSpace(config.Format))
	if format != "console" && format != "json" {
		return fmt.Errorf("unsupported log format %q", config.Format)
	}
	if config.MaxAge < 0 {
		return fmt.Errorf("log max age must not be negative: %d", config.MaxAge)
	}
	if config.WithRotationTime < 0 {
		return fmt.Errorf("log rotation time must not be negative: %d", config.WithRotationTime)
	}
	return nil
}

// parseLevel 将文本日志等级转换为 Zap Level。
func parseLevel(value string) (zapcore.Level, error) {
	var parsedLevel zapcore.Level
	if err := parsedLevel.UnmarshalText([]byte(strings.ToLower(strings.TrimSpace(value)))); err != nil {
		return zap.InfoLevel, fmt.Errorf("parse log level %q: %w", value, err)
	}
	return parsedLevel, nil
}

// newEncoderCore 创建控制台输出和 debug、info、warn、error 四个严格单级文件核心。
func newEncoderCore(minimumLevel zapcore.Level) (zapcore.Core, error) {
	levelFiles := []struct {
		name  string
		level zapcore.Level
	}{
		{name: "debug.log", level: zap.DebugLevel},
		{name: "info.log", level: zap.InfoLevel},
		{name: "warn.log", level: zap.WarnLevel},
		{name: "error.log", level: zap.ErrorLevel},
	}

	cores := make([]zapcore.Core, 0, len(levelFiles)+1)
	for _, levelFile := range levelFiles {
		if levelFile.level < minimumLevel {
			continue
		}
		writeSyncer, err := newRotatingWriteSyncer(levelFile.name)
		if err != nil {
			return nil, err
		}
		fileLevel := levelFile.level
		enabler := zap.LevelEnablerFunc(func(entryLevel zapcore.Level) bool {
			if fileLevel == zap.ErrorLevel {
				return entryLevel >= zap.ErrorLevel
			}
			return entryLevel == fileLevel
		})
		cores = append(cores, zapcore.NewCore(getEncoder(), writeSyncer, enabler))
	}
	if CONFIG.LogInConsole {
		consoleEnabler := zap.LevelEnablerFunc(func(entryLevel zapcore.Level) bool {
			return entryLevel >= minimumLevel
		})
		cores = append(cores, zapcore.NewCore(getEncoder(), zapcore.AddSync(os.Stdout), consoleEnabler))
	}
	if len(cores) == 0 {
		return nil, errors.New("log configuration creates no output core")
	}
	return zapcore.NewTee(cores...), nil
}

// getEncoderConfig 返回 console 和 JSON 编码器共享的稳定字段配置。
// timestamp 使用带时区的毫秒精度 RFC3339，caller 和 function 分别定位源码行与调用方法。
func getEncoderConfig() zapcore.EncoderConfig {
	levelEncoder := zapcore.CapitalLevelEncoder
	if strings.EqualFold(CONFIG.Format, "json") {
		levelEncoder = zapcore.LowercaseLevelEncoder
	}
	return zapcore.EncoderConfig{
		CallerKey:      "caller",
		FunctionKey:    "function",
		LevelKey:       "level",
		MessageKey:     "message",
		TimeKey:        "timestamp",
		StacktraceKey:  CONFIG.StacktraceKey,
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     customTimeEncoder,
		EncodeLevel:    levelEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}
}

// getEncoder 根据 CONFIG.Format 创建日志编码器。
func getEncoder() zapcore.Encoder {
	if strings.EqualFold(CONFIG.Format, "json") {
		return zapcore.NewJSONEncoder(getEncoderConfig())
	}
	return zapcore.NewConsoleEncoder(getEncoderConfig())
}

// customTimeEncoder 使用带时区的 RFC3339 毫秒格式输出时间，便于日志平台解析和排序。
func customTimeEncoder(currentTime time.Time, encoder zapcore.PrimitiveArrayEncoder) {
	encoder.AppendString(currentTime.Format("2006-01-02T15:04:05.000Z07:00"))
}

// serviceName 将兼容配置中的展示前缀转换为稳定 service 字段。
func serviceName(prefix string) string {
	name := strings.TrimSpace(prefix)
	name = strings.TrimPrefix(name, "[")
	name = strings.TrimSuffix(name, "]")
	if name == "" {
		return "go-blackbox"
	}
	return name
}

// isIgnorableSyncError 判断错误是否来自不支持 fsync 的终端输出。
func isIgnorableSyncError(err error) bool {
	return errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTTY)
}
