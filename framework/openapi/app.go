// Package openapi 提供开放 API 入站网关:第三方通过 AppKey + 签名调用业务接口,
// 签名校验、时间戳窗口、nonce 防重放、每 App 限流全部由脚手架完成,
// 业务项目只需像注册普通路由一样注册 handler。
package openapi

import (
	"errors"
	"sync"
)

// Algorithm 签名算法。
type Algorithm string

const (
	// AlgHMAC HMAC-SHA256(对称,AppKey + AppSecret)。
	AlgHMAC Algorithm = "HMAC-SHA256"
	// AlgRSA RSA-SHA256(非对称,我方公钥验签)。
	AlgRSA Algorithm = "RSA-SHA256"
)

// App 第三方应用(开放接口调用方)。
type App struct {
	// AppKey 应用标识(请求头 X-App-Key 携带)。
	AppKey string
	// AppSecret HMAC 算法使用的对称密钥(AlgHMAC 时必填)。
	AppSecret string
	// PublicKey RSA 算法使用的验签公钥(PEM 格式,AlgRSA 时必填)。
	PublicKey string
	// Algorithm 签名算法,默认 AlgHMAC。
	Algorithm Algorithm
	// Enabled 是否启用;false 时网关直接拒绝该应用。
	Enabled bool
	// RatePerSecond 该应用限流速率(每秒令牌数);非正数时默认 100。
	RatePerSecond float64
	// Burst 突发容量;非正数时取 RatePerSecond。
	Burst int
}

// ErrAppExists 应用已注册。
var ErrAppExists = errors.New("openapi: app already registered")

// Registry 第三方应用注册表(内存实现,支持运行时热更新)。
// 生产环境可改为从数据库/配置中心加载,实现相同的注册接口即可。
type Registry struct {
	mu   sync.RWMutex
	apps map[string]*App
}

// NewRegistry 创建空注册表。
func NewRegistry() *Registry {
	return &Registry{apps: make(map[string]*App)}
}

// NewRegistryWith 创建并注册多个应用。
func NewRegistryWith(apps ...*App) *Registry {
	registry := NewRegistry()
	for _, app := range apps {
		_ = registry.Register(app)
	}
	return registry
}

// Register 注册应用;AppKey 重复返回 ErrAppExists。
func (r *Registry) Register(app *App) error {
	if r == nil {
		return errors.New("openapi: registry is nil")
	}
	if app == nil || app.AppKey == "" {
		return errors.New("openapi: app key is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.apps[app.AppKey]; exists {
		return ErrAppExists
	}
	r.apps[app.AppKey] = app
	return nil
}

// Set 注册或整体替换应用(热更新:修改密钥/禁用立即生效)。
func (r *Registry) Set(app *App) {
	if r == nil || app == nil || app.AppKey == "" {
		return
	}
	r.mu.Lock()
	r.apps[app.AppKey] = app
	r.mu.Unlock()
}

// Unregister 移除应用。
func (r *Registry) Unregister(appKey string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.apps, appKey)
	r.mu.Unlock()
}

// Get 查询应用;不存在返回 nil。
func (r *Registry) Get(appKey string) *App {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.apps[appKey]
}

// Apps 返回全部应用副本(管理端展示用)。
func (r *Registry) Apps() []*App {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	apps := make([]*App, 0, len(r.apps))
	for _, app := range r.apps {
		apps = append(apps, app)
	}
	return apps
}
