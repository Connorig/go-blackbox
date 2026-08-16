package datasource

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestQuotePGValue 特殊字符密码在字段拼接路径的引用规则。
func TestQuotePGValue(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"plain-password", "plain-password"},   // 安全值不引用
		{"qwertyui#$DTGop", "'qwertyui#$DTGop'"}, // # 触发引用
		{"with space", "'with space'"},         // 空格触发引用
		{"#lead", "'#lead'"},                   // # 触发引用
		{"", "''"},                             // 空值显式引用
	}
	for _, tc := range cases {
		if got := quotePGValue(tc.value); got != tc.want {
			t.Errorf("quotePGValue(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

// TestBuildPostgreSQLDSNSpecialPassword 字段拼接后密码完整可解析:
// 密码含空格/#/$ 时,pgx ParseConfig 解析出的密码必须与配置一致。
func TestBuildPostgreSQLDSNSpecialPassword(t *testing.T) {
	passwords := []string{"p@ss w0rd", "qwertyui#$DTGop", "秘密 密码"}
	for _, password := range passwords {
		dsn := buildPostgreSQLDSN(Config{
			Host: "127.0.0.1", Port: 5432, UserName: "thingple",
			Password: password, Database: "2d3db",
			SSLMode: "disable", TimeZone: "Asia/Shanghai",
		}, 5)
		parsed, err := pgx.ParseConfig(dsn)
		if err != nil {
			t.Fatalf("password=%q: built DSN unparsable: %v\ndsn=%s", password, err, dsn)
		}
		if parsed.Password != password {
			t.Errorf("password=%q: parsed password = %q, dsn=%s", password, parsed.Password, dsn)
		}
	}
}

// TestValidatePostgreSQLDSNSpecialChars # $ 密码直连不误报、空格密码直连报截断。
func TestValidatePostgreSQLDSNSpecialChars(t *testing.T) {
	// # $ 在 pgx key=value 格式安全,校验通过
	if err := validatePostgreSQLDSN("host=127.0.0.1 user=thingple password=qwertyui#$DTGop dbname=2d3db sslmode=disable"); err != nil {
		t.Errorf("#$ password DSN should pass: %v", err)
	}
	// 空格截断:校验必须拦截并提示修复方式
	err := validatePostgreSQLDSN("host=127.0.0.1 user=thingple password=qwe rty dbname=2d3db")
	if err == nil {
		t.Fatal("space-truncated password DSN must be rejected")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("error should mention truncated: %v", err)
	}
	// 错误信息不得泄露密码明文
	if strings.Contains(err.Error(), "qwe") || strings.Contains(err.Error(), "rty") {
		t.Errorf("error leaks password: %v", err)
	}
	// 单引号包裹的密码解析一致,校验通过
	if err := validatePostgreSQLDSN("host=127.0.0.1 user=thingple password='qwe rty' dbname=2d3db"); err != nil {
		t.Errorf("quoted password DSN should pass: %v", err)
	}
	// 语法错误 DSN 报清晰错误
	if err := validatePostgreSQLDSN("host=127.0.0.1 password='unclosed"); err == nil {
		t.Fatal("malformed DSN must be rejected")
	}
}

// TestValidateDatabaseConfigSingleQuotePassword 字段配置含单引号密码必须拒绝(pgx 无法表达)。
func TestValidateDatabaseConfigSingleQuotePassword(t *testing.T) {
	err := validateDatabaseConfig(Config{
		Driver: DriverPostgreSQL, Host: "127.0.0.1", UserName: "u",
		Password: "o'clock", Database: "d", Port: 5432,
		MaxIdleConns: 10, MaxOpenConns: 20, ConnMaxLifetime: 3600, ConnectTimeout: 5,
	})
	if err == nil {
		t.Fatal("single-quote password must be rejected in field config")
	}
	if !strings.Contains(err.Error(), "URL format") {
		t.Errorf("error should suggest URL format: %v", err)
	}
}

// TestNewPostgreSQLDialectorDSNValidation DSN 分支先校验后建连。
func TestNewPostgreSQLDialectorDSNValidation(t *testing.T) {
	if _, err := newPostgreSQLDialector(Config{Driver: DriverPostgreSQL, DSN: "host=127.0.0.1 user=u password=qwe rty dbname=d"}); err == nil {
		t.Fatal("truncated DSN must fail before dialing")
	}
	if _, err := newPostgreSQLDialector(Config{Driver: DriverPostgreSQL, DSN: "host=127.0.0.1 user=u password='qwe rty' dbname=d sslmode=disable"}); err != nil {
		t.Fatalf("valid DSN should build dialector: %v", err)
	}
}
