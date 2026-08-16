# AGENTS.md - go-blackbox 项目开发指南(Agent 必读)

> 本文件是 go-blackbox 仓库的 Agent 操作手册。任何 Agent 在本仓库开发前必须完整阅读,
> 开发过程中**必须严格遵守**「五、泰山版开发规范(强制)」,代码不符合规范视为缺陷。

## 一、项目核心介绍

go-blackbox 是企业级 Go Web 应用脚手架(依赖库),对标 Spring Boot 的开箱即用体验:
`gbx new` 一键生成项目 → 安全基线/认证/数据权限/监控/开放平台开箱即得 → Web 化低代码生成平台。

| 项 | 值 |
|---|---|
| Module | `github.com/Connorig/go-blackbox` |
| Go | 1.21+(最低兼容 1.21,可随工具链升级,兼容新版特性) |
| 根包 | `appbox`(ApplicationBuild 装配) |
| Web | Iris v12(12.2.0) |
| ORM | GORM + glebarez/sqlite(测试用) |
| IOC | gbxioc(Spring 式容器) |
| 配置 | Viper 分层(默认 < 全局 < 项目 < 环境变量) |
| 发布 | 每功能点:commit → tag vX.Y.Z → push → Action 自动验证 + Release |

## 二、目录结构与模块地图

```
component/    通用能力(与业务无关,零框架依赖)
├── auth/      JWT(scope/组织身份/密钥轮换)+ RSA
├── error/     阿里手册 A/B/C 错误码(Code 字符串码 + HTTP 自动映射)
├── security/  SQL 注入检测(17 模式)+ 日志注入防护
├── util/      工具集(CopyProperties/DeepCopy/MD5/UUID/时间...)
└── mask/      脱敏器

framework/    框架能力(业务直接使用)
├── web/       webiris:中间件(AccessLog/RequestID/CORS/安全头/ErrorHandler/
│              Limit/BodyLimit/Timeout/SQLGuard)/Auth/统一响应/Admin/健康探针
├── database/  多实例数据源 + WithTx 事务 + 迁移 + DataScope 数据权限 + 雪花/UUID + 公共 Model
├── gbxioc/    依赖注入容器(Register/GetBean/单例/多例/生命周期)
├── config/    分层配置加载(SetGlobalConfigFile/SetConfigFileSearcher/热更新)
├── openapi/   开放 API 网关(AppKey 签名/防重放/限流/审计)
├── thirdparty/ 出站签名客户端(HMAC/RSA/Bearer + 重试 + 熔断)
├── circuit/   熔断器(closed/open/half-open)
├── aop/       方法级切面(Aspect 接口 + JoinPoint + Proxy;Before/After/Around)
├── monitor/   服务器资源监控(跨平台采集 + 内置页面)
├── alert/     监控告警(规则引擎 + 企微/钉钉/飞书 webhook)
├── cache/     RedisTemplate(Incr/Hash/List/锁/防击穿)
├── mongo/     MongoTemplate(Find/InsertMany/Count + 原生暴露)
├── mq/        RabbitMQ 状态机(自动重连/Consumer/Producer)
├── kafka/     KafkaTemplate(Producer/Consumer)
├── es/        EsTemplate(Index/Search/原生暴露)
├── storage/   StorageTemplate(MinIO/OSS S3 协议)
├── influx/    InfluxTemplate(时序写入/Flux 查询)
├── sms/       阿里云短信(零依赖自实现签名)
├── trace/     OpenTelemetry(OTLP 导出/Span 封装)
├── push/      SSE 实时推送 / WebSocket Hub
├── event/     事件总线
├── cron/      定时任务(单例防重入)
├── mail/      邮件
├── gencode/   Web 化低代码生成平台(RuoYi 风格)
├── audit/     审计
└── lifecycle/ 生命周期/优雅关闭

cmd/gbx      代码生成 CLI(code/config/gen 三种风格)
examples/     web-basic(全家桶)/ openapi / mongodb-demo
docs/         全部使用文档(见 docs/MODULES_GUIDE.md 索引)
```

## 三、核心设计约定(新代码必须遵守)

1. **Template 模式**:中间件集成 = `framework/<x>` 包 + `<Xxx>Template` 接口(常用操作)+ 原生实例暴露(`Client()/Channel()/DB()`)
2. **模块开关**:组件进 `config.Modules`(`enabled` 开关,零值关闭);builder `Enable*` 显式调用优先
3. **统一响应**:`webiris.OK/Fail` + 手册错误码(apperr.Code*);业务错误用 `apperr.New/Wrap`
4. **事务**:写操作用 `datasource.WithTx(ctx, fn)`(自动 Begin/Commit/Rollback),禁止手写样板
5. **数据权限**:查询带 `webiris.DataScope(ctx).Condition()` 组织隔离
6. **生命周期**:组件实例注册 gbxioc + 关闭钩子接入 `builder.OnShutdown`
7. **测试**:真实服务用环境变量启用(`GO_BLACKBOX_*`),未配置自动 `t.Skip`,不得阻塞 CI
8. **零依赖优先**:能用标准库/已有依赖解决就不加新依赖(参考 sms 自实现签名)

## 四、开发工作流(每个功能点)

```bash
# 1. 开发 → 全量验证(必须全绿)
go build ./...
go test ./...        # 全绿
go vet ./...         # 零告警

# 2. 提交(详细 commit message:功能说明 + 模块清单 + 验证结果)
git add -A
git commit -m "feat: ..."

# 3. 发布(每功能点一个 tag)
git tag vX.Y.Z
git push origin main
git push origin vX.Y.Z     # 触发 GitHub Action 验证 + 自动 Release

# 4. 真实验证(涉及模板/生成器改动时)
#    生成项目 → 编译 → 冒烟运行(参考各 GUIDELINES 的验证章节)
```

⚠️ 网络注意:本机 git 全局配置 github.com 走 `127.0.0.1:7890` 代理;
代理未运行时 push 会失败,先确认代理或临时用 `-c http.https://github.com.proxy=` 绕过。

## 五、泰山版开发规范(强制,代码审查项)

**以下规则来自阿里巴巴《Java 开发手册(泰山版)》,所有代码必须遵守。完整版见 `docs/DEVELOPMENT_STANDARDS.md`。**

### 5.1 命名(违反=缺陷)
- 包名:小写,简短(单数),禁止下划线(如 `framework/mongo` 包名 `mongodb` 例外需说明)
- 类型/接口:大驼峰;方法:小驼峰;常量:全大写+下划线
- 表名/列名:小写下划线;索引 `idx_` 前缀、唯一 `uk_` 前缀
- 禁止拼音命名、禁止无意义缩写(除领域公认)

### 5.2 错误码(手册附 3,A/B/C 三级)
- A 系列=用户端、B 系列=系统端、C 系列=第三方
- 业务错误必须用 `apperr.New(Code*, msg)` 携带手册码,禁止裸 `errors.New` 上抛
- HTTP 状态由 `apperr.HTTPStatus(code)` 自动映射

### 5.3 数据库(手册五)
- 字段:`id` / `gmt_create` / `gmt_modified` / `is_deleted`(StandardModel 已固化)
- 业务模型必须嵌入 `model.StandardModel`(或 SnowflakeModel/StringIDModel)
- 查询禁止 `SELECT *`;禁止在循环中查库;大批量用分页

### 5.4 API(手册五 API 部分 + docs/API_GUIDELINES.md)
- 统一响应 `{code, message, data}`;URL 小写连字符;分页 page/page_size 上限 100

### 5.5 日志与安全(手册二/四)
- 日志用 `zaplog` 结构化字段,禁止字符串拼接
- 密码/密钥/连接串禁止打印;SQL 注入用参数化(GORM 预编译)
- 敏感接口挂认证 + 限流

### 5.6 并发(手册一(七))
- 单例必须线程安全(gbxioc 已保证);共享可变状态加锁
- goroutine 必须有退出机制(ctx 取消),禁止泄漏

### 5.7 单元测试(手册三)
- 测试命名 `TestXxx`;断言明确;测试隔离不共享状态

## 六、常用命令速查

| 命令 | 用途 |
|---|---|
| `go run ./cmd/gbx new -name demo -style gen` | 生成 CRUD 全栈项目 |
| `go test ./framework/gencode/ -v` | 单包测试 |
| `go test ./... -run '^$'` | 全包编译(CI 第一步) |
| `go test ./framework/lifecycle . && go test ./framework/web -run 'TestNew\|TestInit\|TestRunRejects\|TestStaticSource'` | CI 隔离生命周期测试 |
| `git tag -f vX.Y.Z HEAD && git push origin :refs/tags/vX.Y.Z && git push origin vX.Y.Z` | 修正 tag 后重推(注意:tag 指向的 commit 里的 workflow 才是 Action 用的) |

## 七、文档要求

- 每个功能点必须更新 README.md 模块表 + docs 对应指南
- 文档要写清:如何配置、如何调用、示例代码、验证方式
- 新模块必须补 `docs/<模块>_GUIDELINES.md` 并登记到 `docs/MODULES_GUIDE.md` 索引
