package apploader

import (
	"time"
)

// 模块自动配置结构(对标 Spring Boot Auto-Configuration + @ConditionalOnProperty):
// 业务配置结构内嵌 Modules,在 config.toml 的 [modules] 段通过 enabled 开关模块,
// builder.AutoConfigure 按配置自动装配。零值 = 关闭(安全默认,显式开启)。

// Modules 脚手架统一模块配置(业务内嵌)。
//
//	type AppConfig struct {
//	    apploader.Modules `mapstructure:",squash"`
//	    Business          BusinessConfig `mapstructure:"business"`
//	}
type Modules struct {
	Log      LogModule      `mapstructure:"log"`
	Auth     AuthModule     `mapstructure:"auth"`
	Web      WebModule      `mapstructure:"web"`
	Database DatabaseModule `mapstructure:"database"`
	Cache    CacheModule    `mapstructure:"cache"`
	Mongo    MongoModule    `mapstructure:"mongo"`
	Cron     CronModule     `mapstructure:"cron"`
	Admin    AdminModule    `mapstructure:"admin"`
	Monitor  MonitorModule  `mapstructure:"monitor"`
	OpenAPI  OpenAPIModule  `mapstructure:"openapi"`
}

// LogModule 日志模块配置。
type LogModule struct {
	Enabled bool   `mapstructure:"enabled"`
	Level   string `mapstructure:"level"`    // debug/info/warn/error
	OutDir  string `mapstructure:"out_dir"`  // 日志目录,默认 "."
}

// AuthModule JWT 认证参数(密钥仍由业务代码 apptoken.SetSecretKey 注入)。
type AuthModule struct {
	Enabled     bool          `mapstructure:"enabled"`
	AccessTTL   time.Duration `mapstructure:"access_ttl"`   // 访问 token 有效期,默认 30m
	RefreshTTL  time.Duration `mapstructure:"refresh_ttl"`  // 刷新 token 有效期,默认 168h
	Issuer      string        `mapstructure:"issuer"`       // 签发者
}

// WebModule Web 服务与安全基线配置。
type WebModule struct {
	Enabled  bool   `mapstructure:"enabled"`
	Port     string `mapstructure:"port"`      // 监听地址,默认 ":8080"
	Level    string `mapstructure:"level"`     // 日志级别
	TimeFmt  string `mapstructure:"time_format"` // iris 时间格式(默认 appbox.TimeFormat)
	// Baseline 是否自动挂载安全基线中间件(限流/请求体上限/超时/SQL 注入拦截/日志/安全头)。
	Baseline bool `mapstructure:"baseline"`
	// RatePerSecond 基线全局限流 QPS(默认 100)。
	RatePerSecond float64 `mapstructure:"rate_per_second"`
	// Burst 基线限流突发容量(默认 200)。
	Burst int `mapstructure:"burst"`
	// MaxBodyBytes 请求体上限字节(默认 1MB)。
	MaxBodyBytes int64 `mapstructure:"max_body_bytes"`
	// Timeout 请求超时(默认 10s)。
	Timeout time.Duration `mapstructure:"timeout"`
}

// DatabaseModule 关系数据库模块配置。
type DatabaseModule struct {
	Enabled     bool   `mapstructure:"enabled"`
	Driver      string `mapstructure:"driver"`       // sqlite/postgres/mysql
	DSN         string `mapstructure:"dsn"`          // 连接串或 SQLite 文件路径
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	User        string `mapstructure:"user"`
	Password    string `mapstructure:"password"`
	Database    string `mapstructure:"database"`
	SSLMode     string `mapstructure:"ssl_mode"`
	AutoMigrate bool   `mapstructure:"auto_migrate"`
	MaxIdle     int    `mapstructure:"max_idle_conns"`
	MaxOpen     int    `mapstructure:"max_open_conns"`
}

// CacheModule Redis 缓存模块配置。
type CacheModule struct {
	Enabled  bool   `mapstructure:"enabled"`
	Addr     string `mapstructure:"addr"`     // host:port
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// MongoModule MongoDB 模块配置。
type MongoModule struct {
	Enabled  bool   `mapstructure:"enabled"`
	Addr     string `mapstructure:"addr"` // host:port
	DB       string `mapstructure:"db"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Timeout  int    `mapstructure:"timeout"` // 秒,默认 20
}

// CronModule 定时任务模块配置。
type CronModule struct {
	Enabled bool `mapstructure:"enabled"`
}

// AdminModule Admin 管理服务配置(pprof/metrics/日志级别)。
type AdminModule struct {
	Enabled bool   `mapstructure:"enabled"`
	Listen  string `mapstructure:"listen"` // 默认 ":6060"
}

// MonitorModule 资源监控模块配置(路由级,Web 启用时挂载)。
type MonitorModule struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"` // 默认 "/monitor"
}

// OpenAPIApp 开放平台应用注册配置。
type OpenAPIApp struct {
	Secret    string `mapstructure:"secret"`    // HMAC 密钥或 RSA 公钥(PEM)
	Algorithm string `mapstructure:"algorithm"` // HMAC-SHA256 / RSA-SHA256,默认 HMAC-SHA256
	Enabled   bool   `mapstructure:"enabled"`
}

// OpenAPIModule 开放平台模块配置。
type OpenAPIModule struct {
	Enabled bool                  `mapstructure:"enabled"`
	Apps    map[string]OpenAPIApp `mapstructure:"apps"` // AppKey → 应用配置
}
