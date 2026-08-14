package apploader

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Loader 定义配置文件和环境变量的链式加载能力。
// 链式配置阶段发生的错误会被暂存，并在 LoadToStruct 时统一返回给调用方。
type Loader interface {
	// LoadToStruct 将配置文件和环境变量映射到非 nil 结构体指针。
	LoadToStruct(config interface{}) error
	// SetConfigFileSearcher 设置配置文件名称和搜索目录，并立即尝试读取配置文件。
	SetConfigFileSearcher(configName string, searchPath ...string) Loader
	// EnableEnvSearcher 开启环境变量覆盖；非空前缀会生成 PREFIX_SECTION_FIELD 格式的变量名。
	EnableEnvSearcher(envPrefix string) Loader
	// Watch 监听配置文件变更，重载成功（含业务校验）后同步调用 handler。
	// 必须在 LoadToStruct 之后调用；仅支持文件源的变更监听。
	Watch(handler func()) error
}

// Validator 可由业务配置结构实现，用于在反序列化完成后执行必填项和取值范围校验。
type Validator interface {
	// Validate 返回配置不可用于启动的具体原因，不得在错误中包含密码或完整连接串。
	Validate() error
}

// loader 保存独立 Viper 实例和链式配置阶段产生的错误。
// 每个 loader 只应服务于一次配置加载，避免多个应用共享可变配置状态。
type loader struct {
	vConf            *viper.Viper
	envSearchEnable  bool
	configurationErr error
	watchTarget      interface{} // watchTarget 是 Watch 重载时反序列化的目标结构体
}

// NewLoader 创建互不共享状态的配置加载器。
func NewLoader() Loader {
	config := viper.New()
	applyBuiltInDefaults(config)
	return &loader{vConf: config}
}

// applyBuiltInDefaults 设置脚手架内置配置的安全默认值。
// 自定义配置结构只会接收名称匹配的默认值，不会产生额外字段。
func applyBuiltInDefaults(config *viper.Viper) {
	config.SetDefault("web.level", "info")
	config.SetDefault("db.ssl", "disable")
	config.SetDefault("db.maxIdleConns", 10)
	config.SetDefault("db.maxOpenConns", 20)
	config.SetDefault("redis.poolSize", 10)
	config.SetDefault("logConf.outDirPath", ".")
	config.SetDefault("logConf.logLevel", "info")
}

// SetConfigFileSearcher 配置并读取指定文件。
// 缺少名称、搜索路径无效或文件解析失败时，错误会保留到 LoadToStruct 返回。
func (lo *loader) SetConfigFileSearcher(configName string, searchPath ...string) Loader {
	if lo == nil || lo.vConf == nil {
		return lo
	}
	if strings.TrimSpace(configName) == "" {
		lo.appendConfigurationError(errors.New("config file name is empty"))
		return lo
	}

	lo.vConf.SetConfigName(configName)
	for _, path := range searchPath {
		if strings.TrimSpace(path) == "" {
			continue
		}
		lo.vConf.AddConfigPath(path)
	}
	if err := lo.vConf.ReadInConfig(); err != nil {
		lo.appendConfigurationError(fmt.Errorf("read config file %q: %w", configName, err))
	}
	return lo
}

// EnableEnvSearcher 开启环境变量读取并保留调用方设置的前缀。
func (lo *loader) EnableEnvSearcher(envPrefix string) Loader {
	if lo == nil || lo.vConf == nil {
		return lo
	}
	trimmedPrefix := strings.TrimSpace(envPrefix)
	if trimmedPrefix != "" {
		lo.vConf.SetEnvPrefix(trimmedPrefix)
	}
	lo.envSearchEnable = true
	return lo
}

// Watch 监听配置文件变更并重载到 LoadToStruct 使用的同一目标结构体。
// 重载成功（含业务 Validator 校验）后同步调用 handler。
// 必须在 LoadToStruct 成功之后调用；监听启动失败会立即返回错误。
func (lo *loader) Watch(handler func()) error {
	if lo == nil || lo.vConf == nil {
		return errors.New("config loader is nil")
	}
	if lo.watchTarget == nil {
		return errors.New("watch requires LoadToStruct called first")
	}
	if handler == nil {
		return errors.New("watch handler is nil")
	}

	lo.vConf.WatchConfig()
	lo.vConf.OnConfigChange(func(in fsnotify.Event) {
		if err := lo.vConf.Unmarshal(lo.watchTarget); err != nil {
			lo.appendConfigurationError(fmt.Errorf("reload configuration on change: %w", err))
			return
		}
		normalizeBuiltInConfiguration(lo.watchTarget)
		if validator, ok := lo.watchTarget.(Validator); ok {
			if err := validator.Validate(); err != nil {
				lo.appendConfigurationError(fmt.Errorf("validate configuration on change: %w", err))
				return
			}
		}
		handler()
	})
	return nil
}

// LoadToStruct 校验目标、准备环境变量绑定并反序列化配置。
// 文件读取、环境变量绑定和反序列化错误会聚合返回，避免启动时使用半有效配置。
func (lo *loader) LoadToStruct(config interface{}) error {
	if lo == nil || lo.vConf == nil {
		return errors.New("config loader is nil")
	}
	if err := validateConfigTarget(config); err != nil {
		return err
	}

	var loadErrors []error
	if lo.configurationErr != nil {
		loadErrors = append(loadErrors, lo.configurationErr)
	}
	if lo.envSearchEnable {
		if err := lo.prepareEnv(config); err != nil {
			loadErrors = append(loadErrors, err)
		}
	}
	if len(loadErrors) > 0 {
		return errors.Join(loadErrors...)
	}
	if err := lo.vConf.Unmarshal(config); err != nil {
		return fmt.Errorf("unmarshal application configuration: %w", err)
	}
	normalizeBuiltInConfiguration(config)
	if validator, ok := config.(Validator); ok {
		if err := validator.Validate(); err != nil {
			return fmt.Errorf("validate application configuration: %w", err)
		}
	}
	lo.watchTarget = config
	return nil
}

// prepareEnv 为目标结构体的叶子字段绑定环境变量，并启用点号到下划线的转换。
func (lo *loader) prepareEnv(target interface{}) error {
	lo.vConf.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	lo.vConf.AutomaticEnv()

	keys := collectConfigKeys(reflect.TypeOf(target), nil)
	var bindErrors []error
	for _, key := range keys {
		if err := lo.vConf.BindEnv(key); err != nil {
			bindErrors = append(bindErrors, fmt.Errorf("bind environment key %q: %w", key, err))
		}
	}
	return errors.Join(bindErrors...)
}

// collectConfigKeys 按 mapstructure、toml、json 标签优先级递归生成 Viper 配置键。
func collectConfigKeys(targetType reflect.Type, parent []string) []string {
	for targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	if targetType.Kind() != reflect.Struct {
		return nil
	}

	var keys []string
	for index := 0; index < targetType.NumField(); index++ {
		field := targetType.Field(index)
		if field.PkgPath != "" {
			continue
		}
		fieldName := configFieldName(field)
		if fieldName == "-" || fieldName == "" {
			continue
		}
		path := append(append([]string(nil), parent...), fieldName)
		fieldType := field.Type
		for fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() == reflect.Struct && fieldType != reflect.TypeOf(struct{}{}) {
			keys = append(keys, collectConfigKeys(field.Type, path)...)
			continue
		}
		keys = append(keys, strings.Join(path, "."))
	}
	return keys
}

// configFieldName 返回字段用于配置映射的名称。
func configFieldName(field reflect.StructField) string {
	for _, tagName := range []string{"mapstructure", "toml", "json"} {
		if tagValue, ok := field.Tag.Lookup(tagName); ok {
			name := strings.Split(tagValue, ",")[0]
			if name != "" {
				return name
			}
		}
	}
	return field.Name
}

// validateConfigTarget 确保反序列化目标是有效的结构体指针。
func validateConfigTarget(config interface{}) error {
	if config == nil {
		return errors.New("config target is nil")
	}
	value := reflect.ValueOf(config)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return errors.New("config target must be a non-nil pointer")
	}
	if value.Elem().Kind() != reflect.Struct {
		return errors.New("config target must point to a struct")
	}
	return nil
}

// appendConfigurationError 聚合链式配置阶段发生的错误。
func (lo *loader) appendConfigurationError(err error) {
	if err == nil {
		return
	}
	lo.configurationErr = errors.Join(lo.configurationErr, err)
}
