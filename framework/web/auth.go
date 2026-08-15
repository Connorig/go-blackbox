package webiris

import (
	"strings"

	"github.com/Connorig/go-blackbox/component/auth/token"
	"github.com/kataras/iris/v12"
)

// AuthConfig 定义 JWT 认证中间件配置。
type AuthConfig struct {
	// Whitelist 是无需认证的路径前缀列表（如 /health、/login）。
	// 路径以任一前缀开头时直接放行。
	Whitelist []string
	// Scope 是本接口要求的权限标识（逗号分隔匹配 token 声明的 scope）。
	// 非空时，token 声明未包含该 scope 的请求返回 403。
	Scope string
}

// Auth 返回 JWT 认证中间件。
// 校验 Authorization: Bearer <token> 头（使用 apptoken.VerifyToken，
// 需先通过 apptoken.SetSecretKey 注入密钥）；认证通过后把用户身份写入
// ctx.Values() 的 user_id / user_email，业务代码通过 UserID / UserEmail 读取。
// 认证失败返回 401 统一响应；配置 Scope 时无权限返回 403 统一响应。
// 用法：app.Use(webiris.Auth(webiris.AuthConfig{Whitelist: []string{"/health", "/login"}}))
func Auth(config ...AuthConfig) iris.Handler {
	authConfig := AuthConfig{}
	if len(config) > 0 {
		authConfig = config[0]
	}
	return func(ctx iris.Context) {
		path := ctx.Path()
		for _, prefix := range authConfig.Whitelist {
			if prefix != "" && strings.HasPrefix(path, prefix) {
				ctx.Next()
				return
			}
		}

		const bearerPrefix = "Bearer "
		header := ctx.GetHeader("Authorization")
		if !strings.HasPrefix(header, bearerPrefix) {
			Fail(ctx, iris.StatusUnauthorized, iris.StatusUnauthorized, "missing or malformed authorization header")
			ctx.StopExecution()
			return
		}

		token := strings.TrimPrefix(header, bearerPrefix)
		claim, err := apptoken.VerifyToken(token)
		if err != nil {
			Fail(ctx, iris.StatusUnauthorized, iris.StatusUnauthorized, "invalid or expired token")
			ctx.StopExecution()
			return
		}

		if authConfig.Scope != "" && !hasScope(claim.Scope, authConfig.Scope) {
			Fail(ctx, iris.StatusForbidden, iris.StatusForbidden, "insufficient scope")
			ctx.StopExecution()
			return
		}

		ctx.Values().Set("user_id", claim.UserID)
		ctx.Values().Set("user_email", claim.UserEmail)
		ctx.Next()
	}
}

// hasScope 判断逗号分隔的 scope 列表中是否包含期望值。
func hasScope(scopeList, expected string) bool {
	for _, scope := range strings.Split(scopeList, ",") {
		if strings.TrimSpace(scope) == expected {
			return true
		}
	}
	return false
}

// UserID 从上下文读取认证用户 ID；未认证或缺失时返回 0。
func UserID(ctx iris.Context) int64 {
	value := ctx.Values().Get("user_id")
	if userID, ok := value.(int64); ok {
		return userID
	}
	return 0
}

// UserEmail 从上下文读取认证用户邮箱；未认证或缺失时返回空字符串。
func UserEmail(ctx iris.Context) string {
	return ctx.Values().GetString("user_email")
}
