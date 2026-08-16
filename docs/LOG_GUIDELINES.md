# 日志使用规范(LOGGING GUIDELINES)

gbx 统一日志体系:业务代码、Iris(Web)、GORM(SQL)、标准库 log 全部经 zap 输出,
格式统一、按级别分流文件、字段结构化。

## 一、统一格式

**console(控制台 + debug/info/warn 文件)**:

```
2026-08-16T19:11:15.555+08:00	INFO	go-blackbox/application.starter.go:394	starting WebService...	{"service": "go-blackbox"}
2026-08-16T19:11:15.555+08:00	DEBUG	log/gorm_adapter.go:104	SQL	{"component": "gorm", "elapsed_ms": 6.7, "rows": 1, "sql": "SELECT ... WHERE id = ?"}
```

时间(毫秒级带时区)· 级别 · caller(文件:行) · message · 结构化字段(JSON 风格)。

**error.log(JSON 结构化,单行一条)**:

```json
{"level":"error","timestamp":"...","caller":"main.go:14","function":"main.main","message":"database connect failed","component":"database","stacktrace":"..."}
```

## 二、级别分流与策略

| 来源 | Debug(开发) | Info(生产默认) | Warn | Error |
|---|---|---|---|---|
| 业务日志 | 调试详情、入参出参 | 关键业务节点 | 可恢复异常 | 需人工介入的失败 |
| Iris(web) | 请求详情 | 启动/路由表/监听 | 限流拒绝等 | panic、500 |
| GORM | SQL + 参数(参数化 ?) + 行数 + 耗时 | — | 慢查询(>200ms) | SQL 错误 |
| 标准库 log | 透传 | 透传 | 透传 | 透传 |

- 开发:`builder.InitLog(dir, "debug")` — 路由、SQL 全可见
- 生产:`builder.InitLog(dir, "info")` — 自动只剩关键节点 + 告警级,无需改代码
- GORM 日志级别随全局级别联动:debug→SQL 详情,info→慢查询+错误

## 三、业务代码用法(zap 函数)

```go
import zaplog "github.com/Connorig/go-blackbox/framework/log"

zaplog.WithComponent("live").Infow("stream published", "app", "live", "stream", "test")
zaplog.WithComponent("live").Errorw("kick stream failed", "stream", "test", "error", err)
// WithComponent 附加 component 字段;或用 Sugar 函数:
zaplog.SugaredLogger.Debugw("debug detail", "key", value)
zaplog.SugaredLogger.Infow("business node", "key", value)
zaplog.SugaredLogger.Warnw("recoverable", "key", value)
zaplog.SugaredLogger.Errorw("failure", "key", value, "error", err)
```

规则:优先 Infow/Errorw 键值对风格(不要 fmt.Sprintf 拼 message);
error 一律放 `"error"` 字段;禁止打密码/Token/完整 DSN。

## 四、框架内自动接入(业务零配置)

| 来源 | 接入点 | 说明 |
|---|---|---|
| Iris | EnableWeb 自动 | GologHandler(结构化)+ GologWriter(路由表等 Printer 直写兜底),路由表/监听/请求异常全进 zap |
| GORM | EnableDatabase 自动 | SQL/慢查询/错误分流,ParameterizedQueries 默认开(值以 ? 隐藏) |
| 标准库 log | Init 自动 | 第三方依赖 log.Print* 桥接到 zap(info, component=stdlib) |
| 业务 | 手动调用 | 见第三节 |

自定义接入(罕见场景):
```go
app.Logger().Handle(zaplog.GologHandler("myiris"))   // 结构化拦截
app.Logger().SetOutput(zaplog.GologWriter("myiris")) // Printer 直写兜底
```

## 五、文件与轮转

- 目录:`<director>/zap/{debug,info,warn,error}.log`,严格分级(error.log 含 error 及以上)
- 轮转:默认 24h 轮转、保留 7 天(CONFIG.MaxAge / WithRotationTime 可调)
- 控制台:CONFIG.LogInConsole 控制;运行时 SetLevel("info") 可降噪
