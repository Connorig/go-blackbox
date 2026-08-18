// Package rbac 提供角色权限访问控制(RBAC)判定层。
// 与 JWT scope 粗粒度互补:scope 控制「API 组访问」,rbac 控制「业务操作粒度」。
// 用法:业务实现 Provider(从 DB/Redis 加载用户角色与权限点),
// Enforcer 带缓存判定;Web 层用 RequirePermission/RequireRole 声明式拦截。
package rbac

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kataras/iris/v12"

	apperr "github.com/Connorig/go-blackbox/component/error"
	web "github.com/Connorig/go-blackbox/framework/web"
)

// Provider 是权限数据源抽象,业务实现(通常查询数据库):
// Permissions 返回用户全部权限点(如 "order:create"、"user:delete"),合并其所有角色;
// Roles 返回用户角色列表(如 "admin"、"operator")。
type Provider interface {
	Permissions(ctx context.Context, userID int64) ([]string, error)
	Roles(ctx context.Context, userID int64) ([]string, error)
}

// cacheEntry 是权限缓存条目。
type cacheEntry struct {
	permissions []string
	roles       []string
	expiresAt   time.Time
}

// Enforcer 是带内存缓存的权限判定器。
type Enforcer struct {
	provider Provider
	mu       sync.RWMutex
	cache    map[int64]cacheEntry
	ttl      time.Duration
}

// NewEnforcer 创建判定器;provider 为 nil 时所有判定返回错误。
func NewEnforcer(provider Provider) *Enforcer {
	return &Enforcer{
		provider: provider,
		cache:    make(map[int64]cacheEntry),
		ttl:      60 * time.Second,
	}
}

// WithTTL 设置权限缓存时间(默认 60 秒;非正数禁用缓存)。
func (e *Enforcer) WithTTL(ttl time.Duration) *Enforcer {
	if e != nil {
		e.ttl = ttl
	}
	return e
}

// Permissions 返回用户权限点(带缓存)。
func (e *Enforcer) Permissions(ctx context.Context, userID int64) ([]string, error) {
	if e == nil || e.provider == nil {
		return nil, errors.New("rbac: provider is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if entry, ok := e.cacheLookup(userID); ok {
		return entry.permissions, nil
	}
	permissions, err := e.provider.Permissions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("rbac: load permissions for user %d: %w", userID, err)
	}
	roles, _ := e.provider.Roles(ctx, userID) // 角色加载失败不阻塞权限判定
	e.cacheStore(userID, permissions, roles)
	return permissions, nil
}

// HasPermission 判断用户是否拥有指定权限点。
func (e *Enforcer) HasPermission(ctx context.Context, userID int64, permission string) (bool, error) {
	permissions, err := e.Permissions(ctx, userID)
	if err != nil {
		return false, err
	}
	return contains(permissions, permission), nil
}

// HasAnyPermission 判断用户是否拥有任一权限点。
func (e *Enforcer) HasAnyPermission(ctx context.Context, userID int64, permissions ...string) (bool, error) {
	owned, err := e.Permissions(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, permission := range permissions {
		if contains(owned, permission) {
			return true, nil
		}
	}
	return false, nil
}

// HasRole 判断用户是否拥有指定角色。
func (e *Enforcer) HasRole(ctx context.Context, userID int64, role string) (bool, error) {
	if e == nil || e.provider == nil {
		return false, errors.New("rbac: provider is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if entry, ok := e.cacheLookup(userID); ok {
		return contains(entry.roles, role), nil
	}
	if _, err := e.Permissions(ctx, userID); err != nil {
		return false, err
	}
	entry, _ := e.cacheLookup(userID)
	return contains(entry.roles, role), nil
}

// RequirePermission 返回中间件:用户缺少全部指定权限时 403(A0312)。
// 依赖 web.Auth 已写入 user_id;未认证时由 Auth 先行拦截。
func (e *Enforcer) RequirePermission(permissions ...string) iris.Handler {
	return func(ctx iris.Context) {
		userID := web.UserID(ctx)
		ok, err := e.HasAnyPermission(ctx, userID, permissions...)
		if err != nil {
			web.Fail(ctx, 500, apperr.CodeSystemError, "load user permissions failed")
			ctx.StopExecution()
			return
		}
		if !ok {
			web.Fail(ctx, 403, apperr.CodeAPINoPermission, "insufficient permission")
			ctx.StopExecution()
			return
		}
		ctx.Next()
	}
}

// RequireRole 返回中间件:用户缺少全部指定角色时 403(A0312)。
func (e *Enforcer) RequireRole(roles ...string) iris.Handler {
	return func(ctx iris.Context) {
		userID := web.UserID(ctx)
		ok, err := e.HasAnyRole(ctx, userID, roles...)
		if err != nil {
			web.Fail(ctx, 500, apperr.CodeSystemError, "load user roles failed")
			ctx.StopExecution()
			return
		}
		if !ok {
			web.Fail(ctx, 403, apperr.CodeAPINoPermission, "insufficient role")
			ctx.StopExecution()
			return
		}
		ctx.Next()
	}
}

// HasAnyRole 判断用户是否拥有任一角色。
func (e *Enforcer) HasAnyRole(ctx context.Context, userID int64, roles ...string) (bool, error) {
	if e == nil || e.provider == nil {
		return false, errors.New("rbac: provider is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if entry, ok := e.cacheLookup(userID); ok {
		for _, role := range roles {
			if contains(entry.roles, role) {
				return true, nil
			}
		}
		return false, nil
	}
	if _, err := e.Permissions(ctx, userID); err != nil {
		return false, err
	}
	entry, _ := e.cacheLookup(userID)
	for _, role := range roles {
		if contains(entry.roles, role) {
			return true, nil
		}
	}
	return false, nil
}

// ClearCache 清除指定用户权限缓存(角色变更后调用)。
func (e *Enforcer) ClearCache(userID int64) {
	if e == nil {
		return
	}
	e.mu.Lock()
	delete(e.cache, userID)
	e.mu.Unlock()
}

// cacheLookup 读取有效缓存。
func (e *Enforcer) cacheLookup(userID int64) (cacheEntry, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	entry, ok := e.cache[userID]
	if !ok {
		return cacheEntry{}, false
	}
	if e.ttl > 0 && time.Now().After(entry.expiresAt) {
		delete(e.cache, userID)
		return cacheEntry{}, false
	}
	return entry, true
}

// cacheStore 写入缓存。
func (e *Enforcer) cacheStore(userID int64, permissions, roles []string) {
	if e.ttl <= 0 {
		return
	}
	e.mu.Lock()
	e.cache[userID] = cacheEntry{
		permissions: append([]string(nil), permissions...),
		roles:       append([]string(nil), roles...),
		expiresAt:   time.Now().Add(e.ttl),
	}
	e.mu.Unlock()
}

// contains 判断切片是否包含指定值。
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
