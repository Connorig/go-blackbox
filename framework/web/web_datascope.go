package webiris

import (
	"github.com/Connorig/go-blackbox/framework/database"
	"github.com/kataras/iris/v12"
)

// 用户组织身份上下文键(由 Auth 中间件写入)。
const (
	contextKeyUserOrgID  = "user_org_id"
	contextKeyUserDeptID = "user_dept_id"
)

// UserOrgID 从上下文读取认证用户的组织 ID;未认证或缺失时返回 0。
func UserOrgID(ctx iris.Context) int64 {
	value := ctx.Values().Get(contextKeyUserOrgID)
	if orgID, ok := value.(int64); ok {
		return orgID
	}
	return 0
}

// UserDeptID 从上下文读取认证用户的部门 ID;未认证或缺失时返回 0。
func UserDeptID(ctx iris.Context) int64 {
	value := ctx.Values().Get(contextKeyUserDeptID)
	if deptID, ok := value.(int64); ok {
		return deptID
	}
	return 0
}

// DataScope 从上下文读取认证用户的数据权限范围(组织/部门)。
// 与 framework/database 的 DataScope 配套:登录时通过 apptoken.GenTokenFull
// 写入组织身份,认证后此处取回,查询时直接 Scopes(scope.Condition()) 隔离数据。
// 未认证或 token 无组织字段时返回空范围(不限制)。
func DataScope(ctx iris.Context) datasource.DataScope {
	return datasource.DataScope{
		OrgID:  UserOrgID(ctx),
		DeptID: UserDeptID(ctx),
	}
}
