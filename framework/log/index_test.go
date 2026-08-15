package zaplog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"go.uber.org/zap"
)

// TestInitCreatesCompleteLogDirectory 验证 Init 会创建实际写入使用的 zap 子目录。
func TestInitCreatesCompleteLogDirectory(t *testing.T) {
	restore := useTestLogConfig(t, "info")
	t.Cleanup(restore)

	if err := Init(); err != nil {
		t.Fatalf("initialize test logger failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(CONFIG.Director, "zap")); err != nil {
		t.Fatalf("log directory was not created: %v", err)
	}
}

// TestStrictLevelFilesDoNotDuplicateMessages 验证 info 和 warn 消息只写入各自等级文件。
func TestStrictLevelFilesDoNotDuplicateMessages(t *testing.T) {
	restore := useTestLogConfig(t, "debug")
	t.Cleanup(restore)

	if err := Init(); err != nil {
		t.Fatalf("initialize test logger failed: %v", err)
	}
	Logger.Info("info-only-message")
	Logger.Warn("warn-only-message")
	if err := Sync(); err != nil {
		t.Fatalf("sync test logger failed: %v", err)
	}

	infoContent := readTestLogFile(t, "info.log")
	warnContent := readTestLogFile(t, "warn.log")
	if !strings.Contains(infoContent, "info-only-message") || strings.Contains(infoContent, "warn-only-message") {
		t.Fatalf("info log contains unexpected messages: %s", infoContent)
	}
	if !strings.Contains(warnContent, "warn-only-message") || strings.Contains(warnContent, "info-only-message") {
		t.Fatalf("warn log contains unexpected messages: %s", warnContent)
	}
}

// TestWithComponentAddsStructuredField 验证组件 Logger 会附加可检索的 component 字段。
func TestWithComponentAddsStructuredField(t *testing.T) {
	restore := useTestLogConfig(t, "info")
	t.Cleanup(restore)

	CONFIG.Format = "json"
	if err := Init(); err != nil {
		t.Fatalf("initialize JSON test logger failed: %v", err)
	}
	WithComponent("cache").Info("component-message")
	if err := Sync(); err != nil {
		t.Fatalf("sync component logger failed: %v", err)
	}

	content := readTestLogFile(t, "info.log")
	if !strings.Contains(content, `"component":"cache"`) {
		t.Fatalf("component field is missing from log: %s", content)
	}
}

// TestStandardLogFields 验证 JSON 日志包含标准时间、服务、调用文件、函数和消息字段。
func TestStandardLogFields(t *testing.T) {
	restore := useTestLogConfig(t, "info")
	t.Cleanup(restore)

	CONFIG.Format = "json"
	if err := Init(); err != nil {
		t.Fatalf("initialize standard field logger failed: %v", err)
	}
	Logger.Info("standard-field-message")
	if err := Sync(); err != nil {
		t.Fatalf("sync standard field logger failed: %v", err)
	}

	content := readTestLogFile(t, "info.log")
	requiredFragments := []string{
		`"timestamp":"`,
		`"level":"info"`,
		`"service":"test"`,
		`"caller":"log/index_test.go:`,
		`"function":"github.com/Connorig/go-blackbox/framework/log.TestStandardLogFields"`,
		`"message":"standard-field-message"`,
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(content, fragment) {
			t.Fatalf("standard log field %s is missing: %s", fragment, content)
		}
	}
}

// TestServiceNameNormalizesLegacyPrefix 验证旧版方括号前缀会转换为稳定服务名。
func TestServiceNameNormalizesLegacyPrefix(t *testing.T) {
	if actual := serviceName(" [go-blackbox] "); actual != "go-blackbox" {
		t.Fatalf("unexpected normalized service name: %s", actual)
	}
	if actual := serviceName("[]"); actual != "go-blackbox" {
		t.Fatalf("empty service name must use default: %s", actual)
	}
}

// TestInitRejectsInvalidConfiguration 验证非法格式和日志等级会返回错误而不是 panic。
func TestInitRejectsInvalidConfiguration(t *testing.T) {
	restore := useTestLogConfig(t, "invalid-level")
	t.Cleanup(restore)

	if err := Init(); err == nil {
		t.Fatal("invalid log level must return an error")
	}
	CONFIG.Level = "info"
	CONFIG.Format = "xml"
	if err := Init(); err == nil {
		t.Fatal("invalid log format must return an error")
	}
}

// TestIgnorableSyncErrors 验证终端不支持 fsync 的平台错误会被识别为可忽略。
func TestIgnorableSyncErrors(t *testing.T) {
	if !isIgnorableSyncError(syscall.EINVAL) || !isIgnorableSyncError(syscall.ENOTTY) {
		t.Fatal("terminal sync errors must be ignorable")
	}
	if isIgnorableSyncError(errors.New("disk failure")) {
		t.Fatal("unrelated sync errors must not be ignored")
	}
}

// useTestLogConfig 安装隔离的日志配置并返回恢复函数。
func useTestLogConfig(t *testing.T, level string) func() {
	t.Helper()
	oldConfig := CONFIG
	oldLogger := Logger
	oldSugaredLogger := SugaredLogger
	CONFIG = Zap{
		Level:            level,
		Format:           "console",
		Prefix:           "[test]",
		Director:         t.TempDir(),
		ShowLine:         true,
		StacktraceKey:    "stacktrace",
		LogInConsole:     false,
		MaxAge:           24,
		WithRotationTime: 1,
	}
	Logger = zap.NewNop()
	SugaredLogger = Logger.Sugar()
	return func() {
		if err := Close(); err != nil {
			t.Errorf("sync logger during cleanup failed: %v", err)
		}
		CONFIG = oldConfig
		Logger = oldLogger
		SugaredLogger = oldSugaredLogger
	}
}

// readTestLogFile 读取轮转器创建的等级软链接内容。
func readTestLogFile(t *testing.T, filename string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(CONFIG.Director, "zap", filename))
	if err != nil {
		t.Fatalf("read test log file %s failed: %v", filename, err)
	}
	return string(content)
}
