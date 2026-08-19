# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

发版约定:每次发版必须在顶部新增 `## vX.Y.Z` 条目,CI 会提取当前 tag 对应条目
作为 GitHub Release 说明(banner 版本常量与 CHANGELOG 条目必须同步更新)。

## v1.46.0 - 2026-08-16

### Fixed
- database:字段配置密码含空格/# 时自动单引号引用,修复 pgx key=value DSN 解析截断风险
- database:DSN 直连预解析校验(语法错误/空格截断密码提前拦截,报错不泄露密码)
- database:字段配置密码含单引号时明确拒绝并提示改用 URL 格式 DSN
- live:ListStreams 兼容 SRS 5.x 顶层 video/audio 嵌套结构(平铺字段同步保留,旧版 publish 结构兼容)
- live:SRS 回调拒绝 msg 只带业务 message,不再透出 (code=...) 内部错误码后缀

## v1.47.0 - 2026-08-16

### Fixed
- database:DSN 直连自动规范化——解析后用 pgx ConnConfig 重建安全 URL 格式 DSN
  (url.UserPassword 编码,#/$/空格/引号等特殊字符密码彻底免疫 key=value 解析歧义;
   RuntimeParams 全量转 query;Unix socket 主机保持原样)
- database:残留场景实测闭环——live-integration 真库(15 字符 #$ 密码)DSN 直连端到端验证通过

## v1.48.0 - 2026-08-16

### Added
- 统一日志体系:业务 zap、Iris、GORM、标准库 log 四大日志源全部收编进 zap
- GologHandler:golog(iris)结构化日志拦截(Fields/调用点/级别映射)
- GologWriter:Printer 直写兜底(iris 路由表有意绕过 handler,SetOutput 拦截,剥离 ANSI/级别前缀/冗余时间戳)
- GormLogger:SQL 详情(参数化 ?/rows/elapsed_ms)进 debug,慢查询(默认 200ms)进 warn,错误进 error;级别随全局配置联动(debug=Info/info=Warn)
- stdlib 桥接:第三方依赖 log.Print* 收编进 zap
- EnableWeb/EnableDatabase 自动接线,业务零配置

### Changed
- console 下移除 function 全限定路径(短 caller 足够定位);error.log JSON 保留全路径

### Docs
- docs/LOG_GUIDELINES.md 日志使用规范(级别策略/用法/接入说明)

## v1.49.0 - 2026-08-16

### Fixed
- apidoc:文档页 JS 方法名大小写匹配——OpenAPI PathItem key 为小写(get/post),
  JS 改为 item[key] 直接取值(原先用大写取值导致接口列表永远为空)
- apidoc:版本号显示去重 v 前缀(version 已含 v 时不再拼 v,修复 vv1 瑕疵)
- apidoc:/docs 无尾斜杠下 fetch 相对路径 404(Bug 4 已在 v1.48.0 合入 prefix 注入,
  本次补回归测试锁死)

## v1.50.0 - 2026-08-16

### Added
- live:命名客户端实例支持(设计建议落地,对齐 datasource NewNamed 模式)
  - SetNamed(name, client) 注册命名实例(nil 注销,幂等)
  - GetNamed(name) 按名获取;Get() 默认实例(旧语义保留,未初始化返回 nil)
  - Clients() 全部实例快照;读写锁并发安全
  - 多 SRS 集群/多 vhost 场景:按集群注册、按名调用

## v1.51.0 - 2026-08-16

### Fixed
- log:webiris 监听日志用真实地址自打(listener.Addr()),替换 iris 残缺输出
  Now listening on: http://[(空 host 时 iris 拼接 bug)
- log:GologWriter 丢弃 iris 的 Now listening on 行(真实地址由 webiris 输出)
- log:iris 路由表中间件行压缩——函数全限定名与文件全路径短名化
  (• web.ErrorHandler (error_handler.go:15)),debug 下路由表更紧凑

## v1.51.1 - 2026-08-16

### Fixed
- log:中间件行短名化仅限 gbx 框架与第三方依赖路径;业务项目代码保留全限定路径
  (报错定位依赖包/类/方法信息,不丢上下文)

## v1.52.0 - 2026-08-16

### Added
- push/ws 房间化(直播 agent 提案落地):Join/Leave/BroadcastRoom/CountRoom/Rooms
- 房间多对多归属(一连接多房间)、Join/Leave 幂等、断开自动清理 + OnLeave 触发
- OnJoin/OnLeave 生命周期回调(锁外触发,回调内可安全调用房间 API)
- Client.Meta/SetMeta/MetaValue 业务属性绑定(用户 ID/昵称等)
- 全局 Broadcast 语义不变,完全向后兼容

### Docs
- docs/WS_ROOM_GUIDELINES.md 房间化使用指南(场景/API/弹幕示例/设计语义/验收要点)

## v1.53.0 - 2026-08-17

### Added
- framework/excel:基于模板的泛型 Excel 导入导出(模板文件业务预调样式/格式,gbx 只填充数据)
  - Export/ExportToFile:泛型列表按模板导出(表头映射/类型转换/样式保留)
  - Import/ImportFromFile:Excel 解析为泛型列表(excel tag/字段名映射/多类型解析)
  - 兼容 string/int/float/bool/指针类型;空单元格零值;未匹配列忽略
- gbx CLI 模板:influx 配置段(code/config/gen 三风格 config.toml 模板)

### Docs
- docs/EXCEL_GUIDELINES.md Excel 模板导入导出指南(设计约定/导出/导入/模板示例)

## v1.54.0 - 2026-08-17

### Added
- excel:多 sheet 导出(ExportMultiSheet,每 sheet 独立模板/数据,自动创建新 sheet 并写表头)
- excel:分批写入(ExportBatch,万行以上大幅降低内存峰值,OnProgress 进度回调)
- excel:流式导出(ExportBatchToWriter,HTTP 下载场景不超时)
- excel:导入增强(ImportWithErrors:跳过空行;返回行级错误列表;HeaderRow/DataStartRow 独立配置)

## v1.55.0 - 2026-08-17

### Added
- excel:导出文件名构造(ExportName)——业务名可指定(中文安全)+ 时间戳后缀(yyyyMMdd_HHmmss)
  防多次导出覆盖;支持自定义后缀;自动去除输入扩展名
- excel:HTTP 下载文件名编码(ContentDisposition)——RFC 5987 percent-encode 中文文件名
  + ASCII fallback,兼容各浏览器

## v1.56.0 - 2026-08-18

### Added
- framework/idempotent:Redis 业务幂等防护(Check/Release/Status)——
  首次执行占用标记,重复请求(回调重试/连点/消息重投)直接拒绝;
  与 cache.Lock 互斥语义互补(锁=执行期互斥,幂等=结果性占用)
- cache.RawClient:暴露底层 go-redis 客户端(高级场景入口)
- push/ws 跨节点广播:WithRedis 配置后房间/全局消息经 Redis pub/sub 路由多实例
  (消息带 node_id 防双发;订阅自动重连;单实例行为不变)

## v1.57.0 - 2026-08-18

### Added
- framework/sensitive:敏感词过滤(自研 DFA,O(n) 单遍匹配,零外部依赖)
  - Contains/Find/Replace/Validate;最长匹配(前缀嵌套);空格绕过防护;动态词条
  - 场景:弹幕风控/昵称审核/评论过滤
- framework/validator:统一参数校验(对标 Spring Validation,go-playground v10)
  - Struct/StructAll/Var;中文错误信息(label tag 优先);A0400 参数错误码集成
  - 自定义规则注册(RegisterCustom);泰山手册 A 级错误码要求落地

## v1.58.0 - 2026-08-18

### Added
- framework/counter:高并发在线计数(直播人数/峰值/时段统计)
  - Counter:Inc/Dec(下限保护)/Value/Peak(历史峰值)/ResetPeak/Snapshot
  - Registry:按房间(键)计数注册表(直播间人数场景)
- framework/ratelimit:Redis 令牌桶分布式限流
  - Allow/AllowN:多实例共享配额,原子 Lua 脚本
  - Reset 解封;key 前缀;接口限流/短信频率/防刷场景

## v1.59.0 - 2026-08-18

### Added
- framework/upload:multipart 文件上传封装(大小/扩展名校验 + 内容读取)
  - iris 与 net/http 双入口;直播封面/头像/附件场景
- framework/captcha:图形验证码(生成 PNG + 答案存储校验)
  - 默认内存存储(带 TTL 与限次);可注入自定义 Store(Redis 分布式)
  - 大小写不敏感;验证失败即消费(防暴力破解)

## v1.60.0 - 2026-08-18

### Added
- database:连接池统计(PoolStats/PoolUtilization)——监控告警场景
  (InUse 接近 MaxOpen、WaitCount 增长即池偏小)
- docs/MODULES_GUIDE.md:新增模块索引(v1.56+ 幂等/敏感词/校验/计数/限流/上传/验证码/Excel/WS 跨节点)

## v1.61.0 - 2026-08-18

### Added
- excel:ImportValidated——业务校验回调(ValidateFunc),失败行收集 RowError(行号+原因),
  部分成功语义;无校验函数时行为同 ImportWithErrors
- excel:ImportStream——流式导入(超大文件几十万行),逐行回调不一次性加载,
  回调错误中断导入;handler 必填校验

## v1.62.0 - 2026-08-18

### Fixed
- config:环境变量覆盖排障(Bug 7)——实测 v1.52.0+ 当前代码 LIVE_LIVE_WEBADDR
  覆盖生效(:9998 验证);gbx config 模块 v1.50→v1.52 零变更,疑为测试环境因素
- config:新增环境变量覆盖回归测试(嵌套键覆盖/文件回退)锁死行为,防回退

## v1.63.0 - 2026-08-19

### Added
- framework/taskqueue:进程内异步任务队列(延迟执行/并发控制/panic 捕获/优雅退出)
- live:回调签名验签(HMAC-SHA256 中间件,防伪造 SRS 回调)+ DVR 录制客户端(StartRecord/StopRecord)
- framework/grayscale:灰度路由(按比例/按用户稳定分流,发版灰度/A-B 测试)
- framework/configcenter:Nacos 风格配置中心客户端(Fetch 拉取 + Watch 轮询监听)

## v1.45.0 - 2026-08-16

### Added
- CHANGELOG.md 版本更新日志:每个版本的功能/修复/变更一目了然
- Release 说明自动化:CI 从 CHANGELOG 提取当前 tag 条目作为 GitHub Release body
  (之前 Release 为空,仅有自动生成的 PR 摘要)
- CI 新增 changelog 条目校验:缺少条目则 Release 失败,杜绝无日志发版

## v1.44.0 - 2026-08-16

### Changed
- zap 日志优化:error.log 改用结构化 JSON 编码(单行一条记录),level/timestamp/
  message/caller/function/service/component/业务字段/stacktrace 全字段完整,
  堆栈与调用链不再散乱,便于日志平台解析与告警匹配
- debug/info/warn 保持 console 人读格式(时间/级别/调用点/函数/消息/字段)

## v1.43.0 - 2026-08-16

### Added
- 版本同步强制校验:banner.go Version 常量必须与 tag 一致
  - scripts/check-version.ps1:发版前本地校验(-ExpectedVersion 指定目标版本)
  - CI 新增 Verify banner version matches tag 步骤,不一致则 Release 失败

## v1.42.1 - 2026-08-15

### Fixed
- PostgreSQL DSN 构建器抽取,修复密码未注入导致 SASL 认证失败的问题
- 回归测试覆盖 DSN 密码拼接

## v1.42.0 - 2026-08-15

### Added
- live 直播模块:SRS 流媒体回调(on_publish/on_play/on_connect/on_unpublish/
  on_dvr/on_hls)+ SRS API 客户端(KickStream 等)

## v1.41.0 - 2026-08-15

### Added
- FillTree:实体模式树形构建(无返回值直接填充子节点)
- SetAppInfo:业务项目打印自身版本号与名称

## v1.40.0 - 2026-08-14

### Added
- util 集合工具:数组/map 公用处理方法
- 树形结构泛型封装:集合传入、N 级树形返回(评论回复/国家地区/角色菜单等场景)

## v1.39.0 - 2026-08-14

### Added
- MQTT 中间件模块(发布/订阅)
- HTTPClient 通用 HTTP 客户端模块

## v1.38.0 - 2026-08-14

### Fixed
- 启动横幅:清晰的 GBX.APP 字样(替换损坏的 ASCII art)

## v1.37.0 - 2026-08-14

### Added
- appbox 资源门面:业务统一通过 appbox.XXX() 获取基础设施组件

## v1.36.0 - 2026-08-13

### Added
- 模块级便捷获取函数:getCache/getMongodb/getKafka 等(避免每次 GetBean)

## v1.35.0 - 2026-08-13

### Changed
- gbxioc 移至 framework 包下,作为核心组件(Spring IOC 设计理念全覆盖)

## v1.34.0 - 2026-08-13

### Added
- util.Time 类型:格式化/解析与 CopyPropertiesNonBlank 联动(嵌套 time 字段可复制)

## v1.33.0 - 2026-08-13

### Added
- 吸收 sg-mes-api 实用工具:RSA、bean copy、pager 等

## v1.32.0 - 2026-08-12

### Added
- datasource.BuildCondition 条件构建
- sg-mes 风格 Party 路由模板

## v1.31.0 - 2026-08-12

### Added
- 路由分组与 Model 注册函数:业务自定义函数返回路由数组/model 列表,main 保持简洁

## v1.30.1 - 2026-08-12

### Changed
- gbx 模板依赖升级至 v1.30.0

## v1.30.0 - 2026-08-12

### Added
- apidoc:API 文档自动生成(CRUD/页面/Schema 反射自动生成,声明简洁)

## v1.29.0 - 2026-08-12

### Changed
- Go 版本策略更新,支持 toolchain 升级(基线 1.21)

## v1.28.0 - 2026-08-11

### Added
- agent.readme.md、模块使用指南与文档索引(AGENTS.md/MODULES_GUIDE.md)

## v1.27.0 - 2026-08-11

### Added
- Web 低代码生成平台

## v1.26.0 - 2026-08-11

### Added
- gbx gen 风格:可运行的 CRUD 全栈生成器

## v1.25.0 - 2026-08-10

### Added
- Kafka、OpenTelemetry 链路追踪、阿里云 SMS 集成

## v1.24.0 - 2026-08-10

### Added
- InfluxDB 时序数据库模板

## v1.23.0 - 2026-08-10

### Added
- ElasticSearch 与对象存储(MinIO/OSS)

## v1.22.0 - 2026-08-09

### Added
- 中间件模板(缓存/Mongo 等)

## v1.21.0 - 2026-08-09

### Changed
- simpleioc 更名为 gbxioc,gbx 模板支持 code/config 双风格

## v1.20.0 - 2026-08-09

### Added
- 配置驱动的自动装配 + 分层配置合并

## v1.19.0 - 2026-08-08

### Added
- 接口驱动 AOP:Aspect 与 JoinPoint 上下文

## v1.18.0 - 2026-08-08

### Added
- 方法级 AOP 框架与企业级文档

## v1.17.0 - 2026-08-08

### Changed
- 移除冒烟测试 SQLite 产物

## v1.16.0 - 2026-08-07

### Added
- 资源监控告警 + Webhook 通知器

## v1.15.0 - 2026-08-07

### Added
- 第三方调用熔断器

## v1.14.0 - 2026-08-06

### Added
- gbx CLI 项目生成器(冒烟验证骨架)

## v1.13.0 - 2026-08-06

### Added
- 服务器资源监控 + 内置仪表盘

## v1.12.0 - 2026-08-05

### Added
- 安全防护(security guards)

## v1.11.0 - 2026-08-05

### Added
- JWT 组织/部门身份的数据权限隔离

## v1.10.0 - 2026-08-04

### Fixed
- v1.8.0 分层重构后的发版工作流路径

## v1.9.0 - 2026-08-04

### Added
- 阿里巴巴泰山版开发手册引入

## v1.8.0 - 2026-08-03

### Changed
- 项目分层布局重构(cmd/internal/pkg 等 Go 标准布局)

## v1.7.0 - 2026-08-02

### Added
- WebSocket 消息中心:广播/消息回调/心跳

## v1.6.0 - 2026-08-01

### Added
- SSE 推送管理器 + eventbus 桥接

## v1.5.0 - 2026-07-31

### Added
- 管理员服务、脱敏器、认证作用域与 web-basic 示例

## v1.4.0 - 2026-07-30

### Added
- 多数据源实例(生命周期管理 + SQLite 支持)

## v1.64.0 - 2026-08-19
## v1.65.0 - 2026-08-19
## v1.66.0 - 2026-08-19
## v1.67.0 - 2026-08-19
## v1.68.0 - 2026-08-19
## v1.69.0 - 2026-08-19
## v1.70.0 - 2026-08-19

### Added
- framework/audit QueryHandler:审计日志查询 HTTP 接口(offset/count 分页,统一响应 list+total),与 RedisListSink 组合,挂载/鉴权由业务决定
- README 模块表登记:RBAC/可靠任务队列/事件桥接/通知中心/审计查询

## v1.71.0 - 2026-08-19
## v1.72.0 - 2026-08-19
## v1.73.0 - 2026-08-19
## v1.74.0 - 2026-08-19
## v1.75.0 - 2026-08-19
## v1.76.0 - 2026-08-19
## v1.77.0 - 2026-08-19
## v1.78.0 - 2026-08-19

### Added
- component/i18n 国际化组件(对标 Java ResourceBundle + Spring MessageSource):
  - Bundle:Register 注册 / LoadDir 按目录加载语言文件(langs/zh-CN.json、en-US.json)
  - T 翻译(缺失回退默认语言,再缺失返回 key);{{key}} 占位符,支持嵌套键
  - Tf fmt 风格格式化;DetectLanguage 解析 Accept-Language(大小写归一)
  - SetFallback 自定义回退语言;协程安全

### Tests
- 注册翻译与回退/目录加载/目录错误/Accept-Language 解析/Tf/nil 与边界 6 组


### Added
- framework/notify 模板注册与渲染:
  - RegisterTemplate/Template:进程内模板注册中心(同名覆盖)
  - RenderTemplate/Render:{{key}} 占位符渲染,支持嵌套键 {{user.name}}(点分隔)
  - 缺失参数返回错误并列出缺失键(便于排查);未注册模板明确报错
  - 与适配器组合:模板内容 → content.Template/Params → Send,统一管理通知文案

### Tests
- 注册渲染(嵌套键/中文/数值)/缺失参数列键/未注册报错/直接渲染/边界与覆盖 6 组


### Added
- framework/grayscale 灰度可观测性:
  - Route 响应头标记命中版本(X-Gray-Version: new|old,WithHeaderName 可关闭)
  - Stats 命中统计(Total/NewHits/OldHits/实际占比,原子计数,可接监控)
  - 与日志埋点配合,灰度期对比新旧版本错误率更直观

### Tests
- 响应头标记与统计一致性(100 请求断言)/关闭标记/nil 与全量边界 3 组


### Docs
- REDQUEUE_GUIDELINES:可靠任务队列使用指南(快速使用、重试与死信、死信治理流程、监控、多实例说明、与进程内 taskqueue 对比)
- CONFIGCENTER_GUIDELINES 新增「本地缓存客户端(推荐)」章节(CachedClient 用法与 API 一览)
- README/MODULES_GUIDE 补文档链接

### Changed
- configcenter:Fetch 返回错误时 CachedClient.Get 未加载场景明确报错,业务使用默认值兜底


### Added
- framework/configcenter CachedClient(带本地缓存的配置中心客户端):
  - Get 首次拉取并缓存,之后直接返回缓存(配置中心不可用也能用最后成功值)
  - Refresh 强制更新(失败保留旧值);Start 后台轮询(启动即刷新)
  - Subscribe 变更订阅(首次立即收到当前值,Close 关闭通道)
  - 对标 Nacos 客户端体验:断网不丢配置,变更自动推送

### Tests
- 首次拉取与刷新/故障保留旧值/订阅广播/后台轮询/nil 安全 5 组(httptest Nacos 风格服务)


### Added
- framework/taskqueue/redqueue 死信回调:
  - WithDeadLetterHook:死信产生时回调(可接 alert/notify/日志),不阻塞消费
  - 回调携带完整 DeadLetter(payload/retries/failed_at);多实例部署时每实例都会收到,业务侧去重

### Tests
- 回调触发(载荷/重试次数/时间戳断言)与未设置回调行为不变 2 组(Redis env 门控)


### Docs
- GRAYSCALE_GUIDELINES:灰度路由使用指南(比例/用户稳定分流算法、发版流程、与配置中心联动、注意事项)
- CONFIGCENTER_GUIDELINES:配置中心使用指南(Fetch/Watch、灰度开关/业务参数热更新、Nacos 接入、注意事项)
- README 模块表登记灰度路由与配置中心

### Changed
- configcenter:Watch 间隔默认 30s;onChange 首次拉取即回调(初始化配置)


### Added
- framework/taskqueue/redqueue 重试上限与死信:
  - WithMaxRetries(默认 5,0 无限);handler 失败计数 +1 延迟 1s 重投,超限进死信队列
  - 存储信封格式 {data, retries};兼容早期裸 payload(自动识别)
  - DeadLetterCount/DeadLetters(倒序分页)/RequeueDeadLetter(重投并移除)
  - 损坏消息不会阻塞队列

### Tests
- 超限进死信(尝试次数断言)/死信查询与重投/裸 payload 兼容 3 组(Redis env 门控)


### Added
- framework/notify 渠道适配器:SMSAdapter(包装 framework/sms,模板覆盖/参数透传/上游拒绝转错误)
- MailAdapter(包装 framework/mail,标题正文映射)
- 与通知中心组合:Register 后业务只依赖 notify 统一入口,开箱即用
- 集成测试:SMS 真实发送 env 门控(GO_BLACKBOX_SMS_KEY/SECRET/PHONE)

### Tests
- 渠道标识/nil 安全/取消 Context 拒绝/注册进 Manager/SMS 集成 5 组


### Added
- framework/audit RedisListSink:审计日志落地 Redis List(LPUSH 倒序,limit 保留最近 N 条),多实例共享
- framework/audit Query/Count:最近审计日志倒序分页查询,损坏条目自动跳过
- Entry 全字段 json tag(snake_case),对接管理界面/导出
- docs/RBAC_GUIDELINES.md:RBAC 使用指南(Provider 实现/装配/缓存策略/数据权限配合/最佳实践)
- MODULES_GUIDE 登记:RBAC/可靠任务队列/事件桥接/通知中心/审计查询

### Tests
- RedisListSink 写入查询往返(倒序+分页)/limit 截断/损坏条目跳过/nil 安全 4 组(Redis env 门控)


### Added
- framework/notify 统一通知中心:多渠道发送抽象(短信/邮件/站内信等),Sender 接口插拔
- Register 渠道注册(重复拒绝)、Send 单渠道、SendAll 并发多渠道(错误聚合,成功渠道不受影响)
- Content 支持渠道模板 + 参数渲染与直发正文两种模式
- 与 framework/sms、framework/mail 组合:业务只依赖 notify 统一入口

### Tests
- 注册/重复拒绝/参数校验/发送成功/SendAll 错误聚合/默认全渠道/nil 安全 7 组全绿


### Added
- framework/event RedisBridge:Redis Pub/Sub 桥接进程内事件总线,多实例部署跨实例事件投递
- Start 订阅远端事件转发本地总线(单条损坏消息跳过);Publish 发布事件到 Redis 频道
- 场景:分布式缓存失效通知、跨实例业务联动、多实例广播
- 注意:跨实例收到的 data 类型为 map[string]interface{}(JSON 反序列化),负载必须可 JSON 序列化

### Tests
- 参数校验/序列化往返/跨实例投递(双客户端 Redis env 门控)3 组


### Added
- framework/taskqueue/redqueue:基于 Redis 的可靠任务队列(进程内 taskqueue 的持久化/多实例补充)
- 即时任务走 List,延迟任务走 ZSet(score=执行时间),Lua 原子搬移到期任务
- BRPop 阻塞消费,handler 失败自动延迟 1s 重新入队;多消费者并行不重复投递
- Pending 队列深度查询;Submit/Consume/Pending 全 nil 安全
- 场景:无 MQ 环境的轻量可靠延迟队列、多实例任务分发、重启不丢任务

### Tests
- 参数校验/即时往返/延迟到期执行/失败重投/多消费者并发 5 组(Redis env 门控)


### Added
- framework/rbac 角色权限判定层:Provider 抽象 + Enforcer(内存缓存 TTL 默认 60s)
- HasPermission/HasAnyPermission/HasRole/HasAnyRole 判定 API,权限点按角色自动合并
- RequirePermission/RequireRole 声明式中间件(403 A0312),与 Auth/scope 叠加使用
- ClearCache:角色变更后即时生效;WithTTL 可调缓存
- 定位:与 JWT scope 互补——scope 控 API 组访问,rbac 控业务操作粒度

### Tests
- 权限合并/任一权限/角色判定/缓存命中/声明式中间件/nil 安全/并发访问 8 组全绿

## v1.3.0 - 2026-07-29

### Changed
- 工程清理与文档

## v1.2.0 - 2026-07-28

### Changed
- 模块迁移与生命周期升级

### Fixed
- 发版工作流 vet 错误(v1.2.1)
- MongoDB demo 键序 BSON 元素(v1.2.2)

## v1.0.x - v1.1.x - 2026-07-27 及更早

- 早期开发阶段:基础框架搭建、Vue 前端工程引入,通过 PR 合并方式迭代
