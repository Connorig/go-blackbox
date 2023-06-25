package zaplog

// CONFIG 基础配置
var CONFIG = Zap{
	Level:    "debug",
	Format:   "console",
	Prefix:   "[go-blackbox]",
	Director: ".",
	LinkName: "latest_log",
	//ShowLine:      true,
	EncodeLevel:      "LowercaseColorLevelEncoder",
	StacktraceKey:    "stacktrace",
	LogInConsole:     true,
	MaxAge:           7 * 24,
	WithRotationTime: 24,
}

type Zap struct {
	Level            string `mapstructure:"level" json:"level" yaml:"level"` //debug ,info,warn,error,panic,fatal
	Format           string `mapstructure:"format" json:"format" yaml:"format"`
	Prefix           string `mapstructure:"prefix" json:"prefix" yaml:"prefix"`
	Director         string `mapstructure:"director" json:"director"  yaml:"director"`
	LinkName         string `mapstructure:"link-name" json:"link-name" yaml:"link-name"`
	ShowLine         bool   `mapstructure:"show-line" json:"show-line" yaml:"show-line"`
	EncodeLevel      string `mapstructure:"encode-level" json:"encode-level" yaml:"encode-level"`
	StacktraceKey    string `mapstructure:"stacktrace-key" json:"stacktrace-key" yaml:"stacktrace-key"`
	LogInConsole     bool   `mapstructure:"log-in-console" json:"log-in-console" yaml:"log-in-console"`
	MaxAge           int
	WithRotationTime int
}
