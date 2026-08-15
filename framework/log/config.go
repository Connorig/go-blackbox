package zaplog

// CONFIG 保存兼容旧调用方的全局日志配置。
// 应用应在启动阶段单线程修改配置，并在 Init 成功后停止写入该变量。
var CONFIG = Zap{
	Level:            "debug",
	Format:           "console",
	Prefix:           "[go-blackbox]",
	Director:         ".",
	LinkName:         "latest_log",
	ShowLine:         true,
	EncodeLevel:      "LowercaseColorLevelEncoder",
	StacktraceKey:    "stacktrace",
	LogInConsole:     true,
	MaxAge:           7 * 24,
	WithRotationTime: 24,
}

// Zap 定义日志编码、输出目录和文件轮转策略。
type Zap struct {
	// Level 是允许输出的最低日志级别。
	Level string `mapstructure:"level" json:"level" yaml:"level"`
	// Format 支持 console 和 json。
	Format string `mapstructure:"format" json:"format" yaml:"format"`
	// Prefix 是兼容旧配置的应用标识，初始化后会规范化为结构化 service 字段。
	Prefix string `mapstructure:"prefix" json:"prefix" yaml:"prefix"`
	// Director 是日志根目录，等级文件写入其 zap 子目录。
	Director string `mapstructure:"director" json:"director" yaml:"director"`
	// LinkName 为旧版配置字段保留；新版每个等级使用独立软链接名称。
	LinkName string `mapstructure:"link-name" json:"link-name" yaml:"link-name"`
	// ShowLine 控制是否记录调用文件和行号。
	ShowLine bool `mapstructure:"show-line" json:"show-line" yaml:"show-line"`
	// EncodeLevel 为旧版编码器名称保留。
	EncodeLevel string `mapstructure:"encode-level" json:"encode-level" yaml:"encode-level"`
	// StacktraceKey 是堆栈字段名称。
	StacktraceKey string `mapstructure:"stacktrace-key" json:"stacktrace-key" yaml:"stacktrace-key"`
	// LogInConsole 控制是否同时输出到标准输出。
	LogInConsole bool `mapstructure:"log-in-console" json:"log-in-console" yaml:"log-in-console"`
	// MaxAge 是轮转日志最大保留小时数。
	MaxAge int `mapstructure:"max-age" json:"max-age" yaml:"max-age"`
	// WithRotationTime 是日志文件轮转间隔小时数。
	WithRotationTime int `mapstructure:"rotation-time" json:"rotation-time" yaml:"rotation-time"`
}
