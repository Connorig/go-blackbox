package datasource

import (
	"context"

	"gorm.io/gorm"
)

// DataScope 数据权限范围(组织/部门维度)。
// 与公共模型 OrgFields(org_id / dept_id)配套:业务查询时带上数据范围,
// 自动为 SQL 追加 org_id / dept_id 过滤条件,实现组织间数据隔离。
//
// 典型链路:
//  1. 登录签发 token 时写入组织身份(apptoken.GenTokenFull)
//  2. webiris.Auth 认证后通过 webiris.DataScope(ctx) 取回
//  3. 查询时 Scopes(scope.Condition()) 自动过滤
type DataScope struct {
	OrgID  int64 // 组织 ID(0 表示不限制)
	DeptID int64 // 部门 ID(0 表示不限制)
}

// IsEmpty 是否未设置任何范围(空范围不产生过滤条件)。
func (s DataScope) IsEmpty() bool {
	return s.OrgID == 0 && s.DeptID == 0
}

// Condition 生成 GORM scope 过滤条件(字段名对齐 OrgFields 的 org_id/dept_id)。
// 仅对非零字段追加条件,零值字段不限制。
//
//	db.WithContext(ctx).Scopes(scope.Condition()).Find(&orders)
func (s DataScope) Condition() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.OrgID != 0 {
			db = db.Where("org_id = ?", s.OrgID)
		}
		if s.DeptID != 0 {
			db = db.Where("dept_id = ?", s.DeptID)
		}
		return db
	}
}

// ConditionFor 生成使用自定义列名的过滤条件(业务表字段命名特殊时使用)。
func (s DataScope) ConditionFor(orgColumn, deptColumn string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.OrgID != 0 && orgColumn != "" {
			db = db.Where(orgColumn+" = ?", s.OrgID)
		}
		if s.DeptID != 0 && deptColumn != "" {
			db = db.Where(deptColumn+" = ?", s.DeptID)
		}
		return db
	}
}

// scopeContextKey 是 DataScope 在 context 中的键。
type scopeContextKey struct{}

// WithScope 将数据权限范围写入 context。
// 中间件/拦截器在请求入口注入,业务代码通过 ScopeFrom 读取。
func WithScope(ctx context.Context, scope DataScope) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, scopeContextKey{}, scope)
}

// ScopeFrom 从 context 读取数据权限范围;未注入时返回 (零值, false)。
// ctx 为 nil 时安全返回 (零值, false)。
func ScopeFrom(ctx context.Context) (DataScope, bool) {
	if ctx == nil {
		return DataScope{}, false
	}
	scope, ok := ctx.Value(scopeContextKey{}).(DataScope)
	return scope, ok
}

// MustScope 从 context 读取数据权限范围;未注入时返回零值(不限制)。
// 用于「范围可选」的业务场景,避免到处判断 ok。
func MustScope(ctx context.Context) DataScope {
	scope, _ := ScopeFrom(ctx)
	return scope
}
