package datasource

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

const (
	defaultConnectTimeout  = 10 * time.Second
	defaultConnMaxLifetime = time.Hour
	defaultMaxIdleConns    = 10
	defaultMaxOpenConns    = 20
)

var (
	databaseMu sync.RWMutex
	database   *gorm.DB
)

// PostgresConfig 定义 PostgreSQL 建连、连接池和迁移策略。
type PostgresConfig struct {
	// UserName 是数据库用户名。
	UserName string
	// Password 是数据库密码，禁止写入日志。
	Password string
	// Host 是数据库主机名或 IP。
	Host string
	// Port 是 PostgreSQL 监听端口。
	Port int
	// DbName 是目标数据库名称。
	DbName string
	// InitDb 是旧版自动迁移开关。
	// Deprecated: 使用 AutoMigrate。
	InitDb bool
	// AliasName 是旧版连接别名字段，当前仅为源码兼容保留。
	AliasName string
	// SSL 是 PostgreSQL sslmode。
	SSL string
	// MaxIdleConns 是最大空闲连接数，零值使用默认值。
	MaxIdleConns int
	// MaxOpenConns 是最大打开连接数，零值使用默认值。
	MaxOpenConns int
	// ConnMaxLifetime 是连接可复用的最长时间，零值使用默认值。
	ConnMaxLifetime time.Duration
	// ConnectTimeout 是初始化和 Ping 的最大时长，零值使用默认值。
	ConnectTimeout time.Duration
	// AutoMigrate 控制是否在初始化成功后自动迁移 Model，默认关闭。
	AutoMigrate bool
}

// GormInit 保留旧版不接收 Context 的初始化入口。
// 新代码应使用 Initialize，以便调用方控制启动取消和超时。
func GormInit(config *PostgresConfig, models []interface{}) error {
	connectTimeout := defaultConnectTimeout
	if config != nil && config.ConnectTimeout > 0 {
		connectTimeout = config.ConnectTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	_, err := Initialize(ctx, config, models...)
	return err
}

// Initialize 校验配置、建立连接、执行 Ping，并按显式开关决定是否迁移 Model。
// 初始化失败会关闭已创建的连接池且不写入全局实例，后续调用可以安全重试。
func Initialize(ctx context.Context, config *PostgresConfig, models ...interface{}) (*gorm.DB, error) {
	normalizedConfig, err := normalizeConfig(config)
	if err != nil {
		return nil, err
	}
	unifiedConfig := &Config{
		Driver:          DriverPostgreSQL,
		UserName:        normalizedConfig.UserName,
		Password:        normalizedConfig.Password,
		Host:            normalizedConfig.Host,
		Port:            normalizedConfig.Port,
		Database:        normalizedConfig.DbName,
		SSLMode:         normalizedConfig.SSL,
		TimeZone:        "Asia/Shanghai",
		MaxIdleConns:    normalizedConfig.MaxIdleConns,
		MaxOpenConns:    normalizedConfig.MaxOpenConns,
		ConnMaxLifetime: normalizedConfig.ConnMaxLifetime,
		ConnectTimeout:  normalizedConfig.ConnectTimeout,
		AutoMigrate:     normalizedConfig.AutoMigrate,
	}
	return InitializeDatabase(ctx, unifiedConfig, models...)
}

// GetDbInstance 返回已经完成 Ping 的全局 GORM 实例。
func GetDbInstance() (*gorm.DB, error) {
	databaseMu.RLock()
	defer databaseMu.RUnlock()
	if database == nil {
		return nil, errors.New("relational database is not initialized")
	}
	return database, nil
}

// Health 使用调用方 Context 检查当前 PostgreSQL 连接池状态。
func Health(ctx context.Context) error {
	if ctx == nil {
		return errors.New("check database health: context is nil")
	}
	db, err := GetDbInstance()
	if err != nil {
		return err
	}
	sqlDatabase, err := db.DB()
	if err != nil {
		return fmt.Errorf("get database connection pool for health check: %w", err)
	}
	if err := sqlDatabase.PingContext(ctx); err != nil {
		return fmt.Errorf("check database health: %w", err)
	}
	return nil
}

// Close 关闭 PostgreSQL 连接池并清除全局实例，重复调用安全返回 nil。
func Close() error {
	databaseMu.Lock()
	defer databaseMu.Unlock()
	if database == nil {
		return nil
	}
	sqlDatabase, err := database.DB()
	if err != nil {
		return fmt.Errorf("get database connection pool for close: %w", err)
	}
	if err := sqlDatabase.Close(); err != nil {
		return fmt.Errorf("close database connection pool: %w", err)
	}
	database = nil
	databaseDriver = ""
	return nil
}

// normalizeConfig 复制、补齐默认值并校验 PostgreSQL 配置。
func normalizeConfig(config *PostgresConfig) (PostgresConfig, error) {
	if config == nil {
		return PostgresConfig{}, errors.New("PostgreSQL config is nil")
	}
	normalized := *config
	normalized.Host = strings.TrimSpace(normalized.Host)
	normalized.UserName = strings.TrimSpace(normalized.UserName)
	normalized.DbName = strings.TrimSpace(normalized.DbName)
	normalized.SSL = strings.ToLower(strings.TrimSpace(normalized.SSL))
	if normalized.SSL == "" {
		normalized.SSL = "disable"
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
	normalized.AutoMigrate = normalized.AutoMigrate || normalized.InitDb

	if err := validateConfig(normalized); err != nil {
		return PostgresConfig{}, err
	}
	return normalized, nil
}

// validateConfig 检查必填项、端口、连接池和超时边界。
func validateConfig(config PostgresConfig) error {
	if config.Host == "" {
		return errors.New("PostgreSQL host is empty")
	}
	if config.UserName == "" {
		return errors.New("PostgreSQL user name is empty")
	}
	if config.DbName == "" {
		return errors.New("PostgreSQL database name is empty")
	}
	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("PostgreSQL port is out of range: %d", config.Port)
	}
	if config.MaxIdleConns < 0 || config.MaxOpenConns < 1 || config.MaxIdleConns > config.MaxOpenConns {
		return fmt.Errorf("PostgreSQL connection pool is invalid: max_idle=%d max_open=%d", config.MaxIdleConns, config.MaxOpenConns)
	}
	if config.ConnMaxLifetime < 0 {
		return fmt.Errorf("PostgreSQL connection max lifetime is negative: %s", config.ConnMaxLifetime)
	}
	if config.ConnectTimeout <= 0 {
		return fmt.Errorf("PostgreSQL connect timeout must be positive: %s", config.ConnectTimeout)
	}
	validSSLMode := map[string]struct{}{"disable": {}, "allow": {}, "prefer": {}, "require": {}, "verify-ca": {}, "verify-full": {}}
	if _, ok := validSSLMode[config.SSL]; !ok {
		return fmt.Errorf("PostgreSQL ssl mode is invalid: %q", config.SSL)
	}
	return nil
}

// applyPoolConfig 将已校验的连接池参数应用到底层 database/sql。
func applyPoolConfig(sqlDatabase interface {
	SetMaxIdleConns(int)
	SetMaxOpenConns(int)
	SetConnMaxLifetime(time.Duration)
}, config PostgresConfig) {
	sqlDatabase.SetMaxIdleConns(config.MaxIdleConns)
	sqlDatabase.SetMaxOpenConns(config.MaxOpenConns)
	sqlDatabase.SetConnMaxLifetime(config.ConnMaxLifetime)
}

// filterModels 返回保持原顺序的非 nil Model 列表，不修改调用方切片。
func filterModels(models []interface{}) []interface{} {
	filteredModels := make([]interface{}, 0, len(models))
	for _, model := range models {
		if isNilModel(model) {
			continue
		}
		filteredModels = append(filteredModels, model)
	}
	return filteredModels
}

// isNilModel 同时识别 nil 接口和包含 nil 指针的接口。
func isNilModel(model interface{}) bool {
	if model == nil {
		return true
	}
	value := reflect.ValueOf(model)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
