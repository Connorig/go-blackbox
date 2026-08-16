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
