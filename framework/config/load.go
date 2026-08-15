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
	// SetConfigFileSearcher 设置项目(子级)配置文件名称和搜索目录。
	// 文件在 LoadToStruct 时读取；项目配置覆盖全局配置的同名键。
	SetConfigFileSearcher(configName string, searchPath ...string) Loader
	// SetGlobalConfigFile 设置全局(父级)配置文件名称和搜索目录。
	// 全局配置先于项目配置加载，项目配置中的同名键覆盖全局配置(子覆盖父)。
	// 全局文件缺失时仅记录错误，不阻塞加载(可选层)。
	SetGlobalConfigFile(configName string, searchPath ...string) Loader
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
	global           *globalConfig // 全局(父级)配置定位,分层加载用
	configName       string
	configPaths      []string
}

// globalConfig 保存全局(父级)配置定位。
type globalConfig struct {
	name  string
	paths []string
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

// SetConfigFileSearcher 设置项目(子级)配置文件名称和搜索目录。
// 读取延迟到 LoadToStruct 统一执行(与全局配置合并后)。
func (lo *loader) SetConfigFileSearcher(configName string, searchPath ...string) Loader {
	if lo == nil || lo.vConf == nil {
		return lo
	}
	if strings.TrimSpace(configName) == "" {
		lo.appendConfigurationError(errors.New("config file name is empty"))
		return lo
	}
	lo.configName = configName
	lo.configPaths = nil
	for _, path := range searchPath {
		if strings.TrimSpace(path) != "" {
			lo.configPaths = append(lo.configPaths, path)
		}
	}
	return lo
}

// SetGlobalConfigFile 设置全局(父级)配置文件名称和搜索目录。
// 项目配置(SetConfigFileSearcher)中同键的值覆盖全局配置(子覆盖父,键级合并)。
func (lo *loader) SetGlobalConfigFile(configName string, searchPath ...string) Loader {
	if lo == nil || lo.vConf == nil {
		return lo
	}
	if strings.TrimSpace(configName) == "" {
		lo.appendConfigurationError(errors.New("global config file name is empty"))
		return lo
	}
	lo.global = &globalConfig{name: configName}
	for _, path := range searchPath {
		if strings.TrimSpace(path) != "" {
			lo.global.paths = append(lo.global.paths, path)
		}
	}
	return lo
}

// loadLayers 按优先级加载全局(父)与项目(子)配置并键级合并。
// 项目配置读取为主 viper(兼容 Watch 热更新);全局配置经临时 viper 读取。
func (lo *loader) loadLayers() error {
	// 层②:全局配置(父级,低优先级,可选)
	globalSettings := map[string]interface{}{}
	if lo.global != nil && lo.global.name != "" {
		globalViper := viper.New()
		globalViper.SetConfigName(lo.global.name)
		for _, path := range lo.global.paths {
			globalViper.AddConfigPath(path)
		}
		if err := globalViper.ReadInConfig(); err != nil {
			// 全局配置是可选层:文件缺失仅跳过;解析等其他错误仍上报
			var notFound viper.ConfigFileNotFoundError
			if !errors.As(err, &notFound) {
				lo.appendConfigurationError(fmt.Errorf("read global config %q: %w", lo.global.name, err))
			}
		} else {
			globalSettings = globalViper.AllSettings()
		}
	}

	// 层③:项目配置(子级,高优先级)
	projectSettings := map[string]interface{}{}
	if lo.configName != "" {
		// 纯文件读取(临时 viper 无内置默认,避免默认值污染合并结果)
		projectViper := viper.New()
		projectViper.SetConfigName(lo.configName)
		projectViper.SetConfigType("toml")
		for _, path := range lo.configPaths {
			projectViper.AddConfigPath(path)
		}
		if err := projectViper.ReadInConfig(); err != nil {
			lo.appendConfigurationError(fmt.Errorf("read config file %q: %w", lo.configName, err))
		} else {
			projectSettings = projectViper.AllSettings()
		}
		// 主 viper 同样指向项目文件(供 Watch 热更新监听)
		lo.vConf.SetConfigName(lo.configName)
		lo.vConf.SetConfigType("toml")
		for _, path := range lo.configPaths {
			lo.vConf.AddConfigPath(path)
		}
		if err := lo.vConf.ReadInConfig(); err != nil {
			lo.appendConfigurationError(fmt.Errorf("read config file %q: %w", lo.configName, err))
		}
	}

	// 键级合并:项目覆盖全局(递归合并,子键级粒度)
	merged := mergeConfigMaps(globalSettings, projectSettings)
	if len(merged) > 0 {
		lo.vConf.MergeConfigMap(merged)
	}
	return nil
}

// mergeConfigMaps 递归合并两个配置 map,src(高优先级)覆盖 base 的同名键;
// 子 map 按键级合并(base 中 src 未覆盖的键保留)。
func mergeConfigMaps(base, src map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(base)+len(src))
	for key, value := range base {
		result[key] = value
	}
	for key, srcValue := range src {
		if baseValue, exists := result[key]; exists {
			baseMap, baseIsMap := baseValue.(map[string]interface{})
			srcMap, srcIsMap := srcValue.(map[string]interface{})
			if baseIsMap && srcIsMap {
				result[key] = mergeConfigMaps(baseMap, srcMap)
				continue
			}
		}
		result[key] = srcValue
	}
	return result
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

// LoadToStruct 校验目标、分层加载配置、准备环境变量绑定并反序列化。
// 文件读取、环境变量绑定和反序列化错误会聚合返回，避免启动时使用半有效配置。
func (lo *loader) LoadToStruct(config interface{}) error {
	if lo == nil || lo.vConf == nil {
		return errors.New("config loader is nil")
	}
	if err := validateConfigTarget(config); err != nil {
		return err
	}

	// 分层加载:全局(父)与项目(子)配置键级合并,项目覆盖全局
	if err := lo.loadLayers(); err != nil {
		lo.appendConfigurationError(err)
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

// validateConfigTarget 校验配置目标为非 nil 结构体指针。
func validateConfigTarget(config interface{}) error {
	if config == nil {
		return errors.New("config target is nil")
	}
	value := reflect.ValueOf(config)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return errors.New("config target must be a non-nil pointer")
	}
	if value.Elem().Kind() != reflect.Struct {
		return errors.New("config target must point to a struct")
	}
	return nil
}

// configFieldName 按 mapstructure、toml、json 标签优先级提取字段名。
func configFieldName(field reflect.StructField) string {
	for _, tag := range []string{"mapstructure", "toml", "json"} {
		if name := field.Tag.Get(tag); name != "" {
			return strings.Split(name, ",")[0]
		}
	}
	return field.Name
}

// appendConfigurationError 累积链式配置阶段的错误,LoadToStruct 时统一返回。
func (lo *loader) appendConfigurationError(err error) {
	if lo == nil || err == nil {
		return
	}
	if lo.configurationErr == nil {
		lo.configurationErr = err
		return
	}
	lo.configurationErr = errors.Join(lo.configurationErr, err)
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
		if fieldType.Kind() == reflect.Struct {
			keys = append(keys, collectConfigKeys(fieldType, path)...)
		} else {
			keys = append(keys, strings.Join(path, "."))
		}
	}
	return keys
}
