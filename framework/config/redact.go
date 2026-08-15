package apploader

import (
	"fmt"
	"strings"
)

// Redactor 由业务配置结构实现，输出不包含敏感信息的配置快照。
// 典型用途：启动或热更新时打印脱敏后的配置摘要。
type Redactor interface {
	// Redact 返回可安全输出的配置快照；密码、Token 等敏感字段必须打码。
	Redact() map[string]interface{}
}

// Redact 返回配置的安全快照；配置未实现 Redactor 时返回类型名占位。
func Redact(config interface{}) interface{} {
	if redactor, ok := config.(Redactor); ok {
		return redactor.Redact()
	}
	if config == nil {
		return nil
	}
	return fmt.Sprintf("%T (no redactor)", config)
}

// EnvFile 返回按环境约定的配置文件名：
// env 为空时返回 baseName，否则返回 baseName.{env}（如 config.prod）。
// 配合 SetConfigFileSearcher 使用：loader.SetConfigFileSearcher(EnvFile("config", env), ".")
func EnvFile(baseName, env string) string {
	trimmedEnv := strings.TrimSpace(env)
	if trimmedEnv == "" {
		return strings.TrimSpace(baseName)
	}
	return strings.TrimSpace(baseName) + "." + trimmedEnv
}
