package rbac

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/httptest"
)

// memoryProvider 是测试用内存权限源。
type memoryProvider struct {
	mu               sync.Mutex
	userRoles        map[int64][]string
	rolePerms        map[string][]string
	permissionsCalls int
}

// Permissions 合并用户全部角色的权限点。
func (p *memoryProvider) Permissions(_ context.Context, userID int64) ([]string, error) {
	p.mu.Lock()
	p.permissionsCalls++
	roles := append([]string(nil), p.userRoles[userID]...)
	p.mu.Unlock()
	merged := make(map[string]bool)
	for _, role := range roles {
		for _, permission := range p.rolePerms[role] {
			merged[permission] = true
		}
	}
	result := make([]string, 0, len(merged))
	for permission := range merged {
		result = append(result, permission)
	}
	return result, nil
}

// Roles 返回用户角色。
func (p *memoryProvider) Roles(_ context.Context, userID int64) ([]string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.userRoles[userID]...), nil
}

// testProvider 构造标准测试数据。
func testProvider() *memoryProvider {
	return &memoryProvider{
		userRoles: map[int64][]string{
			1: {"admin"},
			2: {"operator"},
		},
		rolePerms: map[string][]string{
			"admin":    {"order:create", "order:delete", "user:manage"},
			"operator": {"order:create"},
		},
	}
}

// TestHasPermission 验证权限判定与角色合并。
func TestHasPermission(t *testing.T) {
	provider := testProvider()
	enforcer := NewEnforcer(provider)

	ok, err := enforcer.HasPermission(context.Background(), 1, "order:create")
	if err != nil || !ok {
		t.Fatalf("admin must have order:create: ok=%v err=%v", ok, err)
	}
	ok, err = enforcer.HasPermission(context.Background(), 1, "user:manage")
	if err != nil || !ok {
		t.Fatalf("admin must have user:manage: ok=%v err=%v", ok, err)
	}
	ok, err = enforcer.HasPermission(context.Background(), 2, "user:manage")
	if err != nil {
		t.Fatalf("permission check failed: %v", err)
	}
	if ok {
		t.Fatal("operator must not have user:manage")
	}
	ok, err = enforcer.HasPermission(context.Background(), 99, "order:create")
	if err != nil {
		t.Fatalf("unknown user check failed: %v", err)
	}
	if ok {
		t.Fatal("unknown user must not have permissions")
	}
}

// TestHasAnyPermission 验证任一权限判定。
func TestHasAnyPermission(t *testing.T) {
	enforcer := NewEnforcer(testProvider())
	ok, err := enforcer.HasAnyPermission(context.Background(), 2, "order:delete", "order:create")
	if err != nil || !ok {
		t.Fatalf("operator must match order:create: ok=%v err=%v", ok, err)
	}
	ok, err = enforcer.HasAnyPermission(context.Background(), 2, "order:delete", "user:manage")
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if ok {
		t.Fatal("operator must not match any of order:delete/user:manage")
	}
}

// TestHasRole 验证角色判定。
func TestHasRole(t *testing.T) {
	enforcer := NewEnforcer(testProvider())
	ok, err := enforcer.HasRole(context.Background(), 1, "admin")
	if err != nil || !ok {
		t.Fatalf("user 1 must have admin role: ok=%v err=%v", ok, err)
	}
	ok, err = enforcer.HasRole(context.Background(), 2, "admin")
	if err != nil {
		t.Fatalf("role check failed: %v", err)
	}
	if ok {
		t.Fatal("user 2 must not have admin role")
	}
}

// TestPermissionCache 验证权限缓存(命中缓存不再调 provider)。
func TestPermissionCache(t *testing.T) {
	provider := testProvider()
	enforcer := NewEnforcer(provider).WithTTL(time.Minute)

	for index := 0; index < 5; index++ {
		if _, err := enforcer.HasPermission(context.Background(), 1, "order:create"); err != nil {
			t.Fatalf("check failed: %v", err)
		}
	}
	if provider.permissionsCalls != 1 {
		t.Fatalf("provider must be called once with cache, got %d", provider.permissionsCalls)
	}
	// 清缓存后重新加载
	enforcer.ClearCache(1)
	if _, err := enforcer.HasPermission(context.Background(), 1, "order:create"); err != nil {
		t.Fatalf("check after clear failed: %v", err)
	}
	if provider.permissionsCalls != 2 {
		t.Fatalf("provider must reload after clear, got %d", provider.permissionsCalls)
	}
}

// TestRequirePermissionMiddleware 验证声明式权限中间件。
func TestRequirePermissionMiddleware(t *testing.T) {
	enforcer := NewEnforcer(testProvider())
	app := iris.New()
	app.Get("/admin-orders", func(ctx iris.Context) {
		ctx.Values().Set("user_id", int64(1)) // admin
		enforcer.RequirePermission("user:manage")(ctx)
	})
	app.Get("/operator-orders", func(ctx iris.Context) {
		ctx.Values().Set("user_id", int64(2)) // operator
		enforcer.RequirePermission("user:manage")(ctx)
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	e := httptest.New(t, app)
	e.GET("/admin-orders").Expect().Status(200)
	e.GET("/operator-orders").Expect().Status(403).
		JSON().Object().ValueEqual("code", "A0312")
}

// TestRequireRoleMiddleware 验证角色中间件。
func TestRequireRoleMiddleware(t *testing.T) {
	enforcer := NewEnforcer(testProvider())
	app := iris.New()
	app.Get("/role-guarded", func(ctx iris.Context) {
		ctx.Values().Set("user_id", int64(2))
		enforcer.RequireRole("admin")(ctx)
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	e := httptest.New(t, app)
	e.GET("/role-guarded").Expect().Status(403).
		JSON().Object().ValueEqual("code", "A0312")
}

// TestNilEnforcerSafety 验证 nil 判定器安全。
func TestNilEnforcerSafety(t *testing.T) {
	var enforcer *Enforcer
	if _, err := enforcer.HasPermission(context.Background(), 1, "x"); err == nil {
		t.Fatal("nil enforcer must return error")
	}
	if _, err := enforcer.HasRole(context.Background(), 1, "x"); err == nil {
		t.Fatal("nil enforcer role check must return error")
	}
	enforcer = NewEnforcer(nil)
	if _, err := enforcer.Permissions(context.Background(), 1); err == nil {
		t.Fatal("nil provider must return error")
	}
}

// TestConcurrentAccess 验证并发缓存读写安全。
func TestConcurrentAccess(t *testing.T) {
	enforcer := NewEnforcer(testProvider()).WithTTL(time.Minute)
	var waitGroup sync.WaitGroup
	for index := 0; index < 20; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, _ = enforcer.HasPermission(context.Background(), 1, "order:create")
		}()
	}
	waitGroup.Wait()
	if _, err := enforcer.HasPermission(context.Background(), 1, "order:create"); err != nil {
		t.Fatalf("check after concurrency failed: %v", err)
	}
}
