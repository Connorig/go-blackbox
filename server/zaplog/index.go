package zaplog

import (
	"github.com/snowlyg/helper/dir"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"time"
)

var (
	level         zapcore.Level      // 设置日志打印级别
	Logger        *zap.Logger        // 标准打印
	SugaredLogger *zap.SugaredLogger // 类似于printf
)

func Init() {
	var logger *zap.Logger

	if !dir.IsExist(CONFIG.Director) {
		dir.InsureDir(CONFIG.Director)
	}

	switch CONFIG.Level {
	case "debug":
		level = zap.DebugLevel
	case "info":
		level = zap.InfoLevel
	case "warn":
		level = zap.WarnLevel
	case "error":
		level = zap.ErrorLevel
	case "dpanic":
		level = zap.DPanicLevel
	case "panic":
		level = zap.PanicLevel
	case "fatal":
		level = zap.FatalLevel
	default:
		level = zap.InfoLevel
	}
	// 默认debug、error级别打开链路追踪
	if level == zap.DebugLevel || level == zap.ErrorLevel {
		logger = zap.New(getEncoderCore(), zap.AddStacktrace(level))
	} else {
		logger = zap.New(getEncoderCore())
	}
	// 默认开启链路追踪，不需要打印行
	//if CONFIG.ShowLine {
	//	logger = logger.WithOptions(zap.AddCaller())
	//}
	// 全局标准日志对象
	Logger = logger
	// 功能类似于printf
	SugaredLogger = logger.Sugar()
}

// getEncoderConfig 获取 zapcore.EncoderConfig
func getEncoderConfig() (conf zapcore.EncoderConfig) {
	// 自定义日志级别显示
	customLevelEncoder := func(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString("[" + level.CapitalString() + "]")
	}
	// 自定义文件：行号输出项
	customCallerEncoder := func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
		//enc.AppendString("[" + l.traceId + "]")
		enc.AppendString("[" + caller.TrimmedPath() + "]")
	}

	// 配置日志打印格式
	conf = zapcore.EncoderConfig{
		CallerKey:      "caller_line", // 打印文件名和行数
		LevelKey:       "level_name",
		MessageKey:     "msg",
		TimeKey:        "ts",
		StacktraceKey:  CONFIG.StacktraceKey, //文件链路追踪
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     customTimeEncoder,   // 自定义时间格式
		EncodeLevel:    customLevelEncoder,  // 小写编码器
		EncodeCaller:   customCallerEncoder, // 全路径编码器
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}
	return conf
}

// getEncoder 日志内容输出格式- 控制台形式、JSON形式
func getEncoder() zapcore.Encoder {
	if CONFIG.Format == "json" {
		return zapcore.NewJSONEncoder(getEncoderConfig())
	}
	return zapcore.NewConsoleEncoder(getEncoderConfig())
}

// getEncoderCore
func getEncoderCore() (core zapcore.Core) {

	// 根据当前logger实例到日志级别将内容输入到对应到文件中
	debugSyncer := GetWriteSyncer2("/zap/debug.log")
	infoSyncer := GetWriteSyncer2("/zap/info.log")
	warnSyncer := GetWriteSyncer2("/zap/warn.log")
	errorSyncer := GetWriteSyncer2("/zap/error.log")

	// 实现判断日志等级的interface
	// 动态判断当前日志级别分别打印到不同级别文件中
	debugLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.DebugLevel
	})
	infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.InfoLevel
	})
	warnLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.WarnLevel
	})
	errorLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})

	var syncer zapcore.WriteSyncer
	syncer = zapcore.AddSync(debugSyncer)

	// 日志打印是否输出到控制台
	if CONFIG.LogInConsole {
		// 默认将最大范围级别 DEBUG设置输出内容到控制台中，其他级别都已经包含在内
		syncer = zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(debugSyncer))
	}

	// 创建具体的Logger
	core = zapcore.NewTee(
		zapcore.NewCore(getEncoder(), syncer, debugLevel),
		zapcore.NewCore(getEncoder(), zapcore.AddSync(infoSyncer), infoLevel),
		zapcore.NewCore(getEncoder(), zapcore.AddSync(warnSyncer), warnLevel),
		zapcore.NewCore(getEncoder(), zapcore.AddSync(errorSyncer), errorLevel),
	)
	return
}

func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format(CONFIG.Prefix + " " + "[2006-01-02 15:04:05.000]"))
}
