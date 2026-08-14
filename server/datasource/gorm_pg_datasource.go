package datasource

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	defaultConnectTimeout  = 10 * time.Second
	defaultConnMaxLifetime = time.Hour
	defaultMaxIdleConns    = 10
	defaultMaxOpenConns    = 20
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
// 返回的 *gorm.DB 属于默认实例；多数据源场景请使用 NewNamed。
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
	instance, err := New(ctx, unifiedConfig, models...)
	if err != nil {
		return nil, err
	}
	return instance.DB(), nil
}

// GetDbInstance 返回默认实例的 GORM 对象。
func GetDbInstance() (*gorm.DB, error) {
	instance, err := Get()
	if err != nil {
		return nil, err
	}
	return instance.DB(), nil
}

// Health 使用调用方 Context 检查默认实例连接池状态。
func Health(ctx context.Context) error {
	instance, err := Get()
	if err != nil {
		return err
	}
	return instance.Health(ctx)
}

// Close 关闭默认实例连接池并清除默认实例注册，重复调用安全返回 nil。
// 其他具名实例需要分别调用对应实例的 Close。
func Close() error {
	instancesMu.Lock()
	defer instancesMu.Unlock()
	if defaultInstance == nil {
		return nil
	}
	instance := defaultInstance
	defaultInstance = nil
	delete(instances, "")
	if err := instance.Close(); err != nil {
		return err
	}
	return nil
}

// WithTx 在默认实例上执行带事务的业务函数，自动提交或回滚。
func WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	instance, err := Get()
	if err != nil {
		return err
	}
	return instance.WithTx(ctx, fn)
}

// PageResult 是分页查询的元数据与结果容器。
type PageResult struct {
	Total      int64 // 符合条件的总记录数
	Page       int   // 当前页（从 1 开始）
	PageSize   int   // 每页大小
	TotalPages int64 // 总页数
}

// Page 在默认实例上执行带分页的查询。
// db 应预先携带过滤条件（Where 等）；page 从 1 开始，pageSize 非正数时使用默认 10。
// 查询结果解码到 out（必须是指向切片的指针）。
func Page(ctx context.Context, db *gorm.DB, page, pageSize int, out interface{}) (*PageResult, error) {
	instance, err := Get()
	if err != nil {
		return nil, err
	}
	return PageOn(instance.DB(), ctx, db, page, pageSize, out)
}

// PageOn 在指定 GORM 连接上执行带分页的查询。
// 适合多实例场景：pageDb 使用 instance.DB() 派生。
func PageOn(query *gorm.DB, ctx context.Context, db *gorm.DB, page, pageSize int, out interface{}) (*PageResult, error) {
	if query == nil {
		return nil, errors.New("page query: gorm db is nil")
	}
	if ctx != nil {
		query = query.WithContext(ctx)
	}
	if out == nil {
		return nil, errors.New("page query: result target is nil")
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("page count: %w", err)
	}
	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).Find(out).Error; err != nil {
		return nil, fmt.Errorf("page find: %w", err)
	}

	totalPages := int64(0)
	if total > 0 {
		totalPages = (total + int64(pageSize) - 1) / int64(pageSize)
	}
	return &PageResult{Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages}, nil
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

// applyPoolConfig 将已校验的连接池参数应用到底层 database/sql。
// Deprecated: 统一配置路径使用 applyUnifiedPoolConfig。
func applyPoolConfig(sqlDatabase interface {
	SetMaxIdleConns(int)
	SetMaxOpenConns(int)
	SetConnMaxLifetime(time.Duration)
}, config PostgresConfig) {
	sqlDatabase.SetMaxIdleConns(config.MaxIdleConns)
	sqlDatabase.SetMaxOpenConns(config.MaxOpenConns)
	sqlDatabase.SetConnMaxLifetime(config.ConnMaxLifetime)
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
