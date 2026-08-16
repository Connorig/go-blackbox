package zaplog

import (
	"context"
	"errors"
	"regexp"
	"time"

	gormlogger "gorm.io/gorm/logger"
)

// GormLoggerConfig 定义 GORM SQL 日志接入 zap 的策略。
// 配置理念吸收 gorm 官方 logger.Config:
// SlowThreshold 慢查询阈值、IgnoreRecordNotFoundError 忽略未找到记录、
// ParameterizedQueries 参数化 SQL(防敏感数据进日志)。
type GormLoggerConfig struct {
	// SlowThreshold 超过该耗时的查询按 Warn 输出(慢查询告警),默认 200ms。
	SlowThreshold time.Duration
	// LogLevel 过滤最低级别,默认 Warn(生产只留慢查询与错误)。
	LogLevel gormlogger.LogLevel
	// IgnoreRecordNotFoundError 忽略 ErrRecordNotFound(常见于 First 探测查询)。
	IgnoreRecordNotFoundError bool
	// ParameterizedQueries 以 $1 占位符隐藏 SQL 参数值。
	ParameterizedQueries bool
}

// DefaultGormLoggerConfig 返回生产友好的 GORM 日志默认配置。
func DefaultGormLoggerConfig() GormLoggerConfig {
	return GormLoggerConfig{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
	}
}

// NewGormLogger 创建接入 gbx zap 体系的 GORM 日志器。
// SQL 详情按级别分流:Info(SQL+参数+行数+耗时,开发期)、
// Warn(慢查询)、Error(SQL 错误),字段结构化(sql/rows/elapsed/component)。
func NewGormLogger(config GormLoggerConfig) gormlogger.Interface {
	if config.SlowThreshold <= 0 {
		config.SlowThreshold = 200 * time.Millisecond
	}
	return &gormZapLogger{config: config}
}

// gormZapLogger 实现 gormlogger.Interface,字段化输出到 zap。
type gormZapLogger struct {
	config GormLoggerConfig
}

// LogMode 调整日志级别,返回新配置的日志器。
func (l *gormZapLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	cloned := *l
	cloned.config.LogLevel = level
	return &cloned
}

// Info 输出 GORM 信息日志。
func (l *gormZapLogger) Info(_ context.Context, message string, args ...interface{}) {
	if l.config.LogLevel >= gormlogger.Info {
		SugaredLogger.With("component", "gorm").Infof(message, args...)
	}
}

// Warn 输出 GORM 告警日志。
func (l *gormZapLogger) Warn(_ context.Context, message string, args ...interface{}) {
	if l.config.LogLevel >= gormlogger.Warn {
		SugaredLogger.With("component", "gorm").Warnf(message, args...)
	}
}

// Error 输出 GORM 错误日志。
func (l *gormZapLogger) Error(_ context.Context, message string, args ...interface{}) {
	if l.config.LogLevel >= gormlogger.Error {
		SugaredLogger.With("component", "gorm").Errorf(message, args...)
	}
}

// Trace 输出 SQL 执行详情:普通 SQL 走 Debug(开发期)、慢查询走 Warn、错误走 Error。
// 字段:sql、rows、elapsed_ms、component;参数化模式用 ? 占位符替代真实字面量。
func (l *gormZapLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.config.LogLevel <= gormlogger.Silent {
		return
	}
	elapsed := time.Since(begin)
	rawSQL, rows := fc()
	sql := rawSQL
	if l.config.ParameterizedQueries {
		sql = parameterizeSQL(rawSQL)
	}
	base := SugaredLogger.With(
		"component", "gorm",
		"elapsed_ms", float64(elapsed.Microseconds())/1000.0,
		"rows", rows,
		"sql", sql,
	)
	switch {
	case err != nil && l.config.LogLevel >= gormlogger.Error && !(errors.Is(err, gormlogger.ErrRecordNotFound) && l.config.IgnoreRecordNotFoundError):
		base.Errorw("SQL execute error", "error", err.Error())
	case elapsed > l.config.SlowThreshold && l.config.SlowThreshold != 0 && l.config.LogLevel >= gormlogger.Warn:
		base.Warnw("slow SQL")
	case l.config.LogLevel >= gormlogger.Info:
		base.Debugw("SQL")
	}
}

// parameterizeSQL 以 ? 占位符替换 SQL 中的字面量(引号内字符串与数字),
// 防止敏感数据进日志(对齐 gorm 官方 ParameterizedQueries 理念)。
func parameterizeSQL(sql string) string {
	return sqlLiteralRe.ReplaceAllString(sql, "?")
}

// sqlLiteralRe 匹配 SQL 字面量:单引号字符串(含 '' 转义)与数字。
var sqlLiteralRe = regexp.MustCompile(`'([^']|'')*'|\b\d+(?:\.\d+)?\b`)
