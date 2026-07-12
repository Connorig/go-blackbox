package datasource

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// testModel 是 nil Model 过滤测试使用的最小 GORM Model。
type testModel struct {
	// ID 是测试模型主键。
	ID int
}

// fakeConnectionPool 记录连接池配置函数接收到的值。
type fakeConnectionPool struct {
	maxIdleConns    int
	maxOpenConns    int
	connMaxLifetime time.Duration
}

// SetMaxIdleConns 记录最大空闲连接数。
func (pool *fakeConnectionPool) SetMaxIdleConns(value int) {
	pool.maxIdleConns = value
}

// SetMaxOpenConns 记录最大打开连接数。
func (pool *fakeConnectionPool) SetMaxOpenConns(value int) {
	pool.maxOpenConns = value
}

// SetConnMaxLifetime 记录连接最大复用时间。
func (pool *fakeConnectionPool) SetConnMaxLifetime(value time.Duration) {
	pool.connMaxLifetime = value
}

// TestNormalizeConfigAppliesDefaults 验证零值连接池、SSL 和超时会使用安全默认值。
func TestNormalizeConfigAppliesDefaults(t *testing.T) {
	config, err := normalizeConfig(&PostgresConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		UserName: "postgres",
		DbName:   "app",
	})
	if err != nil {
		t.Fatalf("normalize PostgreSQL config failed: %v", err)
	}
	if config.SSL != "disable" || config.MaxIdleConns != defaultMaxIdleConns || config.MaxOpenConns != defaultMaxOpenConns {
		t.Fatalf("unexpected PostgreSQL defaults: %+v", config)
	}
	if config.ConnectTimeout != defaultConnectTimeout || config.ConnMaxLifetime != defaultConnMaxLifetime {
		t.Fatalf("unexpected PostgreSQL duration defaults: %+v", config)
	}
}

// TestNormalizeConfigPreservesLegacyMigrationSwitch 验证旧 InitDb 开关仍会启用显式迁移行为。
func TestNormalizeConfigPreservesLegacyMigrationSwitch(t *testing.T) {
	config, err := normalizeConfig(&PostgresConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		UserName: "postgres",
		DbName:   "app",
		InitDb:   true,
	})
	if err != nil {
		t.Fatalf("normalize legacy PostgreSQL config failed: %v", err)
	}
	if !config.AutoMigrate {
		t.Fatal("legacy InitDb must enable AutoMigrate")
	}
}

// TestNormalizeConfigRejectsInvalidValues 验证必填项、端口和连接池错误会在建连前返回。
func TestNormalizeConfigRejectsInvalidValues(t *testing.T) {
	testCases := []struct {
		name   string
		config *PostgresConfig
	}{
		{name: "nil config", config: nil},
		{name: "missing host", config: &PostgresConfig{Port: 5432, UserName: "postgres", DbName: "app"}},
		{name: "invalid port", config: &PostgresConfig{Host: "localhost", Port: 70000, UserName: "postgres", DbName: "app"}},
		{name: "idle exceeds open", config: &PostgresConfig{Host: "localhost", Port: 5432, UserName: "postgres", DbName: "app", MaxIdleConns: 11, MaxOpenConns: 10}},
		{name: "invalid ssl", config: &PostgresConfig{Host: "localhost", Port: 5432, UserName: "postgres", DbName: "app", SSL: "invalid"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := normalizeConfig(testCase.config); err == nil {
				t.Fatal("invalid PostgreSQL config must return an error")
			}
		})
	}
}

// TestInitializeRejectsCanceledContextBeforeConnection 验证已取消 Context 不会发起数据库连接。
func TestInitializeRejectsCanceledContextBeforeConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Initialize(ctx, &PostgresConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		UserName: "postgres",
		DbName:   "app",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context error, got: %v", err)
	}
}

// TestFilterModelsRemovesAllNilValuesWithoutChangingOrder 验证相邻 nil Model 不会造成遗漏。
func TestFilterModelsRemovesAllNilValuesWithoutChangingOrder(t *testing.T) {
	first := &testModel{ID: 1}
	second := &testModel{ID: 2}
	var nilModel *testModel

	filtered := filterModels([]interface{}{nil, nilModel, first, nil, second})
	expected := []interface{}{first, second}
	if !reflect.DeepEqual(filtered, expected) {
		t.Fatalf("unexpected filtered models: want=%v got=%v", expected, filtered)
	}
}

// TestApplyPoolConfigUsesValidatedValues 验证连接池配置会完整传递到底层 database/sql。
func TestApplyPoolConfigUsesValidatedValues(t *testing.T) {
	pool := &fakeConnectionPool{}
	config := PostgresConfig{
		MaxIdleConns:    5,
		MaxOpenConns:    15,
		ConnMaxLifetime: 30 * time.Minute,
	}
	applyPoolConfig(pool, config)
	if pool.maxIdleConns != 5 || pool.maxOpenConns != 15 || pool.connMaxLifetime != 30*time.Minute {
		t.Fatalf("unexpected applied pool config: %+v", pool)
	}
}

// TestGetDbInstanceReturnsExplicitErrorBeforeInitialization 验证缺失全局实例时不会返回无错误 nil。
func TestGetDbInstanceReturnsExplicitErrorBeforeInitialization(t *testing.T) {
	databaseMu.Lock()
	previousDatabase := database
	database = nil
	databaseMu.Unlock()
	t.Cleanup(func() {
		databaseMu.Lock()
		database = previousDatabase
		databaseMu.Unlock()
	})

	instance, err := GetDbInstance()
	if err == nil || instance != nil {
		t.Fatalf("uninitialized database must return explicit error: instance=%v error=%v", instance, err)
	}
}
