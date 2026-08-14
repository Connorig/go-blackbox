package zaplog

import (
	"fmt"
	"io"
	stdlog "log"
	"path/filepath"
	"strings"
	"time"

	zaprotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap/zapcore"
)

// newRotatingWriteSyncer 创建指定等级的轮转文件写入器。
// filename 只能是文件名，目录由 CONFIG.Director/zap 统一管理。
func newRotatingWriteSyncer(filename string) (zapcore.WriteSyncer, error) {
	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	if baseName == "" || baseName == "." {
		return nil, fmt.Errorf("log filename %q is invalid", filename)
	}
	logDirectory := filepath.Join(CONFIG.Director, "zap")
	pattern := filepath.Join(logDirectory, baseName+"-%Y%m%d%H.log")
	linkName := filepath.Join(logDirectory, baseName+".log")
	maxAge := time.Duration(CONFIG.MaxAge) * time.Hour
	rotationTime := time.Duration(CONFIG.WithRotationTime) * time.Hour
	if maxAge <= 0 {
		maxAge = 7 * 24 * time.Hour
	}
	if rotationTime <= 0 {
		rotationTime = 24 * time.Hour
	}

	hook, err := zaprotatelogs.New(
		pattern,
		zaprotatelogs.WithLinkName(linkName),
		zaprotatelogs.WithMaxAge(maxAge),
		zaprotatelogs.WithRotationTime(rotationTime),
	)
	if err != nil {
		return nil, fmt.Errorf("create rotating log writer for %s: %w", baseName, err)
	}
	registerLogWriter(hook)
	return zapcore.AddSync(hook), nil
}

// GetWriteSyncer2 保留旧版直接获取 io.Writer 的公开入口。
// Deprecated: 新代码应由 Init 统一创建日志核心并处理初始化错误。
func GetWriteSyncer2(filename string) io.Writer {
	syncer, err := newRotatingWriteSyncer(filename)
	if err != nil {
		stdlog.Printf("create compatibility log writer failed, filename=%s, error=%v", filepath.Base(filename), err)
		return io.Discard
	}
	return syncer
}
