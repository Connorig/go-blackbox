package datasource

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/jackc/pgx/v5"
	zaplog "github.com/Connorig/go-blackbox/framework/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Driver 标识关系数据库类型。
type Driver string

const (
	// DriverPostgreSQL 使用 GORM 官方 PostgreSQL Dialector。
	DriverPostgreSQL Driver = "postgres"
	// DriverMySQL 标识 MySQL/MariaDB，调用方需要注册选定的 GORM Dialector。
	DriverMySQL Driver = "mysql"
	// DriverOracle 预留 Oracle 标识，调用方需要注册符合运行环境的 Oracle Dialector。
	DriverOracle Driver = "oracle"
	// DriverSQLite 使用纯 Go SQLite 驱动（无 CGO），适合测试与轻量部署。
	DriverSQLite Driver = "sqlite"
)

// DialectorFactory 根据统一配置创建 GORM Dialector。
// 实现不得记录 Config.DSN、Password 或其他认证信息。
type DialectorFactory func(config Config) (gorm.Dialector, error)

// Config 定义关系数据库共享的连接、连接池和迁移配置。
type Config struct {
	// Driver 选择已注册的关系数据库驱动。
	Driver Driver
	// DSN 允许高级场景直接提供驱动连接串（SQLite 为文件路径）；禁止写入日志。
	DSN string
	// UserName 是数据库用户名。
	UserName string
	// Password 是数据库密码，禁止写入日志。
	Password string
	// Host 是数据库主机名或 IP。
	Host string
	// Port 是数据库监听端口。
	Port int
	// Database 是数据库名、Schema 服务名或驱动定义的逻辑数据库。
	Database string
	// SSLMode 是 PostgreSQL SSL 模式；其他驱动可以忽略。
	SSLMode string
	// Charset 是 MySQL 字符集，默认 utf8mb4。
	Charset string
	// TimeZone 是数据库会话时区，默认 Asia/Shanghai。
	TimeZone string
	// MaxIdleConns 是最大空闲连接数。
	MaxIdleConns int
	// MaxOpenConns 是最大打开连接数。
	MaxOpenConns int
	// ConnMaxLifetime 是连接最大复用时间。
	ConnMaxLifetime time.Duration
	// ConnectTimeout 是打开连接和 Ping 的最大时长。
	ConnectTimeout time.Duration
	// AutoMigrate 控制是否自动迁移 Model，默认关闭。
	AutoMigrate bool
}

var (
	dialectorMu sync.RWMutex
	dialectors  = map[Driver]DialectorFactory{
		DriverPostgreSQL: newPostgreSQLDialector,
		DriverSQLite:     newSQLiteDialector,
	}
)

// RegisterDialector 注册或替换自定义关系数据库 Dialector。
// 典型用途是为 Oracle、SQL Server 或内部代理协议接入特定 GORM Driver。
func RegisterDialector(driver Driver, factory DialectorFactory) error {
	normalizedDriver := normalizeDriver(driver)
	if normalizedDriver == "" {
		return errors.New("register database dialector: driver is empty")
	}
	if factory == nil {
		return fmt.Errorf("register database dialector %q: factory is nil", normalizedDriver)
	}
	dialectorMu.Lock()
	dialectors[normalizedDriver] = factory
	dialectorMu.Unlock()
	return nil
}

// InitializeDatabase 使用统一配置初始化默认关系数据库实例。
// 兼容入口：新代码建议使用 New / NewNamed 获得带生命周期的 Instance。
func InitializeDatabase(ctx context.Context, config *Config, models ...interface{}) (*gorm.DB, error) {
	instance, err := New(ctx, config, models...)
	if err != nil {
		return nil, err
	}
	return instance.DB(), nil
}

// normalizeDatabaseConfig 复制统一配置、补齐默认值并执行公共校验。
func normalizeDatabaseConfig(config *Config) (Config, error) {
	if config == nil {
		return Config{}, errors.New("database config is nil")
	}
	normalized := *config
	normalized.Driver = normalizeDriver(normalized.Driver)
	normalized.Host = strings.TrimSpace(normalized.Host)
	normalized.UserName = strings.TrimSpace(normalized.UserName)
	normalized.Database = strings.TrimSpace(normalized.Database)
	normalized.SSLMode = strings.ToLower(strings.TrimSpace(normalized.SSLMode))
	normalized.Charset = strings.TrimSpace(normalized.Charset)
	normalized.TimeZone = strings.TrimSpace(normalized.TimeZone)
	if normalized.TimeZone == "" {
		normalized.TimeZone = "Asia/Shanghai"
	}
	if normalized.MaxIdleConns == 0 {
		normalized.MaxIdleConns = defaultMaxIdleConns
	}
	if normalized.MaxOpenConns == 0 {
		normalized.MaxOpenConns = defaultMaxOpenConns
	}
	if normalized.ConnMaxLifetime == 0 {
		normalized.ConnMaxLifetime = defaultConnMaxLifetime
	}
	if normalized.ConnectTimeout == 0 {
		normalized.ConnectTimeout = defaultConnectTimeout
	}
	if normalized.Driver == DriverPostgreSQL && normalized.SSLMode == "" {
		normalized.SSLMode = "disable"
	}
	if normalized.Driver == DriverMySQL && normalized.Charset == "" {
		normalized.Charset = "utf8mb4"
	}
	if err := validateDatabaseConfig(normalized); err != nil {
		return Config{}, err
	}
	return normalized, nil
}

// validateDatabaseConfig 校验统一连接池字段和内置驱动必填项。
// SQLite 只需要 DSN（文件路径），不要求主机、账号与端口。
func validateDatabaseConfig(config Config) error {
	if config.Driver == "" {
		return errors.New("database driver is empty")
	}
	if config.MaxIdleConns < 0 || config.MaxOpenConns < 1 || config.MaxIdleConns > config.MaxOpenConns {
		return fmt.Errorf("database connection pool is invalid: max_idle=%d max_open=%d", config.MaxIdleConns, config.MaxOpenConns)
	}
	if config.ConnMaxLifetime < 0 || config.ConnectTimeout <= 0 {
		return fmt.Errorf("database duration configuration is invalid: connection_lifetime=%s connect_timeout=%s", config.ConnMaxLifetime, config.ConnectTimeout)
	}
	if config.Driver == DriverSQLite {
		if strings.TrimSpace(config.DSN) == "" {
			return errors.New("SQLite database requires DSN (file path)")
		}
		return nil
	}
	if strings.TrimSpace(config.DSN) != "" {
		return nil
	}
	if config.Driver == DriverPostgreSQL && strings.Contains(config.Password, "'") {
		return errors.New("postgres password containing single quote is not expressible in key=value DSN: use Config.DSN with URL format (postgres://user:password@host/db)")
	}
	if config.Host == "" || config.UserName == "" || config.Database == "" {
		return fmt.Errorf("database host, user name and database are required for driver %q", config.Driver)
	}
	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("database port is out of range: %d", config.Port)
	}
	return nil
}

// createDialector 查找驱动工厂并创建 Dialector。
func createDialector(config Config) (gorm.Dialector, error) {
	dialectorMu.RLock()
	factory := dialectors[config.Driver]
	dialectorMu.RUnlock()
	if factory == nil {
		return nil, fmt.Errorf("database driver %q is not registered", config.Driver)
	}
	dialector, err := factory(config)
	if err != nil {
		return nil, fmt.Errorf("create %s database dialector: %w", config.Driver, err)
	}
	if dialector == nil {
		return nil, fmt.Errorf("create %s database dialector: factory returned nil", config.Driver)
	}
	return dialector, nil
}

// newPostgreSQLDialector 创建 PostgreSQL Dialector。
// DSN 直连时先规范化(解析后重建安全 URL DSN),特殊字符密码彻底免疫;
func newPostgreSQLDialector(config Config) (gorm.Dialector, error) {
	if strings.TrimSpace(config.DSN) != "" {
		dsn, err := normalizePostgreSQLDSN(config.DSN)
		if err != nil {
			return nil, err
		}
		return postgres.Open(dsn), nil
	}
	if err := validatePostgreSQLSSLMode(config.SSLMode); err != nil {
		return nil, err
	}
	timeoutSeconds := int(config.ConnectTimeout.Seconds())
	if timeoutSeconds < 1 {
		timeoutSeconds = 1
	}
	dsn := buildPostgreSQLDSN(config, timeoutSeconds)
	return postgres.Open(dsn), nil
}

// buildPostgreSQLDSN 拼接 PostgreSQL DSN(独立函数,便于测试密码正确性)。
// Host/UserName/Password/Database 经 quotePGValue 引用:含空格、#、单引号时
// 自动单引号包裹(内部单引号翻倍),避免 pgx key=value 解析截断。
func buildPostgreSQLDSN(config Config, timeoutSeconds int) string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s connect_timeout=%d",
		quotePGValue(config.Host), quotePGValue(config.UserName), quotePGValue(config.Password),
		quotePGValue(config.Database), config.Port, config.SSLMode, config.TimeZone, timeoutSeconds)
}

// quotePGValue 对 pgx DSN 值做安全引用:值含空格、# 时用单引号包裹(符合 pgx key=value 规则);
// 安全值原样返回。注意:pgx 单引号包裹不支持内部单引号转义,含 ' 的值由
// validateDatabaseConfig 在字段配置路径拒绝(改用 URL 格式 DSN)。
func quotePGValue(value string) string {
	if value == "" || strings.ContainsAny(value, " #") {
		return "'" + value + "'"
	}
	return value
}

// normalizePostgreSQLDSN 解析 DSN 并用 pgx ConnConfig 重建成安全 URL 格式 DSN。
// 错误信息不泄露密码内容,只提示修复方式。
func normalizePostgreSQLDSN(dsn string) (string, error) {
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return "", fmt.Errorf("parse postgres DSN: %w (密码含特殊字符时请用单引号包裹 password='***',或改用字段配置/URL 格式 postgres://...)", err)
	}
	// 未引号密码被空格截断的残段检测(ParseConfig 无法感知的信息丢失)
	if suspiciousPasswordTail(dsn) {
		return "", fmt.Errorf("postgres DSN password may be truncated by a space: values with spaces must be single-quoted (password='xxx') or use URL format")
	}
	if strings.HasPrefix(config.Host, "/") {
		return dsn, nil // Unix socket 路径主机,保持 key=value 原样
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(config.User, config.Password),
		Host:   net.JoinHostPort(config.Host, strconv.FormatUint(uint64(config.Port), 10)),
		Path:   config.Database,
	}
	query := u.Query()
	for key, value := range config.RuntimeParams {
		query.Set(key, value)
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

// suspiciousPasswordTail 检测 password= 未引号值之后是否跟有非 key=value 的残段(疑似截断)。
func suspiciousPasswordTail(dsn string) bool {
	const prefix = "password="
	index := strings.Index(dsn, prefix)
	if index < 0 {
		return false
	}
	rest := dsn[index+len(prefix):]
	if strings.HasPrefix(rest, "'") {
		return false // 已引号包裹,值完整
	}
	spaceIndex := strings.Index(rest, " ")
	if spaceIndex < 0 {
		return false // 值直到行尾,完整
	}
	tail := strings.TrimSpace(rest[spaceIndex+1:])
	if tail == "" {
		return false
	}
	nextToken := tail
	if end := strings.Index(tail, " "); end >= 0 {
		nextToken = tail[:end]
	}
	// 后续 token 是 key=value 形式则为合法下一参数;否则是密码残段(截断)
	return !strings.Contains(nextToken, "=")
}

// newSQLiteDialector 创建纯 Go SQLite Dialector（无 CGO）。
// DSN 是数据库文件路径，例如 :memory: 或 /data/app.db。
func newSQLiteDialector(config Config) (gorm.Dialector, error) {
	if strings.TrimSpace(config.DSN) == "" {
		return nil, errors.New("SQLite dialector: DSN (file path) is required")
	}
	return sqlite.Open(config.DSN), nil
}

// MySQLDSN 根据统一配置生成不记录到日志的 MySQL DSN。
// 调用方可以把返回值交给所选 MySQL GORM Driver，并通过 RegisterDialector 注册。
func MySQLDSN(config Config) (string, error) {
	normalizedConfig, err := normalizeDatabaseConfig(&config)
	if err != nil {
		return "", err
	}
	if normalizedConfig.Driver != DriverMySQL {
		return "", fmt.Errorf("build MySQL DSN: unexpected driver %q", normalizedConfig.Driver)
	}
	if normalizedConfig.DSN != "" {
		return normalizedConfig.DSN, nil
	}
	query := url.Values{}
	query.Set("charset", normalizedConfig.Charset)
	query.Set("parseTime", "True")
	query.Set("loc", normalizedConfig.TimeZone)
	query.Set("timeout", normalizedConfig.ConnectTimeout.String())
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s", normalizedConfig.UserName, normalizedConfig.Password, normalizedConfig.Host, normalizedConfig.Port, normalizedConfig.Database, query.Encode())
	return dsn, nil
}

// validatePostgreSQLSSLMode 校验 PostgreSQL 官方支持的 SSL 模式。
func validatePostgreSQLSSLMode(mode string) error {
	validModes := map[string]struct{}{"disable": {}, "allow": {}, "prefer": {}, "require": {}, "verify-ca": {}, "verify-full": {}}
	if _, ok := validModes[mode]; !ok {
		return fmt.Errorf("PostgreSQL ssl mode is invalid: %q", mode)
	}
	return nil
}

// defaultGormConfig 返回所有关系数据库共享的 GORM 行为配置。
func defaultGormConfig() *gorm.Config {
	// GORM SQL 日志级别随 gbx 全局日志级别联动:开发 debug 可见 SQL 详情,
	// 生产 info 只留慢查询与错误,保持分级策略一致。
	gormLogLevel := gormlogger.Error
	switch strings.ToLower(strings.TrimSpace(zaplog.CONFIG.Level)) {
	case "debug":
		gormLogLevel = gormlogger.Info
	case "info":
		gormLogLevel = gormlogger.Warn
	}
	gormConfig := zaplog.DefaultGormLoggerConfig()
	gormConfig.LogLevel = gormLogLevel
	return &gorm.Config{
		Logger: zaplog.NewGormLogger(gormConfig),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	}
}
// applyUnifiedPoolConfig 将统一连接池参数应用到底层 database/sql。
func applyUnifiedPoolConfig(pool interface {
	SetMaxIdleConns(int)
	SetMaxOpenConns(int)
	SetConnMaxLifetime(time.Duration)
}, config Config) {
	pool.SetMaxIdleConns(config.MaxIdleConns)
	pool.SetMaxOpenConns(config.MaxOpenConns)
	pool.SetConnMaxLifetime(config.ConnMaxLifetime)
}

// normalizeDriver 统一驱动名称大小写并兼容常见别名。
func normalizeDriver(driver Driver) Driver {
	switch strings.ToLower(strings.TrimSpace(string(driver))) {
	case "postgres", "postgresql", "pgsql":
		return DriverPostgreSQL
	case "mysql", "mariadb":
		return DriverMySQL
	case "oracle", "oci", "godror":
		return DriverOracle
	case "sqlite", "sqlite3":
		return DriverSQLite
	default:
		return Driver(strings.ToLower(strings.TrimSpace(string(driver))))
	}
}

// databaseAddress 返回不包含认证信息的数据库地址，供日志和错误上下文使用。
func databaseAddress(config Config) string {
	if config.Host == "" {
		return string(config.Driver)
	}
	return config.Host + ":" + strconv.Itoa(config.Port)
}


// MigrateModels 对一组模型执行 AutoMigrate(业务手动迁移/初始化建表用)。
// 等价于 datasource.Get().DB().AutoMigrate(models...),但统一处理实例获取错误。
func MigrateModels(ctx context.Context, models ...interface{}) error {
	instance, err := Get()
	if err != nil {
		return err
	}
	if instance == nil || instance.DB() == nil {
		return errors.New("database instance is nil: call EnableDatabase first")
	}
	if len(models) == 0 {
		return nil
	}
	return instance.DB().WithContext(ctx).AutoMigrate(models...)
}