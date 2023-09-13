package loader

import (
	"github.com/fatih/structs"
	"github.com/jeremywohl/flatten"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"strings"
)

// Loader 定义加载器-解析配置文件
type Loader interface {
	LoadToStruct(config interface{}) error                                // 将解析的配置文件值、环境变量值映射到 配置结构体中
	SetConfigFileSearcher(configName string, searchPath ...string) Loader // 设置配置文件名称，路径多个
	EnableEnvSearcher(envPrefix string) Loader                            // 开启读取环境变量，设置环境变量前缀可选
}

type loader struct {
	vConf           *viper.Viper
	envSearchEnable bool
}

// 初始化配置
func NewLoader() (o Loader) {
	config := viper.New()
	o = &loader{
		vConf: config,
	}
	return
}

func (lo *loader) SetConfigFileSearcher(configName string, searchPath ...string) Loader {
	lo.vConf.SetConfigName(configName)

	if len(searchPath) > 0 {
		for _, p := range searchPath {
			lo.vConf.AddConfigPath(p)
		}
	}

	// 读取配置文件
	err := lo.vConf.ReadInConfig()
	if err != nil {
		logrus.Errorf("%s", err)
	}
	return lo
}
func (lo *loader) EnableEnvSearcher(envPrefix string) Loader {

	if len(envPrefix) > 0 {
		lo.vConf.SetEnvPrefix(envPrefix)
	}
	// 开启环境变量读取
	lo.envSearchEnable = true
	return lo
}
func (t *loader) prepareEnv(target interface{}) Loader {
	t.vConf.AutomaticEnv()

	t.vConf.SetEnvPrefix("")
	t.vConf.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	confMap := structs.Map(target)
	flat, err := flatten.Flatten(confMap, "", flatten.DotStyle)
	if err != nil {
		return t
	}

	for key := range flat {
		err = t.vConf.BindEnv(key)
		if err != nil {
			return t
		}
	}
	return t
}

func (lo *loader) LoadToStruct(config interface{}) (err error) {

	// 开启环境变量读取
	if lo.envSearchEnable {
		// 按照配置类读取环境变量
		lo.prepareEnv(config)
	}
	// 将读取的值赋值到 配置类中
	err = lo.vConf.Unmarshal(config)

	return err
}
