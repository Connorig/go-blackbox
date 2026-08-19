package notify

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// 模板占位符:{{key}} 或 {{ key }}(key 允许字母数字下划线点,支持 a.b 嵌套取值)。
var placeholderPattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.]+)\s*\}\}`)

// templateStore 模板注册中心(进程内,启动时注册,运行期可覆盖)。
var templateStore sync.Map // map[string]string

// RegisterTemplate 注册通知模板(同名覆盖)。
// 占位符语法:{{key}};渲染时从 params 取值,支持 map 嵌套键如 {{user.name}}。
func RegisterTemplate(name, content string) {
	if name == "" {
		return
	}
	templateStore.Store(name, content)
}

// Template 返回已注册模板(不存在返回 false)。
func Template(name string) (string, bool) {
	content, ok := templateStore.Load(name)
	if !ok {
		return "", false
	}
	return content.(string), true
}

// RenderTemplate 按模板名渲染:占位符从 params 取值。
// 模板未注册、参数缺失时返回错误(缺失参数会在错误中列出,便于排查)。
// 嵌套键 {{user.name}} 从 map[string]interface{} 逐层取值。
func RenderTemplate(name string, params map[string]interface{}) (string, error) {
	content, ok := Template(name)
	if !ok {
		return "", fmt.Errorf("notify: template %q not registered", name)
	}
	return renderContent(content, params)
}

// Render 直接渲染模板内容(无需注册)。
func Render(content string, params map[string]interface{}) (string, error) {
	return renderContent(content, params)
}

// renderContent 渲染模板内容。
func renderContent(content string, params map[string]interface{}) (string, error) {
	if content == "" {
		return "", nil
	}
	missing := make(map[string]struct{})
	result := placeholderPattern.ReplaceAllStringFunc(content, func(placeholder string) string {
		key := placeholderPattern.FindStringSubmatch(placeholder)[1]
		value, ok := lookup(params, key)
		if !ok {
			missing[key] = struct{}{}
			return placeholder // 保留占位符,最后统一报错
		}
		return fmt.Sprintf("%v", value)
	})
	if len(missing) > 0 {
		keys := make([]string, 0, len(missing))
		for key := range missing {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return "", fmt.Errorf("notify: template missing params: %s", strings.Join(keys, ", "))
	}
	return result, nil
}

// lookup 从 params 取值:支持嵌套键(点分隔)。
func lookup(params map[string]interface{}, key string) (interface{}, bool) {
	parts := strings.Split(key, ".")
	current := interface{}(params)
	for _, part := range parts {
		mapping, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		value, exists := mapping[part]
		if !exists {
			return nil, false
		}
		current = value
	}
	return current, true
}
