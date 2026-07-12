package datasource

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Connorig/go-blackbox/server/zaplog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
)

// DialectorFactory 根据统一配置创建 GORM Dialector。
// 实现不得记录 Config.DSN、Password 或其他认证信息。
type DialectorFactory func(config Config) (gorm.Dialector, error)

// Config 定义关系数据库共享的连接、连接池和迁移配置。
type Config struct {
	// Driver 选择已注册的关系数据库驱动。
	Driver Driver
	// DSN 允许高级场景直接提供驱动连接串；禁止写入日志。
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
	}
	databaseDriver Driver
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

// InitializeDatabase 使用统一配置初始化已注册的关系数据库。
// 成功后实例可通过 GetDbInstance 获取，并由 Health 和 Close 统一管理。
func InitializeDatabase(ctx context.Context, config *Config, models ...interface{}) (*gorm.DB, error) {
	if ctx == nil {
		return nil, errors.New("initialize database: context is nil")
	}
	normalizedConfig, err := normalizeDatabaseConfig(config)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("initialize %s database canceled before connection: %w", normalizedConfig.Driver, err)
	}

	databaseMu.Lock()
	defer databaseMu.Unlock()
	if database != nil {
		if databaseDriver != normalizedConfig.Driver {
			return nil, fmt.Errorf("database already initialized with driver %q", databaseDriver)
		}
		return database, nil
	}

	dialector, err := createDialector(normalizedConfig)
	if err != nil {
		return nil, err
	}
	initializedDatabase, err := gorm.Open(dialector, defaultGormConfig())
	if err != nil {
		return nil, fmt.Errorf("open %s database host=%s port=%d database=%s: %w", normalizedConfig.Driver, normalizedConfig.Host, normalizedConfig.Port, normalizedConfig.Database, err)
	}
	sqlDatabase, err := initializedDatabase.DB()
	if err != nil {
		return nil, fmt.Errorf("get %s connection pool: %w", normalizedConfig.Driver, err)
	}
	closeOnFailure := true
	defer func() {
		if closeOnFailure {
			if closeErr := sqlDatabase.Close(); closeErr != nil {
				zaplog.WithComponent("datasource").Errorw("close failed database initialization", "driver", normalizedConfig.Driver, "error", closeErr)
			}
		}
	}()

	applyUnifiedPoolConfig(sqlDatabase, normalizedConfig)
	pingCtx, cancelPing := context.WithTimeout(ctx, normalizedConfig.ConnectTimeout)
	defer cancelPing()
	if err := sqlDatabase.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("ping %s database host=%s port=%d database=%s: %w", normalizedConfig.Driver, normalizedConfig.Host, normalizedConfig.Port, normalizedConfig.Database, err)
	}

	models = filterModels(models)
	if normalizedConfig.AutoMigrate && len(models) > 0 {
		if err := initializedDatabase.WithContext(ctx).AutoMigrate(models...); err != nil {
			return nil, fmt.Errorf("auto migrate %s database models: %w", normalizedConfig.Driver, err)
		}
	}

	database = initializedDatabase
	databaseDriver = normalizedConfig.Driver
	closeOnFailure = false
	zaplog.WithComponent("datasource").Infow("database initialized", "driver", normalizedConfig.Driver, "host", normalizedConfig.Host, "port", normalizedConfig.Port, "database", normalizedConfig.Database)
	return database, nil
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
	if strings.TrimSpace(config.DSN) != "" {
		return nil
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
func newPostgreSQLDialector(config Config) (gorm.Dialector, error) {
	if config.DSN != "" {
		return postgres.Open(config.DSN), nil
	}
	if err := validatePostgreSQLSSLMode(config.SSLMode); err != nil {
		return nil, err
	}
	timeoutSeconds := int(config.ConnectTimeout.Seconds())
	if timeoutSeconds < 1 {
		timeoutSeconds = 1
	}
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s connect_timeout=%d", config.Host, config.UserName, config.Password, config.Database, config.Port, config.SSLMode, config.TimeZone, timeoutSeconds)
	return postgres.Open(dsn), nil
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
	return &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
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
