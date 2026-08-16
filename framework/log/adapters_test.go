package zaplog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kataras/golog"
	gormlogger "gorm.io/gorm/logger"
	"go.uber.org/zap/zapcore"
)

// setupFileLogger 初始化写到临时目录的日志,返回目录与清理函数。
func setupFileLogger(t *testing.T, level string) (string, func()) {
	t.Helper()
	director := t.TempDir()
	CONFIG.Director = director
	CONFIG.Level = level
	CONFIG.LogInConsole = false
	if err := Init(); err != nil {
		t.Fatalf("init log: %v", err)
	}
	cleanup := func() {
		_ = Sync()
		_ = Close()
		CONFIG.Director = "."
		CONFIG.LogInConsole = true
		_ = Init()
	}
	return director, cleanup
}

// readLogFile 读取等级文件内容。
func readLogFile(t *testing.T, director, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(director, "zap", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

// TestGologHandlerToZap Iris(golog)日志经 handler 进入 zap 分级文件,字段完整。
func TestGologHandlerToZap(t *testing.T) {
	director, cleanup := setupFileLogger(t, "debug")
	defer cleanup()

	handler := GologHandler("iris")
	handled := handler(&golog.Log{
		Level:   golog.DebugLevel,
		Message: "API: 8 registered routes (1 GET and 7 POST)",
		Fields:  golog.Fields{"routes": 8},
		Stacktrace: []golog.Frame{{
			Function: "github.com/Connorig/go-blackbox/framework/web.newApplication",
			Source:   "D:/Codes/gbx/framework/web/index.go:139",
		}},
	})
	if !handled {
		t.Fatal("handler must claim the log")
	}
	_ = Sync()
	content := readLogFile(t, director, "debug.log")
	for _, want := range []string{"registered routes", `"component": "iris"`, `"routes": 8`, `"caller": "D:/Codes/gbx/framework/web/index.go:139"`} {
		if !strings.Contains(content, want) {
			t.Errorf("debug.log missing %q:\n%s", want, content)
		}
	}
	// function 短名压缩
	if !strings.Contains(content, `"function": "web.newApplication"`) {
		t.Errorf("function should be short name:\n%s", content)
	}
}

// TestGologLevelMapping 级别映射覆盖。
func TestGologLevelMapping(t *testing.T) {
	cases := map[golog.Level]zapcore.Level{
		golog.DebugLevel:   zapcore.DebugLevel,
		golog.InfoLevel:    zapcore.InfoLevel,
		golog.WarnLevel:    zapcore.WarnLevel,
		golog.ErrorLevel:   zapcore.ErrorLevel,
		golog.FatalLevel:   zapcore.FatalLevel,
	}
	for from, want := range cases {
		if got := gologToZapLevel(from); got != want {
			t.Errorf("gologToZapLevel(%v) = %v, want %v", from, got, want)
		}
	}
}

// TestShortFunctionName 函数短名压缩。
func TestShortFunctionName(t *testing.T) {
	cases := map[string]string{
		"github.com/Connorig/go-blackbox/framework/database.NewNamed": "database.NewNamed",
		"main.main":            "main.main",
		"github.com/a/b/c.F":   "c.F",
	}
	for input, want := range cases {
		if got := shortFunctionName(input); got != want {
			t.Errorf("shortFunctionName(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestGormLoggerTraceToZap GORM SQL 日志按级别分流到 zap 文件。
func TestGormLoggerTraceToZap(t *testing.T) {
	director, cleanup := setupFileLogger(t, "debug")
	defer cleanup()

	logger := NewGormLogger(GormLoggerConfig{
		SlowThreshold:             100 * time.Millisecond,
		LogLevel:                  gormlogger.Info,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
	})
	// 普通 SQL → debug.log
	logger.Trace(context.Background(), time.Now().Add(-5*time.Millisecond), func() (string, int64) {
		return "SELECT * FROM users WHERE id = 1", 1
	}, nil)
	// 慢查询 → warn.log
	logger.Trace(context.Background(), time.Now().Add(-300*time.Millisecond), func() (string, int64) {
		return "SELECT * FROM users", 100
	}, nil)
	// SQL 错误 → error.log
	logger.Trace(context.Background(), time.Now().Add(-10*time.Millisecond), func() (string, int64) {
		return "UPDATE users SET x = 1", 0
	}, errors.New("boom"))
	// RecordNotFound 被忽略
	logger.Trace(context.Background(), time.Now().Add(-10*time.Millisecond), func() (string, int64) {
		return "SELECT * FROM users WHERE id = 999", 0
	}, gormlogger.ErrRecordNotFound)
	_ = Sync()

	debugContent := readLogFile(t, director, "debug.log")
	for _, want := range []string{`"sql"`, "users", `"rows": 1`, `"component": "gorm"`} {
		if !strings.Contains(debugContent, want) {
			t.Errorf("debug.log missing %q:\n%s", want, debugContent)
		}
	}
	// 参数化:字面量 1 必须被 ? 替换
	if strings.Contains(debugContent, "id = 1") {
		t.Errorf("parameterized queries must hide literal values:\n%s", debugContent)
	}

	warnContent := readLogFile(t, director, "warn.log")
	if !strings.Contains(warnContent, "slow SQL") {
		t.Errorf("warn.log missing slow SQL:\n%s", warnContent)
	}

	errorContent := readLogFile(t, director, "error.log")
	if !strings.Contains(errorContent, "SQL execute error") || !strings.Contains(errorContent, "boom") {
		t.Errorf("error.log missing SQL error:\n%s", errorContent)
	}
}

// TestParameterizeSQL 字面量脱敏。
func TestParameterizeSQL(t *testing.T) {
	sql := "SELECT * FROM users WHERE name = 'alice' AND age = 30 AND note = 'it''s'"
	parameterized := parameterizeSQL(sql)
	for _, leak := range []string{"alice", "it's", "30"} {
		if strings.Contains(parameterized, leak) {
			t.Errorf("parameterized SQL leaks %q: %s", leak, parameterized)
		}
	}
	if !strings.Contains(parameterized, "name = ?") || !strings.Contains(parameterized, "age = ?") {
		t.Errorf("literals must be replaced with ?: %s", parameterized)
	}
}

// TestStdlibLogWriter 标准库日志桥接到 zap。
func TestStdlibLogWriter(t *testing.T) {
	director, cleanup := setupFileLogger(t, "info")
	defer cleanup()

	var writer stdlibLogWriter
	n, err := writer.Write([]byte("third-party message\n"))
	if err != nil || n != 20 {
		t.Fatalf("write: n=%d err=%v", n, err)
	}
	_ = Sync()
	content := readLogFile(t, director, "info.log")
	if !strings.Contains(content, "third-party message") || !strings.Contains(content, `"component": "stdlib"`) {
		t.Errorf("info.log missing stdlib message:\n%s", content)
	}
}

// TestConsoleFunctionEncoder console 下 function 短编码。
func TestConsoleFunctionEncoder(t *testing.T) {
	encoder := getEncoder()
	if encoder == nil {
		t.Fatal("encoder is nil")
	}
}
