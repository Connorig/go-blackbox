# web-basic 示例(全家桶版)

展示 go-blackbox v1.17 全部核心能力的最小完整应用:安全基线、JWT 认证与数据权限、
开放平台、出站调用与熔断、资源监控与告警、Admin 管理。

## 运行

```bash
go run ./examples/web-basic
```

| 端口 | 说明 |
| --- | --- |
| `:9528` | 业务 Web |
| `:6060` | Admin 管理服务(pprof/metrics/日志级别) |

## 验证路径

```bash
# 1. 健康探针
curl http://localhost:9528/health/live
curl http://localhost:9528/health/ready

# 2. SQL 注入拦截(命中返回 400 A0400)
curl "http://localhost:9528/api/v1/users?x=1'%20OR%20'1'='1"

# 3. 登录获取 token(携带 scope + 组织身份 org 101/dept 10101)
curl -X POST http://localhost:9528/api/v1/login
# → data.access_token 用于后续请求

# 4. 携带 token 访问认证接口
curl -H "Authorization: Bearer <access_token>" http://localhost:9528/api/v1/me

# 5. 数据权限隔离:登录 token 携带 org 101 + dept 10101,查询自动过滤,仅返回本部门数据(org1-a)\n#    (若想返回整个组织,签发 token 时 DeptID 传 0 即可)
curl -H "Authorization: Bearer <access_token>" http://localhost:9528/api/v1/users

# 6. 资源监控页面(浏览器打开)
open http://localhost:9528/monitor

# 7. 监控数据接口(未认证 → 401;监控接口限流防轰炸)
curl http://localhost:9528/monitor/api/stats

# 8. 出站调用示例:签名客户端 + 熔断器(内部调用自身监控接口)
curl -H "Authorization: Bearer <access_token>" http://localhost:9528/api/v1/partner

# 9. Admin:Prometheus 指标 / pprof / 运行时日志级别
curl http://localhost:6060/metrics
curl http://localhost:6060/debug/pprof/goroutine?debug=1
curl -X POST -d '{"level":"debug"}' http://localhost:6060/cl
curl http://localhost:6060/ops/demo
```

## 开放平台调用(第三方签名)

开放接口 `/openapi/v1/order/query`(GET)与 `/openapi/v1/order/update`(POST),
注册应用:AppKey `partner-001`,AppSecret `change-me-secret`,算法 HMAC-SHA256。
签名规范与生成脚本见 [OPENAPI_GUIDELINES.md](../../docs/OPENAPI_GUIDELINES.md)(examples/openapi 的 README 有 PowerShell 签名示例)。

## 告警

示例配置了 CPU 90%/内存 85%/磁盘 85% 规则(连续 3 次触发),`OnNotify` 打日志演示;
生产替换为 `alert.NewWeComWebhook/NewDingTalkWebhook/NewFeishuWebhook` 真实机器人地址,
详见 [ALERT_GUIDELINES.md](../../docs/ALERT_GUIDELINES.md)。

## 修改指南

- 端口:main.go 中 `EnableWeb(appbox.TimeFormat, ":9528", ...)` / `EnableAdmin()`
- JWT 密钥:替换 `apptoken.SetSecretKey` 为密钥管理注入
- 数据库:SQLite → PostgreSQL(`datasource.DriverPostgreSQL` + DSN)
- 开放平台 App:修改 `openapi.NewRegistryWith(...)`;生产从数据库/配置中心加载
- 告警渠道:配置真实机器人 webhook 地址
