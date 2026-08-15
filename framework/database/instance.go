package datasource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Connorig/go-blackbox/framework/log"
	"gorm.io/gorm"
)

// Instance 是独立的关系数据库连接实例。
// 一个进程可以创建多个实例（多数据源、读写分离、多租户），每个实例独立管理连接池与生命周期。
type Instance struct {
	name   string   // 实例名称；默认实例为空字符串
	config Config   // 规范化后的配置（不含密码）
	db     *gorm.DB // GORM 实例
	mu     sync.RWMutex
	closed bool
}

// Name 返回实例名称；默认实例返回空字符串。
func (i *Instance) Name() string {
	if i == nil {
		return ""
	}
	return i.name
}

// Driver 返回实例使用的数据库驱动。
func (i *Instance) Driver() Driver {
	if i == nil {
		return ""
	}
	return i.config.Driver
}

// DB 返回 GORM 实例；实例已关闭时返回 nil。
func (i *Instance) DB() *gorm.DB {
	if i == nil {
		return nil
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.closed {
		return nil
	}
	return i.db
}

// Health 使用调用方 Context 检查实例连接池状态。
func (i *Instance) Health(ctx context.Context) error {
	if ctx == nil {
		return errors.New("check database health: context is nil")
	}
	if i == nil {
		return errors.New("check database health: instance is nil")
	}
	db := i.DB()
	if db == nil {
		return errors.New("check database health: instance is closed")
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

// Close 关闭实例连接池并标记为已关闭；重复调用安全返回 nil。
func (i *Instance) Close() error {
	if i == nil {
		return nil
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.closed || i.db == nil {
		return nil
	}
	sqlDatabase, err := i.db.DB()
	if err != nil {
		return fmt.Errorf("get database connection pool for close: %w", err)
	}
	if err := sqlDatabase.Close(); err != nil {
		return fmt.Errorf("close database connection pool: %w", err)
	}
	i.closed = true
	return nil
}

// WithTx 在实例上执行带事务的业务函数，自动提交或回滚。
// fn 返回错误时事务回滚并保留错误链。
func (i *Instance) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	if fn == nil {
		return errors.New("run database transaction: function is nil")
	}
	db := i.DB()
	if db == nil {
		return errors.New("run database transaction: instance is nil or closed")
	}
	return db.WithContext(ctx).Transaction(fn)
}

// 实例注册表：name -> Instance，默认实例使用空字符串 key。
var (
	instancesMu     sync.RWMutex
	instances       = map[string]*Instance{}
	defaultInstance *Instance
)

// New 使用统一配置创建并注册默认实例（兼容 InitializeDatabase 语义）。
// 重复创建默认实例时，若驱动一致返回既有实例，否则返回错误。
func New(ctx context.Context, config *Config, models ...interface{}) (*Instance, error) {
	return NewNamed(ctx, "", config, models...)
}

// NewNamed 使用统一配置创建具名实例并注册到实例表。
// 同名实例已存在时返回错误；初始化失败不会注册任何实例。
func NewNamed(ctx context.Context, name string, config *Config, models ...interface{}) (*Instance, error) {
	if ctx == nil {
		return nil, errors.New("initialize database: context is nil")
	}
	name = strings.TrimSpace(name)
	normalizedConfig, err := normalizeDatabaseConfig(config)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("initialize %s database canceled before connection: %w", normalizedConfig.Driver, err)
	}

	instancesMu.Lock()
	if name == "" && defaultInstance != nil {
		existing := defaultInstance
		instancesMu.Unlock()
		if existing.config.Driver != normalizedConfig.Driver {
			return nil, fmt.Errorf("default database already initialized with driver %q", existing.config.Driver)
		}
		return existing, nil
	}
	if existing, ok := instances[name]; ok {
		instancesMu.Unlock()
		return nil, fmt.Errorf("database instance %q already initialized with driver %q", name, existing.config.Driver)
	}

	instance, err := openInstance(name, normalizedConfig, ctx, models...)
	if err != nil {
		instancesMu.Unlock()
		return nil, err
	}
	if name == "" {
		defaultInstance = instance
	}
	instances[name] = instance
	instancesMu.Unlock()

	zaplog.WithComponent("datasource").Infow("database initialized",
		"name", displayInstanceName(name), "driver", normalizedConfig.Driver,
		"host", normalizedConfig.Host, "port", normalizedConfig.Port, "database", normalizedConfig.Database)
	return instance, nil
}

// openInstance 执行建连、Ping、迁移并构造实例；失败时关闭已创建的连接池。
func openInstance(name string, config Config, ctx context.Context, models ...interface{}) (*Instance, error) {
	dialector, err := createDialector(config)
	if err != nil {
		return nil, err
	}
	initializedDatabase, err := gorm.Open(dialector, defaultGormConfig())
	if err != nil {
		return nil, fmt.Errorf("open %s database host=%s port=%d database=%s: %w", config.Driver, config.Host, config.Port, config.Database, err)
	}
	sqlDatabase, err := initializedDatabase.DB()
	if err != nil {
		return nil, fmt.Errorf("get %s connection pool: %w", config.Driver, err)
	}
	closeOnFailure := true
	defer func() {
		if closeOnFailure {
			if closeErr := sqlDatabase.Close(); closeErr != nil {
				zaplog.WithComponent("datasource").Errorw("close failed database initialization",
					"name", displayInstanceName(name), "driver", config.Driver, "error", closeErr)
			}
		}
	}()

	applyUnifiedPoolConfig(sqlDatabase, config)
	pingCtx, cancelPing := context.WithTimeout(ctx, config.ConnectTimeout)
	defer cancelPing()
	if err := sqlDatabase.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("ping %s database host=%s port=%d database=%s: %w", config.Driver, config.Host, config.Port, config.Database, err)
	}

	models = filterModels(models)
	if config.AutoMigrate && len(models) > 0 {
		if err := initializedDatabase.WithContext(ctx).AutoMigrate(models...); err != nil {
			return nil, fmt.Errorf("auto migrate %s database models: %w", config.Driver, err)
		}
	}

	closeOnFailure = false
	return &Instance{name: name, config: config, db: initializedDatabase}, nil
}

// Get 返回默认实例；未初始化返回错误。
func Get() (*Instance, error) {
	instancesMu.RLock()
	defer instancesMu.RUnlock()
	if defaultInstance == nil {
		return nil, errors.New("default relational database is not initialized")
	}
	return defaultInstance, nil
}

// GetNamed 返回具名实例；未注册返回错误。
func GetNamed(name string) (*Instance, error) {
	name = strings.TrimSpace(name)
	instancesMu.RLock()
	defer instancesMu.RUnlock()
	instance, ok := instances[name]
	if !ok {
		return nil, fmt.Errorf("database instance %q is not initialized", name)
	}
	return instance, nil
}

// Instances 返回全部已注册实例（默认实例名为空字符串）。
func Instances() []*Instance {
	instancesMu.RLock()
	defer instancesMu.RUnlock()
	result := make([]*Instance, 0, len(instances))
	for _, instance := range instances {
		result = append(result, instance)
	}
	return result
}

// displayInstanceName 为日志输出提供可读的实例名（默认实例显示 default）。
func displayInstanceName(name string) string {
	if name == "" {
		return "default"
	}
	return name
}
