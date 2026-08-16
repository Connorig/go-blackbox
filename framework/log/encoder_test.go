package zaplog

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TestErrorEncoderIsJSON error 编码器输出结构化 JSON。
func TestErrorEncoderIsJSON(t *testing.T) {
	encoder := getErrorEncoder()
	entry := zapcore.Entry{
		Level:      zapcore.ErrorLevel,
		Time:       time.Now(),
		Message:    "database connect failed",
		Caller:     zapcore.NewEntryCaller(0, "application.starter.go", 123, true),
		Stack:      "goroutine 1 [running]:\ngithub.com/Connorig/go-blackbox.Test()",
		LoggerName: "go-blackbox",
	}
	fields := []zapcore.Field{zap.String("component", "database"), zap.Error(errSample())}
	buf, err := encoder.EncodeEntry(entry, fields)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	output := buf.String()
	t.Logf("error json: %s", output)

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("error log must be valid JSON: %v\n%s", err, output)
	}
	// 全字段完整:时间/级别/消息/调用点/函数/堆栈/组件
	for _, key := range []string{"timestamp", "level", "message", "caller", "function", "stacktrace", "component"} {
		if _, exists := parsed[key]; !exists {
			t.Errorf("error json missing key %q: %s", key, output)
		}
	}
	if parsed["level"] != "error" {
		t.Errorf("level = %v", parsed["level"])
	}
	if parsed["message"] != "database connect failed" {
		t.Errorf("message = %v", parsed["message"])
	}
	// 堆栈文本完整保留(调用链不丢)
	stack, _ := parsed["stacktrace"].(string)
	if !strings.Contains(stack, "goroutine 1") {
		t.Errorf("stacktrace lost: %q", stack)
	}
}

// TestInfoEncoderConsole info 编码器保持 console 人读格式(输出非 JSON)。
func TestInfoEncoderConsole(t *testing.T) {
	encoder := getEncoder()
	entry := zapcore.Entry{Level: zapcore.InfoLevel, Time: time.Now(), Message: "hello", LoggerName: "go-blackbox"}
	buf, err := encoder.EncodeEntry(entry, nil)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	output := buf.String()
	if strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Fatalf("info log must be console format, got: %s", output)
	}
	if !strings.Contains(output, "hello") {
		t.Fatalf("info log must contain message: %s", output)
	}
}

// errSample 示例错误。
func errSample() error {
	return &simpleError{s: "SASL auth failed"}
}

type simpleError struct{ s string }

func (e *simpleError) Error() string { return e.s }
