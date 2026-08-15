# 配置体系指南(CONFIGURATION_GUIDELINES)

go-blackbox 配置体系对标 Spring Boot Auto-Configuration + Spring Cloud Config:
**配置驱动模块启停(开箱即用)+ 四层配置覆盖(父子层级)**。

## 一、分层配置(对标 Spring Cloud Config 父子覆盖)

优先级从低到高,高层覆盖低层,**键级合并**(子配置只声明要覆盖的键,其余继承父级):

| 层 | 来源 | 说明 |
|---|---|---|
| ① 内置默认 | 脚手架代码 | 安全默认值(端口/级别/池大小等) |
| ② 全局配置(父级) | `gbx.toml`(`SetGlobalConfigFile`) | 如 `/etc/gbx/config.toml`,多项目共享,可选 |
| ③ 项目配置(子级) | `config.toml`(`SetConfigFileSearcher`) | 业务项目主配置,覆盖全局同名键 |
| ④ 环境变量 | `GBX_*` 前缀(`EnableEnvSearcher`) | 最高优先级,`GBX_MODULES_WEB_PORT=:9090` |

```go
builder.LoadConfig(&cfg, func(l apploader.Loader) {
    l.SetGlobalConfigFile("gbx", "/etc/gbx") // 父级(可选)
    l.SetConfigFileSearcher("config", ".")   // 子级
    l.EnableEnvSearcher("GBX")               // 环境变量
})
```

特性:项目配置支持热更新(`Watch`);全局文件缺失不阻塞;错误聚合返回(不用半有效配置启动)。

## 二、模块自动配置(对标 @ConditionalOnProperty)

业务配置结构内嵌 `apploader.Modules`,在 config.toml 的 `[modules]` 段用 `enabled` 开关:

```toml
[modules]
[modules.log]
enabled = true
level = "info"

[modules.auth]
enabled = true
access_ttl = "30m"

[modules.web]
enabled = true
port = ":8080"
baseline = true          # 安全基线(限流/请求体上限/超时/SQL 注入拦截/日志/安全头)

[modules.database]
enabled = true
driver = "sqlite"
dsn = "./app.db"

[modules.cache]
enabled = false          # Redis 未启用不初始化
[modules.mongo]
enabled = false
[modules.cron]
enabled = false

[modules.admin]
enabled = true
listen = ":6060"

[modules.monitor]
enabled = true
path = "/monitor"

[modules.openapi]
enabled = false
[modules.openapi.apps]
# partner-001 = { secret = "...", algorithm = "HMAC-SHA256", enabled = true }
```

```go
type AppConfig struct {
    apploader.Modules `mapstructure:"modules"` // 脚手架模块配置
    Business          BusinessConfig           // 业务自有配置
}
var cfg AppConfig
builder.LoadConfig(&cfg, ...)
builder.AutoConfigure(&cfg.Modules, func(app *iris.Application) {
    // 业务路由(安全基线已自动挂载)
})
```

**语义:**
- `enabled = false` 的模块不初始化、不占资源(热插拔)
- 代码级 `builder.Enable*` 仍可用,显式调用优先,两者互补
- Web 自动装配时自动挂载:安全基线中间件 + 健康探针 + 监控/开放平台(按模块开关)+ 业务路由(最后挂载)
- 未启用 Web 时,monitor/openapi 等路由级模块不生效

## 三、模块清单与默认值

| 模块 | 配置段 | 关键字段 | 默认 |
|---|---|---|---|
| 日志 | `[modules.log]` | level/out_dir | info / "." |
| 认证 | `[modules.auth]` | access_ttl/refresh_ttl/issuer | 30m / 168h |
| Web | `[modules.web]` | port/baseline/rate_per_second/burst/max_body_bytes/timeout | :8080 / true / 100 / 200 / 1MB / 10s |
| 数据库 | `[modules.database]` | driver/dsn/host/port/user/password/auto_migrate | — |
| 缓存 | `[modules.cache]` | addr/password/db/pool_size | — |
| MongoDB | `[modules.mongo]` | addr/db/timeout | — |
| Cron | `[modules.cron]` | — | — |
| Admin | `[modules.admin]` | listen | :6060 |
| 监控 | `[modules.monitor]` | path | /monitor |
| 开放平台 | `[modules.openapi]` | apps(AppKey→secret/algorithm/enabled) | — |

## 四、完整示例

见 `gbx new` 生成的项目(`config.toml` 全注释模板 + `main.go` 配置驱动装配)。
