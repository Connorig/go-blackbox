package apploader

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testConfiguration 是 Loader 单元测试使用的最小配置结构。
type testConfiguration struct {
	// Name 是配置文件中的应用名称。
	Name string `mapstructure:"name"`
	// Web 保存嵌套 Web 配置。
	Web testWebConfiguration `mapstructure:"web"`
}

// testWebConfiguration 是环境变量覆盖测试使用的嵌套配置。
type testWebConfiguration struct {
	// Listen 是测试监听地址。
	Listen string `mapstructure:"listen"`
}

// invalidTestConfiguration 模拟实现 Validator 且校验失败的业务配置。
type invalidTestConfiguration struct{}

// Validate 返回固定错误，用于验证 Loader 保留业务校验错误链。
func (invalidTestConfiguration) Validate() error {
	return errInvalidTestConfiguration
}

// errInvalidTestConfiguration 是业务配置校验测试使用的哨兵错误。
var errInvalidTestConfiguration = errors.New("test configuration is invalid")

// TestLoaderReadsConfigurationFile 验证 Loader 能读取 TOML 文件并映射嵌套字段。
func TestLoaderReadsConfigurationFile(t *testing.T) {
	configDirectory := t.TempDir()
	configContent := []byte("name = \"demo\"\n[web]\nlisten = \"127.0.0.1:9528\"\n")
	if err := os.WriteFile(filepath.Join(configDirectory, "config.toml"), configContent, 0o600); err != nil {
		t.Fatalf("write test configuration failed: %v", err)
	}

	var config testConfiguration
	err := NewLoader().
		SetConfigFileSearcher("config", configDirectory).
		LoadToStruct(&config)
	if err != nil {
		t.Fatalf("load test configuration failed: %v", err)
	}
	if config.Name != "demo" || config.Web.Listen != "127.0.0.1:9528" {
		t.Fatalf("unexpected loaded configuration: %+v", config)
	}
}

// TestLoaderReturnsMissingFileError 验证配置文件不存在时错误会返回调用方而不是只写日志。
func TestLoaderReturnsMissingFileError(t *testing.T) {
	var config testConfiguration
	err := NewLoader().
		SetConfigFileSearcher("missing", t.TempDir()).
		LoadToStruct(&config)
	if err == nil {
		t.Fatal("missing configuration file must return an error")
	}
	if !strings.Contains(err.Error(), "read config file") {
		t.Fatalf("missing file error lacks operation context: %v", err)
	}
}

// TestLoaderPreservesEnvironmentPrefix 验证环境变量前缀不会在绑定阶段被清空。
func TestLoaderPreservesEnvironmentPrefix(t *testing.T) {
	t.Setenv("BLACKBOX_WEB_LISTEN", "127.0.0.1:8080")

	var config testConfiguration
	err := NewLoader().
		EnableEnvSearcher("BLACKBOX").
		LoadToStruct(&config)
	if err != nil {
		t.Fatalf("load environment configuration failed: %v", err)
	}
	if config.Web.Listen != "127.0.0.1:8080" {
		t.Fatalf("environment prefix was not applied: %q", config.Web.Listen)
	}
}

// TestLoaderRejectsInvalidTargets 验证 nil、非指针和非结构体目标均返回明确错误。
func TestLoaderRejectsInvalidTargets(t *testing.T) {
	testCases := []struct {
		name   string
		target interface{}
	}{
		{name: "nil target", target: nil},
		{name: "non pointer target", target: testConfiguration{}},
		{name: "pointer to scalar", target: new(string)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := NewLoader().LoadToStruct(testCase.target); err == nil {
				t.Fatal("invalid config target must return an error")
			}
		})
	}
}

// TestBuiltInConfigurationDefaultsAndCompatibility 验证内置默认值和旧拼写字段保持同步。
func TestBuiltInConfigurationDefaultsAndCompatibility(t *testing.T) {
	var config Configuration
	if err := NewLoader().LoadToStruct(&config); err != nil {
		t.Fatalf("load built-in defaults failed: %v", err)
	}
	if config.Web.Level != "info" || config.Db.Ssl != "disable" {
		t.Fatalf("unexpected built-in defaults: %+v", config)
	}
	if config.Db.MaxIdleConns != 10 || config.Db.MaxIdleCones != 10 {
		t.Fatalf("idle connection compatibility fields differ: %+v", config.Db)
	}
	if config.Db.MaxOpenConns != 20 || config.Db.MaxOpenCones != 20 {
		t.Fatalf("open connection compatibility fields differ: %+v", config.Db)
	}
}

// TestConfigurationErrorsRemainDiscoverable 验证聚合后的配置错误仍可通过 errors.Is 定位原始错误。
func TestConfigurationErrorsRemainDiscoverable(t *testing.T) {
	expected := errors.New("configuration failed")
	configLoader := NewLoader().(*loader)
	configLoader.appendConfigurationError(expected)

	var config testConfiguration
	err := configLoader.LoadToStruct(&config)
	if !errors.Is(err, expected) {
		t.Fatalf("configuration error chain was not preserved: %v", err)
	}
}

// TestLoaderRunsBusinessValidation 验证实现 Validator 的配置会在反序列化后执行校验。
func TestLoaderRunsBusinessValidation(t *testing.T) {
	config := invalidTestConfiguration{}
	err := NewLoader().LoadToStruct(&config)
	if !errors.Is(err, errInvalidTestConfiguration) {
		t.Fatalf("business validation error was not preserved: %v", err)
	}
}
