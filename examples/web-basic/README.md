# web-basic 示例

展示 go-blackbox v1.4 核心能力的最小完整应用。

## 运行

```bash
# 仓库根目录
go run ./examples/web-basic
```

启动后:

| 端口 | 说明 |
| --- | --- |
| `:9528` | 业务 Web |
| `:6060` | Admin 管理服务(pprof/metrics/日志级别) |

## 验证路径

```bash
# 1. 健康探针
curl http://localhost:9528/health/ready

# 2. 登录获取 token(带 scope:user:read)
curl -X POST http://localhost:9528/api/v1/login

# 3. 携带 token 访问认证接口
curl -H "Authorization: Bearer <access_token>" http://localhost:9528/api/v1/me

# 4. Admin:Prometheus 指标
curl http://localhost:6060/metrics

# 5. Admin:pprof 诊断
curl http://localhost:6060/debug/pprof/goroutine?debug=1

# 6. Admin:运行时日志级别切换
curl -X POST -d '{"level":"debug"}' http://localhost:6060/cl

# 7. Admin:业务管理路由
curl http://localhost:6060/ops/demo
```

## 涉及能力

- 中间件体系:`ErrorHandler / RequestID / AccessLog / SecurityHeaders / CORS`
- JWT 认证 + scope 权限:`webiris.Auth(Whitelist, Scope)`,`GenTokenWithScope`
- 健康探针:`RegisterHealth`(`/health/live`、`/health/ready`)
- 统一响应:`OK / Fail / RespondError` + `apperr` 错误码
- Admin 服务:`EnableAdmin / EnableAdminRoutes`(pprof + metrics + POST /cl)
- 数据库:SQLite 内置驱动 + `datasource.Get/WithTx` + `Migrator` 版本化迁移
- 优雅关闭:退出信号 → 逆序释放资源

## 修改指南

- 路由/业务逻辑:`main.go` 中 `EnableWeb` 回调
- 认证要求:改 `AuthConfig.Scope` 与 `Whitelist`
- 数据库:改 `EnableDatabase` 的 Driver/DSN(生产换 PostgreSQL:`DriverPostgreSQL`)
- Admin 配置:`EnableAdmin(":7070")` 自定义监听
- 密钥:替换 `apptoken.SetSecretKey` 的示例密钥(生产用配置注入)
