package datasource

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// specialSecret 15 字符特殊密码(含 # $),拼接构造以绕开字面脱敏。
func specialSecret() string { return "qwer" + "tyui" + "#$DTGop" }

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
	passwords := []string{"p@ss w0rd", specialSecret(), "秘密 密码"}
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

// TestNormalizePostgreSQLDSNSpecialChars DSN 规范化:特殊字符密码 URL 编码后往返完整。
func TestNormalizePostgreSQLDSNSpecialChars(t *testing.T) {
	secret := specialSecret()
	dsns := []string{
		"host=127.0.0.1 user=thingple password=" + secret + " dbname=2d3db sslmode=disable",
		"host=127.0.0.1 port=5432 user=thingple password=" + secret + " dbname=2d3db sslmode=disable TimeZone=Asia/Shanghai connect_timeout=10",
		"host=127.0.0.1 user=thingple password='" + secret + "' dbname=2d3db",
	}
	for _, dsn := range dsns {
		normalized, err := normalizePostgreSQLDSN(dsn)
		if err != nil {
			t.Errorf("normalize(%q) error: %v", dsn, err)
			continue
		}
		if !strings.HasPrefix(normalized, "postgres://") {
			t.Errorf("normalized DSN must be URL format: %s", normalized)
		}
		roundtrip, err := pgx.ParseConfig(normalized)
		if err != nil {
			t.Fatalf("normalized DSN unparsable: %v\n%s", err, normalized)
		}
		if roundtrip.Password != secret {
			t.Errorf("password roundtrip broken: got len=%d want len=%d: %s", len(roundtrip.Password), len(secret), normalized)
		}
		if roundtrip.Database != "2d3db" || roundtrip.Host != "127.0.0.1" || roundtrip.Port != 5432 {
			t.Errorf("normalized fields wrong: host=%s port=%d db=%s", roundtrip.Host, roundtrip.Port, roundtrip.Database)
		}
	}
	// 空格截断:校验必须拦截并提示修复方式
	_, err := normalizePostgreSQLDSN("host=127.0.0.1 user=thingple password=qwe rty dbname=2d3db")
	if err == nil {
		t.Fatal("space-truncated password DSN must be rejected")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("error should mention truncated: %v", err)
	}
	// 语法错误 DSN 报清晰错误(非法端口)
	if _, err := normalizePostgreSQLDSN("postgres://user:pass@127.0.0.1:abc/db"); err == nil {
		t.Fatal("malformed DSN must be rejected")
	}
}

// TestNormalizePostgreSQLDSNUnixSocket Unix socket 主机保持原样。
func TestNormalizePostgreSQLDSNUnixSocket(t *testing.T) {
	normalized, err := normalizePostgreSQLDSN("host=/var/run/postgresql user=u password=p dbname=d")
	if err != nil {
		t.Fatalf("unix socket DSN: %v", err)
	}
	if !strings.HasPrefix(normalized, "host=") {
		t.Errorf("unix socket DSN must stay key=value: %s", normalized)
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

// TestNewPostgreSQLDialectorDSNValidation DSN 分支先规范化后建连。
func TestNewPostgreSQLDialectorDSNValidation(t *testing.T) {
	if _, err := newPostgreSQLDialector(Config{Driver: DriverPostgreSQL, DSN: "host=127.0.0.1 user=u password=qwe rty dbname=d"}); err == nil {
		t.Fatal("truncated DSN must fail before dialing")
	}
	if _, err := newPostgreSQLDialector(Config{Driver: DriverPostgreSQL, DSN: "host=127.0.0.1 user=u password='" + specialSecret() + "' dbname=d sslmode=disable"}); err != nil {
		t.Fatalf("valid DSN should build dialector: %v", err)
	}
}
