# RBAC 权限指南(RBAC_GUIDELINES)

`framework/rbac` 提供角色权限判定层,与 JWT scope 互补:
**scope 控制「API 组访问」,rbac 控制「业务操作粒度」**。

## 一、权限模型

```
用户(User) ──多──> 角色(Role) ──多──> 权限点(Permission)

权限点命名:资源:动作,如 order:create / order:delete / user:manage
角色示例:admin(全部)、operator(order:create,order:view)
```

## 二、业务实现 Provider

```go
type permissionProvider struct{ db *gorm.DB }

// Permissions 合并用户全部角色的权限点(通常 join 查询)
func (p *permissionProvider) Permissions(ctx context.Context, userID int64) ([]string, error) {
    var permissions []string
    err := p.db.Table("user_role").
        Select("DISTINCT rp.permission").
        Joins("JOIN role_permission rp ON rp.role = user_role.role").
        Where("user_role.user_id = ?", userID).Find(&permissions).Error
    return permissions, err
}

// Roles 返回用户角色
func (p *permissionProvider) Roles(ctx context.Context, userID int64) ([]string, error) {
    var roles []string
    err := p.db.Table("user_role").Where("user_id = ?", userID).Pluck("role", &roles).Error
    return roles, err
}
```

## 三、启动装配

```go
// 1) 创建判定器(带缓存 TTL,默认 60s)
enforcer := rbac.NewEnforcer(&permissionProvider{db: db}).WithTTL(time.Minute)

// 2) 注册到容器,业务侧使用
simpleioc.RegisterInstance(enforcer)   // 或 gbxioc 等价 API

// 3) Web 层:Auth 之后叠加声明式拦截
app.Use(webiris.Auth(webiris.AuthConfig{Whitelist: []string{"/health"}}))
app.Get("/api/v1/orders/delete", func(ctx iris.Context) {
    enforcer.RequirePermission("order:delete")(ctx)   // 403 A0312
    // ...业务
})
// 或分组拦截:
party := app.Party("/api/v1/admin", webiris.Auth(), enforcer.RequireRole("admin"))
```

## 四、业务代码判定(非 Web 场景)

```go
ok, err := enforcer.HasPermission(ctx, userID, "order:delete")
ok, err = enforcer.HasAnyPermission(ctx, userID, "order:view", "order:delete")
ok, err = enforcer.HasRole(ctx, userID, "admin")
```

## 五、缓存与角色变更

- 权限/角色缓存 TTL(默认 60s),`WithTTL(0)` 禁用缓存(强一致)
- 角色变更后立即生效:`enforcer.ClearCache(userID)`
- 多实例部署:各实例独立缓存,TTL 内短暂不一致可接受;强一致场景禁用缓存或缩短 TTL

## 六、数据权限(组织/部门隔离)

RBAC 解决「能不能做」,数据范围由 data 权限解决(见 DATABASE_STANDARDS 数据权限章节):

```go
// Auth 中间件已注入 DataScope(ctx);查询时叠加
db.Scopes(database.DataScope(ctx).Condition()).Find(&orders)
```

## 七、最佳实践

1. 权限点命名统一 `资源:动作`,集中在业务常量表,禁止魔法字符串
2. Web 层用 RequirePermission 快速拦截;业务内敏感操作二次判定(纵深防御)
3. Provider 查询走主库或从库均可,失败时 Enforcer 返回 500 并记录日志(默认拒绝策略)
4. 越权防护:资源级校验(订单归属)在业务层完成,RBAC 不替代资源归属校验
