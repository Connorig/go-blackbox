package apploader

// Config 保留历史全局配置变量，供尚未迁移到业务自定义配置结构的调用方使用。
// 新项目应优先声明自己的配置实例，避免多个应用共享可变全局状态。
var Config Configuration

// Configuration 定义脚手架内置的基础配置示例。
// 字段标签同时支持 Viper 的 mapstructure 和直接 TOML 解码。
type Configuration struct {
	// Name 是应用名称。
	Name string `mapstructure:"name" toml:"name"`
	// Version 是应用配置声明的版本。
	Version string `mapstructure:"version" toml:"version"`
	// Web 保存 Web 监听配置。
	Web web `mapstructure:"web" toml:"web"`
	// Db 保存 PostgreSQL 配置；字段名为兼容既有调用保留。
	Db db `mapstructure:"db" toml:"db"`
	// Redis 保存 Redis 配置。
	Redis redis `mapstructure:"redis" toml:"redis"`
	// LogConf 保存日志输出配置。
	LogConf logConf `mapstructure:"logConf" toml:"logConf"`
}

// web 定义 Iris Web 服务的监听地址和日志级别。
type web struct {
	// Listen 是包含主机和端口的监听地址。
	Listen string `mapstructure:"listen" toml:"listen"`
	// Level 是 Iris 日志级别。
	Level string `mapstructure:"level" toml:"level"`
}

// db 定义 PostgreSQL 连接和连接池配置。
type db struct {
	// Driver 是关系数据库类型，例如 postgres、mysql 或 oracle。
	Driver string `mapstructure:"driver" toml:"driver"`
	// User 是数据库用户名。
	User string `mapstructure:"user" toml:"user"`
	// Password 是数据库密码，禁止写入日志。
	Password string `mapstructure:"password" toml:"password"`
	// Host 是数据库主机名或 IP。
	Host string `mapstructure:"host" toml:"host"`
	// Port 是数据库监听端口。
	Port int `mapstructure:"port" toml:"port"`
	// DbName 是需要连接的数据库名称。
	DbName string `mapstructure:"dbName" toml:"dbName"`
	// Ssl 是 PostgreSQL SSL 模式。
	Ssl string `mapstructure:"ssl" toml:"ssl"`
	// Charset 是 MySQL 等数据库使用的字符集。
	Charset string `mapstructure:"charset" toml:"charset"`
	// TimeZone 是数据库会话时区。
	TimeZone string `mapstructure:"timeZone" toml:"timeZone"`
	// AutoMigrate 控制是否显式执行 GORM 自动迁移。
	AutoMigrate bool `mapstructure:"autoMigrate" toml:"autoMigrate"`
	// MaxIdleConns 是最大空闲连接数。
	MaxIdleConns int `mapstructure:"maxIdleConns" toml:"maxIdleConns"`
	// MaxOpenConns 是最大打开连接数。
	MaxOpenConns int `mapstructure:"maxOpenConns" toml:"maxOpenConns"`
	// MaxIdleCones 是旧版拼写错误字段，保留用于源码兼容。
	// Deprecated: 使用 MaxIdleConns。
	MaxIdleCones int `mapstructure:"-" toml:"-"`
	// MaxOpenCones 是旧版拼写错误字段，保留用于源码兼容。
	// Deprecated: 使用 MaxOpenConns。
	MaxOpenCones int `mapstructure:"-" toml:"-"`
}

// redis 定义单节点 Redis 客户端配置。
type redis struct {
	// Host 是 Redis 主机和端口。
	Host string `mapstructure:"host" toml:"host"`
	// Password 是 Redis 密码，禁止写入日志。
	Password string `mapstructure:"password" toml:"password"`
	// PoolSize 是 Redis 连接池大小。
	PoolSize int `mapstructure:"poolSize" toml:"poolSize"`
	// Db 是 Redis 逻辑数据库编号。
	Db int `mapstructure:"db" toml:"db"`
}

// logConf 定义日志目录和最低日志级别。
type logConf struct {
	// OutDirPath 是日志根目录。
	OutDirPath string `mapstructure:"outDirPath" toml:"outDirPath"`
	// LogLevel 是最低日志级别。
	LogLevel string `mapstructure:"logLevel" toml:"logLevel"`
}

// normalizeBuiltInConfiguration 同步内置配置中的兼容字段。
// 自定义业务配置不受该兼容逻辑影响。
func normalizeBuiltInConfiguration(config interface{}) {
	builtInConfig, ok := config.(*Configuration)
	if !ok || builtInConfig == nil {
		return
	}
	builtInConfig.Db.MaxIdleCones = builtInConfig.Db.MaxIdleConns
	builtInConfig.Db.MaxOpenCones = builtInConfig.Db.MaxOpenConns
}
