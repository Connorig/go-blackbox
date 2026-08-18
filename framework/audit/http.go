package oplog

import (
	"strconv"

	"github.com/kataras/iris/v12"
	"github.com/redis/go-redis/v9"

	apperr "github.com/Connorig/go-blackbox/component/error"
	web "github.com/Connorig/go-blackbox/framework/web"
)

// QueryHandler 返回审计日志查询 HTTP 处理器。
// 参数:offset(默认 0)、count(默认 20);响应 {code,message,data:{list,total}}。
// 配合 RedisListSink 使用;挂载位置与鉴权中间件由业务决定,
// 建议限制仅管理角色可访问(如叠加 web.Auth + rbac.RequireRole)。
func QueryHandler(client *redis.Client, key string) iris.Handler {
	return func(ctx iris.Context) {
		offset := urlParamInt64(ctx, "offset", 0)
		count := urlParamInt64(ctx, "count", 20)

		entries, err := Query(ctx, client, key, offset, count)
		if err != nil {
			web.Fail(ctx, 500, apperr.CodeSystemError, "query audit logs failed")
			return
		}
		total, err := Count(ctx, client, key)
		if err != nil {
			web.Fail(ctx, 500, apperr.CodeSystemError, "count audit logs failed")
			return
		}
		web.OK(ctx, map[string]interface{}{
			"list":  entries,
			"total": total,
		})
	}
}

// urlParamInt64 解析查询参数;非法或缺省时使用默认值。
func urlParamInt64(ctx iris.Context, name string, defaultValue int64) int64 {
	value := ctx.URLParam(name)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}
