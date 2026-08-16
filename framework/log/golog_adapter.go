package zaplog

import (
	"io"
	"regexp"
	"strings"

	"github.com/kataras/golog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// GologWriter 返回 io.Writer 适配器,把 golog Printer 直写输出(如 iris 路由表、
// 监听地址)收编进 zap(info 级,component 标识来源)。每行一条,自动剥离 ANSI 颜色码。
// 用法:app.Logger().SetOutput(zaplog.GologWriter("iris"))
// 注意:iris 的路由表有意绕过 golog handler 直写 Printer(源码注释明确),
// 只有 SetOutput 能拦截;Handler 负责结构化日志,Writer 负责 Printer 兜底,不双写。
func GologWriter(component string) io.Writer {
	return &gologWriterAdapter{component: component}
}

// gologWriterAdapter 行缓冲写入器:按行拆分成独立 zap 记录。
type gologWriterAdapter struct {
	component string
	buffer    strings.Builder
}

// Write 实现 io.Writer;跨多次写入的行会累积到换行才输出。
func (w *gologWriterAdapter) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\n' {
			w.flush()
			continue
		}
		w.buffer.WriteByte(b)
	}
	return len(p), nil
}

// flush 输出当前累积行(剥离 ANSI 色码、级别前缀、行首时间戳)。
// iris 的 "Now listening on:" 行被丢弃(空 host 时 iris 输出残缺的 http://[,gbx 在
// webiris 侧用真实地址自打监听日志);中间件行做函数名/文件路径短名压缩。
func (w *gologWriterAdapter) flush() {
	line := strings.TrimSpace(w.buffer.String())
	w.buffer.Reset()
	if line == "" {
		return
	}
	cleaned := stripANSI(line)
	if strings.HasPrefix(cleaned, "Now listening on:") {
		return // 监听信息由 webiris 用真实地址输出,丢弃 iris 的残缺文本
	}
	cleaned = shortenMiddlewareLine(cleaned)
	SugaredLogger.With("component", w.component).Info(cleaned)
}

// middlewareLineRe 匹配 iris 路由表中间件行:• 函数全名 (文件:行号)。
var middlewareLineRe = regexp.MustCompile(`^• (\S+) \(([^)]+)\)$`)

// shortenMiddlewareLine 压缩中间件行的函数全限定名与文件全路径:
// "• github.com/Connorig/go-blackbox/framework/web.ErrorHandler (D:/Codes/.../error_handler.go:15)"
// → "• web.ErrorHandler (error_handler.go:15)"。非中间件行原样返回。
func shortenMiddlewareLine(line string) string {
	matches := middlewareLineRe.FindStringSubmatch(line)
	if len(matches) != 3 {
		return line
	}
	function := shortFunctionName(matches[1])
	source := matches[2]
	if index := strings.LastIndexAny(source, "/\\"); index >= 0 {
		source = source[index+1:]
	}
	return "• " + function + " (" + source + ")"
}

// ansiRe 匹配 ANSI 颜色/样式转义序列。
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// gologPrefixRe 匹配 golog 级别前缀(如 [DBUG]),zap 行已有级别字段,前缀冗余。
var gologPrefixRe = regexp.MustCompile(`^\[(DBUG|INFO|WARN|ERRO|FATA|SUCC)\]\s*`)

// stripANSI 移除字符串中的 ANSI 转义序列与 golog 级别前缀。
func stripANSI(s string) string {
	return gologPrefixRe.ReplaceAllString(ansiRe.ReplaceAllString(s, ""), "")
}

// GologHandler 把 Iris(kataras/golog)日志统一接入 gbx zap 体系。
// 参考 golog 的 Log 结构设计:Message 为 zap message,Fields 键值对展开为
// zap 字段,Stacktrace[0] 作为调用点 caller,级别按 golog→zap 映射。
// 用法:app.Logger().Handle(zaplog.GologHandler("iris"))
func GologHandler(component string) golog.Handler {
	return func(log *golog.Log) (handled bool) {
		level := gologToZapLevel(log.Level)
		fields := make([]zapcore.Field, 0, len(log.Fields)+3)
		fields = append(fields, zap.String("component", component))
		for key, value := range log.Fields {
			fields = append(fields, zap.Any(sanitizeFieldKey(key), value))
		}
		// golog 的调用栈第一帧是日志发出点,转 caller 保留定位能力
		if len(log.Stacktrace) > 0 && log.Stacktrace[0].Source != "" {
			fields = append(fields, zap.String("caller", log.Stacktrace[0].Source))
			if log.Stacktrace[0].Function != "" {
				fields = append(fields, zap.String("function", shortFunctionName(log.Stacktrace[0].Function)))
			}
		}
		if ce := Logger.Check(level, log.Message); ce != nil {
			ce.Write(fields...)
		}
		return true // 已由 zap 处理,不再走 golog 默认输出
	}
}

// gologToZapLevel 映射 golog 级别到 zap 级别。
func gologToZapLevel(level golog.Level) zapcore.Level {
	switch level {
	case golog.DebugLevel:
		return zapcore.DebugLevel
	case golog.WarnLevel:
		return zapcore.WarnLevel
	case golog.ErrorLevel:
		return zapcore.ErrorLevel
	case golog.FatalLevel:
		return zapcore.FatalLevel
	default: // InfoLevel 及未知级别
		return zapcore.InfoLevel
	}
}

// shortFunctionName 把包路径全限定函数名压缩为 pkg.Func 短名,
// 例如 github.com/Connorig/go-blackbox/framework/database.NewNamed -> database.NewNamed。
func shortFunctionName(function string) string {
	dotIndex := lastIndexByte(function, '.')
	slashIndex := lastIndexByte(function, '/')
	if dotIndex < 0 {
		return function
	}
	packageName := ""
	if slashIndex >= 0 && slashIndex < dotIndex {
		packageName = function[slashIndex+1 : dotIndex]
	} else if slashIndex < 0 {
		packageName = function[:dotIndex]
	}
	funcName := function[dotIndex+1:]
	if packageName == "" {
		return funcName
	}
	return packageName + "." + funcName
}

// lastIndexByte 返回 byte 在字符串中最后一次出现的索引,不存在返回 -1。
func lastIndexByte(s string, b byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// sanitizeFieldKey 把 golog Fields 的键规整为 zap 合法字段名。
// 含空格/点号的键直接保留(键值对原样透传,便于检索),仅去空键。
func sanitizeFieldKey(key string) string {
	if key == "" {
		return "field"
	}
	return key
}
