package datasource

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestNormalizeDriverSupportsCommonAliases 验证 PostgreSQL、MariaDB 和 Oracle 常见别名会归一化。
func TestNormalizeDriverSupportsCommonAliases(t *testing.T) {
	testCases := map[Driver]Driver{
		"pgsql":      DriverPostgreSQL,
		"PostgreSQL": DriverPostgreSQL,
		"mariadb":    DriverMySQL,
		"godror":     DriverOracle,
	}
	for input, expected := range testCases {
		if actual := normalizeDriver(input); actual != expected {
			t.Fatalf("unexpected normalized driver for %q: want=%q got=%q", input, expected, actual)
		}
	}
}

// TestNormalizeDatabaseConfigAppliesDriverDefaults 验证 PostgreSQL 和 MySQL 会获得各自默认参数。
func TestNormalizeDatabaseConfigAppliesDriverDefaults(t *testing.T) {
	postgresConfig, err := normalizeDatabaseConfig(&Config{
		Driver:   DriverPostgreSQL,
		Host:     "127.0.0.1",
		Port:     5432,
		UserName: "postgres",
		Database: "app",
	})
	if err != nil {
		t.Fatalf("normalize PostgreSQL unified config failed: %v", err)
	}
	if postgresConfig.SSLMode != "disable" {
		t.Fatalf("unexpected PostgreSQL SSL default: %s", postgresConfig.SSLMode)
	}

	mysqlConfig, err := normalizeDatabaseConfig(&Config{
		Driver:   DriverMySQL,
		Host:     "127.0.0.1",
		Port:     3306,
		UserName: "root",
		Database: "app",
	})
	if err != nil {
		t.Fatalf("normalize MySQL unified config failed: %v", err)
	}
	if mysqlConfig.Charset != "utf8mb4" {
		t.Fatalf("unexpected MySQL charset default: %s", mysqlConfig.Charset)
	}
}

// TestBuiltInDialectorsAreAvailable 验证 PostgreSQL 可以从统一工厂创建 Dialector。
func TestBuiltInDialectorsAreAvailable(t *testing.T) {
	config := Config{Driver: DriverPostgreSQL, DSN: "host=127.0.0.1 user=test dbname=test sslmode=disable"}
	normalized, err := normalizeDatabaseConfig(&config)
	if err != nil {
		t.Fatalf("normalize PostgreSQL config failed: %v", err)
	}
	dialector, err := createDialector(normalized)
	if err != nil {
		t.Fatalf("create PostgreSQL dialector failed: %v", err)
	}
	if dialector == nil {
		t.Fatal("PostgreSQL dialector is nil")
	}
}

// TestMySQLDSNBuildsEscapedConnectionString 验证统一 MySQL 配置可以生成带时区和超时参数的 DSN。
func TestMySQLDSNBuildsEscapedConnectionString(t *testing.T) {
	dsn, err := MySQLDSN(Config{
		Driver:   DriverMySQL,
		Host:     "127.0.0.1",
		Port:     3306,
		UserName: "root",
		Password: "password",
		Database: "app",
	})
	if err != nil {
		t.Fatalf("build MySQL DSN failed: %v", err)
	}
	if dsn == "" || !strings.Contains(dsn, "charset=utf8mb4") || !strings.Contains(dsn, "loc=Asia%2FShanghai") {
		t.Fatalf("unexpected MySQL DSN: %s", dsn)
	}
}

// TestRegisterDialectorEnablesOracleAdapter 验证 Oracle 可以通过统一注册接口接入选定的 GORM Driver。
func TestRegisterDialectorEnablesOracleAdapter(t *testing.T) {
	dialectorMu.Lock()
	previousFactory := dialectors[DriverOracle]
	dialectorMu.Unlock()
	t.Cleanup(func() {
		dialectorMu.Lock()
		if previousFactory == nil {
			delete(dialectors, DriverOracle)
		} else {
			dialectors[DriverOracle] = previousFactory
		}
		dialectorMu.Unlock()
	})

	expectedFactoryCalled := false
	err := RegisterDialector(DriverOracle, func(config Config) (gorm.Dialector, error) {
		expectedFactoryCalled = true
		if config.Driver != DriverOracle {
			return nil, errors.New("unexpected Oracle driver")
		}
		return postgres.Open("host=127.0.0.1 user=test dbname=test sslmode=disable"), nil
	})
	if err != nil {
		t.Fatalf("register Oracle dialector failed: %v", err)
	}
	_, err = createDialector(Config{Driver: DriverOracle, DSN: "oracle connection string"})
	if err != nil {
		t.Fatalf("create registered Oracle dialector failed: %v", err)
	}
	if !expectedFactoryCalled {
		t.Fatal("registered Oracle factory was not called")
	}
}

// TestRegisterDialectorRejectsInvalidRegistration 验证空驱动名和 nil 工厂不能进入注册表。
func TestRegisterDialectorRejectsInvalidRegistration(t *testing.T) {
	if err := RegisterDialector("", func(Config) (gorm.Dialector, error) { return nil, nil }); err == nil {
		t.Fatal("empty database driver must return an error")
	}
	if err := RegisterDialector("custom", nil); err == nil {
		t.Fatal("nil dialector factory must return an error")
	}
}

// TestDatabaseAddressDoesNotContainCredentials 验证日志地址只包含主机和端口。
func TestDatabaseAddressDoesNotContainCredentials(t *testing.T) {
	config := Config{Driver: DriverMySQL, Host: "db.internal", Port: 3306, UserName: "secret-user", Password: "secret-password"}
	if actual := databaseAddress(config); actual != "db.internal:3306" {
		t.Fatalf("unexpected safe database address: %s", actual)
	}
}
