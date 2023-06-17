package zaplog

import (
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger     = debugLogger()
	logMap     sync.Map
	Default    = "default"
	timezone   = time.FixedZone("CST", 8*3600)
	layoutdate = "2006-01-02"
	layouttime = "2006-01-02 15:04:05"
)

func init() {
	initLogger("test", &LoggerConfig{
		Filename: "test_filename",
		Options: &LoggerOptions{
			MaxBackups: 10,
			Compress:   true,
			Stderr:     true,
		},
	})

}

// LoggerConfig 日志初始化配置
type LoggerConfig struct {
	// Filename 日志名称
	Filename string `json:"filename"`

	// Options 日志选项
	Options *LoggerOptions `json:"options"`
}

// LoggerOptions 日志配置选项
type LoggerOptions struct {
	// MaxSize 当前文件多大时轮替；默认：100MB
	MaxSize int `json:"max_size"`

	// MaxAge 轮替的旧文件最大保留时长；默认：不限
	MaxAge int `json:"max_age"`

	// MaxBackups 轮替的旧文件最大保留数量；默认：不限
	MaxBackups int `json:"max_backups"`

	// Compress 轮替的旧文件是否压缩；默认：不压缩
	Compress bool `json:"compress"`

	// Stderr 是否输出到控制台
	Stderr bool `json:"stderr"`

	// ZapOptions Zap日志选项
	ZapOptions []zap.Option `json:"zap_options"`
}

func newLogger(cfg *LoggerConfig) *zap.Logger {

	if len(cfg.Filename) == 0 {
		return debugLogger(cfg.Options.ZapOptions...)
	}

	c := zap.NewProductionEncoderConfig()

	c.TimeKey = "time"
	c.EncodeTime = MyTimeEncoder
	c.EncodeCaller = zapcore.FullCallerEncoder

	ws := make([]zapcore.WriteSyncer, 0, 2)

	ws = append(ws, zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.Options.MaxSize,
		MaxAge:     cfg.Options.MaxAge,
		MaxBackups: cfg.Options.MaxBackups,
		Compress:   cfg.Options.Compress,
		LocalTime:  true,
	}))

	if cfg.Options.Stderr {
		ws = append(ws, zapcore.Lock(os.Stderr))
	}

	core := zapcore.NewCore(zapcore.NewJSONEncoder(c), zapcore.NewMultiWriteSyncer(ws...), zap.DebugLevel)

	return zap.New(core, cfg.Options.ZapOptions...)
}

func debugLogger(options ...zap.Option) *zap.Logger {
	cfg := zap.NewDevelopmentConfig()

	cfg.DisableCaller = true
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.EncoderConfig.EncodeTime = MyTimeEncoder
	cfg.EncoderConfig.EncodeCaller = zapcore.FullCallerEncoder

	l, _ := cfg.Build(options...)

	return l
}

func initLogger(name string, cfg *LoggerConfig) {
	if cfg.Options == nil {
		cfg.Options = new(LoggerOptions)
	}

	l := newLogger(cfg)

	if name == Default {
		logger = l
	}

	logMap.Store(name, l)
}

// Logger 返回一个日志实例
func Logger(name ...string) *zap.Logger {
	if len(name) == 0 || name[0] == Default {
		return logger
	}

	v, ok := logMap.Load(name[0])

	if !ok {
		return logger
	}

	return v.(*zap.Logger)
}

// MyTimeEncoder 自定义时间格式化
func MyTimeEncoder(t time.Time, e zapcore.PrimitiveArrayEncoder) {
	e.AppendString(t.In(timezone).Format(layouttime))
}
