package zaplog

import (
	"go.uber.org/zap"
	"testing"
)

func TestGetEncoderConfig(t *testing.T) {
	//defer Remove()
	t.Run("test zaplog get encoder config", func(t *testing.T) {
		config := getEncoderConfig()
		if config.StacktraceKey != CONFIG.StacktraceKey {
			t.Errorf("zaplog config stacktracekey want %s but get %s", CONFIG.StacktraceKey, config.StacktraceKey)
		}
	})
}

func TestLog(t *testing.T) {
	sugar := ZAPLOG.Sugar()
	sugar.Debugf("Debug %d", 1)

	desugar := sugar.Desugar()
	desugar.Debug("debug", zap.Any("key", 1))
}
