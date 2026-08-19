// Package i18n 提供轻量国际化组件:
// 多语言消息资源(JSON 文件按语言加载)、key 翻译、占位符参数、
// 请求语言检测(Accept-Language 解析)。
// 设计对标 Java ResourceBundle + Spring 的 MessageSource。
package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// 默认语言(缺省翻译时使用)。
const DefaultLang = "zh-CN"

// 占位符:{{key}};与 notify 模板语法一致。
var placeholderPattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.]+)\s*\}\}`)

// Bundle 多语言资源包(协程安全)。
type Bundle struct {
	mu       sync.RWMutex
	messages map[string]map[string]string // lang -> key -> message
	fallback string
}

// NewBundle 创建资源包;默认回退语言 zh-CN。
func NewBundle() *Bundle {
	return &Bundle{
		messages: make(map[string]map[string]string),
		fallback: DefaultLang,
	}
}

// SetFallback 设置回退语言(缺省翻译时使用)。
func (b *Bundle) SetFallback(lang string) *Bundle {
	if b != nil && lang != "" {
		b.fallback = lang
	}
	return b
}

// Register 注册语言资源(同名 key 覆盖)。
func (b *Bundle) Register(lang string, messages map[string]string) {
	if b == nil || lang == "" || len(messages) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	table := b.messages[lang]
	if table == nil {
		table = make(map[string]string)
		b.messages[lang] = table
	}
	for key, message := range messages {
		table[key] = message
	}
}

// LoadDir 从目录加载语言资源文件(langs/zh-CN.json、en-US.json 等)。
// 文件名为语言标识(不含扩展名);内容为 {"key":"message"} 映射。
func (b *Bundle) LoadDir(dir string) error {
	if b == nil {
		return fmt.Errorf("i18n: bundle is nil")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("i18n: read dir %q: %w", dir, err)
	}
	loaded := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		lang := strings.TrimSuffix(name, filepath.Ext(name))
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("i18n: read %q: %w", path, err)
		}
		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			return fmt.Errorf("i18n: parse %q: %w", path, err)
		}
		b.Register(lang, messages)
		loaded++
	}
	if loaded == 0 {
		return fmt.Errorf("i18n: no language files found in %q", dir)
	}
	return nil
}

// Langs 返回已注册语言列表(排序)。
func (b *Bundle) Langs() []string {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	langs := make([]string, 0, len(b.messages))
	for lang := range b.messages {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	return langs
}

// Has 判断语言是否已注册。
func (b *Bundle) Has(lang string) bool {
	if b == nil || lang == "" {
		return false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.messages[lang]
	return ok
}

// T 翻译:按 lang 查找,缺失时回退默认语言,再缺失返回 key 本身。
// params 可选:{{key}} 占位符替换(与 notify 模板一致,支持嵌套键)。
func (b *Bundle) T(lang, key string, params ...map[string]interface{}) string {
	if b == nil || key == "" {
		return key
	}
	b.mu.RLock()
	message, ok := b.messages[lang][key]
	if !ok && lang != b.fallback {
		message, ok = b.messages[b.fallback][key]
	}
	b.mu.RUnlock()
	if !ok {
		return key
	}
	if len(params) > 0 && len(params[0]) > 0 {
		if rendered, err := render(message, params[0]); err == nil {
			return rendered
		}
	}
	return message
}

// Tf 翻译并格式化参数(fmt.Sprintf 风格,message 中 %s/%d 占位)。
func (b *Bundle) Tf(lang, key string, args ...interface{}) string {
	message := b.T(lang, key)
	if len(args) == 0 || !strings.Contains(message, "%") {
		return message
	}
	return fmt.Sprintf(message, args...)
}

// render 渲染占位符(缺失参数保留原占位符,不报错——与 notify 策略不同,
// 翻译场景兜底优先)。
func render(message string, params map[string]interface{}) (string, error) {
	result := placeholderPattern.ReplaceAllStringFunc(message, func(placeholder string) string {
		key := placeholderPattern.FindStringSubmatch(placeholder)[1]
		value, ok := lookup(params, key)
		if !ok {
			return placeholder
		}
		return fmt.Sprintf("%v", value)
	})
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

// DetectLanguage 从 Accept-Language 头解析语言(zh-CN,zh;q=0.9 → zh-CN)。
// 未匹配时返回回退语言;仅取第一个有效语言标识,大小写归一(zh-cn → zh-CN)。
func (b *Bundle) DetectLanguage(acceptLanguage string) string {
	if b == nil {
		return DefaultLang
	}
	if acceptLanguage == "" {
		return b.fallback
	}
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if tag == "" {
			continue
		}
		normalized := normalize(tag)
		if b.Has(normalized) {
			return normalized
		}
	}
	return b.fallback
}

// normalize 归一化语言标识:zh-cn → zh-CN;en-US 保持。
func normalize(tag string) string {
	parts := strings.SplitN(tag, "-", 2)
	if len(parts) == 1 {
		return strings.ToLower(parts[0])
	}
	return strings.ToLower(parts[0]) + "-" + strings.ToUpper(parts[1])
}
