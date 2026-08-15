# go-blackbox 开发规范

> 依据:阿里巴巴《Java 开发手册(泰山版)》(2020-04-22) + Go 社区惯例(Effective Go / Go 官方规范)。
> 目标:企业级 + 阿里开发规范 + 行业开放标准的 Go Web 脚手架。
> 每条规范给出:手册要点 → Go 落地方式 → 脚手架落地状态。

---

## 一、命名规约

### 1.1 基本规则(手册一(一) 1-2 条)

| 手册要求 | Go 落地 | 状态 |
| --- | --- | --- |
| 命名不以 `_`/`$` 开头结尾 | Go 以大写开头表示导出;包内命名同理,禁止 `_name` 式命名 | ✅ 已遵循 |
| 严禁拼音/英文混合、中文命名 | 全部英文语义命名;包名小写单数 | ✅ 已遵循 |

### 1.2 类(类型)命名(一(一) 3 条)

- 手册:类名 UpperCamelCase,DO/DTO/VO/BO 例外(全大写缩略)。
- Go 落地:导出类型 PascalCase(`UserService`、`RedisCache`);模型类型遵循 `xxxModel`/`xxx` 表名映射;不强制 DO/DTO 后缀,但**脚手架生成的业务模板使用明确后缀**:

| Go 类型 | 对应手册概念 | 说明 |
| --- | --- | --- |
| `User` / `Order`(model 包) | DO | 与表一一对应,`framework/database/model` |
| `CreateOrderRequest` / `OrderResponse` | DTO / VO | API 出入参,`api` 层定义 |
| `OrderQuery` | Query | 查询条件对象(超 2 参数禁止 Map 传输) |
| `OrderService` + `orderService` 实现 | Service + Impl | 接口与实现分离,依赖倒置 |

### 1.3 方法/变量命名(一(一) 4 条)

- 手册:lowerCamelCase;方法命名 getXxx/listXxx/countXxx/saveXxx/updateXxx/removeXxx。
- Go 落地:方法名 PascalCase(导出);**Service 层方法命名遵循手册前缀规约**:

```go
GetByID(id)        // 获取单个对象
ListByStatus(s)    // 获取多个对象,复数结尾
CountByStatus(s)   // 统计
Save(entity)       // 插入
Update(entity)     // 修改
Remove(id)         // 删除
```

### 1.4 常量(一(一) 5 条 + 一(二))

- 手册:常量全大写下划线;禁止魔法值;按功能归类,禁止大而全常量类;固定范围用 enum。
- Go 落地:常量全大写下划线(`DefaultShutdownTimeout` → `DEFAULT_SHUTDOWN_TIMEOUT`?Go 惯例导出常量也可 PascalCase,但按手册采用 `MAX_CONN_POOL_SIZE` 风格);**错误码、状态值用类型化常量/枚举**:

```go
// 状态枚举(手册:enum 携带延伸属性)
type OrderStatus string

const (
    OrderStatusCreated   OrderStatus = "created"
    OrderStatusPaid      OrderStatus = "paid"
    OrderStatusShipped   OrderStatus = "shipped"
)
```

- 魔法值禁令:脚手架所有超时、TTL、端口、队列容量等均为命名常量。
- 落地状态:框架常量已归类;业务模板将遵循。

### 1.5 接口与实现(一(一) 16 条)

- 手册:Service/DAO 必须接口 + `Impl` 实现类;接口方法不加修饰符。
- Go 落地:Go 惯例是**小而精的接口 + 隐式实现**;脚手架提供两种模式:
  1. 业务层:定义接口(`UserService`)+ 实现(`userService`,容器注册),依赖注入走接口(依赖倒置);
  2. 框架层:保持 Go 惯例(具体类型 + 必要接口,如 `Rediser`、`Receiver`)。

### 1.6 各层模型规约(一(一) 18 条 B)

- DO/DTO/VO/BO/Query 定义见 1.2 表;**禁止 Map 传输查询对象(超 2 参数)**。
- 布尔字段:**数据库字段 `is_xxx`,POJO 属性不加 is 前缀**(Go 为 `Enabled bool` + GORM 列 `is_enabled`)。

## 二、代码格式

| 手册要求 | Go 落地 | 状态 |
| --- | --- | --- |
| 4 空格缩进,禁 tab | `gofmt` 强制 tab 缩进(Go 惯例,`go fmt` 保证全仓库一致) | ✅ gofmt |
| 运算符/关键字两侧空格 | `gofmt` 自动处理 | ✅ |
| 单行 ≤120 字符 | Go 惯例 100-120;超长换行 | ✅ 代码评审要求 |
| 方法 ≤80 行 | 单一职责,过长拆方法 | ✅ AGENTS.md |
| UTF-8 + Unix 换行 | 仓库统一 LF(gitattributes) | ✅ |
| 空行分隔逻辑块 | 遵循 | ✅ |
| 注释 `// ` 后一空格 | GoDoc 规范 | ✅ |

## 三、OOP 规约 → Go 落地

| 手册条款 | Go 落地 |
| --- | --- |
| 覆写方法加 @Override | Go 无覆写概念;接口实现由编译器保证 |
| 外部接口不允许改签名,过时加 @Deprecated | Go:废弃 API 注释 `Deprecated:` + 提供新入口(脚手架已遵循:如 `GetDbInstance` 等兼容层) |
| POJO 属性用包装类型(null 表达额外语义) | Go:结构体字段用指针/`sql.Null*`/`*T` 表达可空;基础类型零值即默认值,需谨慎 |
| 金额用最小货币单位整型 | Go:`int64` 分;文档约束 + 模板示例 |
| 浮点不等值比较 | Go:同规则,用 `math.Abs` 误差或 `decimal` 库 |
| DO 属性类型与库字段匹配 | GORM tag 显式映射;`int64` ↔ `bigint` |
| 构造方法禁业务逻辑 | Go:无构造方法;`NewXxx` 工厂保持纯初始化,业务逻辑放 `Init` 式方法 |
| POJO 必须 toString | Go:日志打印用 `%+v`;敏感字段脱敏(component/mask) |
| 访问控制从严(private 优先) | Go:小写(包私有)优先,导出最小化(脚手架已遵循) |
| 类内方法顺序:公有→私有 | Go 惯例:导出方法在前,辅助方法在后(脚手架已遵循) |

## 四、异常与错误码(手册二(一) 错误码)

### 4.1 三级错误码体系(泰山版附 3)

| 码段 | 含义 | 示例 |
| --- | --- | --- |
| `A0001` | 用户端错误 | A0400 参数错误 / A0200 登录异常 / A0300 权限异常 |
| `B0001` | 系统端错误 | B0210 系统限流 / B0314 连接池耗尽 |
| `C0001` | 第三方服务错误 | C0300 数据库出错 / C0130 缓存出错 |

### 4.2 脚手架落地(`component/error`)

```go
// 错误码常量(按手册分级)
const (
    CodeUserError         apperr.Code = "A0001"
    CodeParamInvalid      apperr.Code = "A0400"
    CodeUnauthorized      apperr.Code = "A0301"
    CodeSystemError       apperr.Code = "B0001"
    CodeRateLimited       apperr.Code = "B0210"
    CodeThirdPartyError   apperr.Code = "C0001"
)

// 业务使用
return apperr.New(http.StatusBadRequest, CodeParamInvalid, "参数错误")
// 响应:{"code":"A0400","message":"参数错误",...}
```

- 已落地:`apperr.Error`(HTTP 状态 + 业务码 + 消息 + 原因链);`webiris.RespondError` 统一转响应。
- **待落地**:预置手册错误码常量表(component/error/codes.go),替换当前数字码。

### 4.3 分层异常处理(手册六(一) 2 条)

| 层 | 手册要求 | Go 落地 |
| --- | --- | --- |
| DAO 层 | 捕获并转 DAOException,不打日志 | GORM 错误上抛,`framework/database` 包装上下文 |
| Service 层 | 必须记录日志,保护案发现场 | 业务 Service 记录结构化日志(带参数上下文) |
| Web 层 | 不抛异常,转友好响应 | `webiris.ErrorHandler` + `RespondError` ✅ 已落地 |
| 开放接口层 | 错误转错误码+信息 | `apperr` + 统一响应 ✅ 已落地 |

## 五、日志规约(手册二(三))

| 手册要点 | 落地 |
| --- | --- |
| 禁止输出敏感信息(密码/token/连接串) | ✅ `framework/log` 脱敏约定 + `component/mask` + 配置 `Redact` |
| 日志包含关键上下文 | ✅ 结构化字段:timestamp/level/service/component/caller/function/message |
| 异常日志带堆栈 | ✅ `zap.AddStacktrace(ErrorLevel)` |
| 日志分级 | ✅ 严格单级文件(debug/info/warn/error) |
| 日志平台可检索 | ✅ JSON 格式 + 稳定字段名;request_id 链路 ✅ |
| 禁止在循环中打大量日志 | 代码评审要求 |
| 生产用 info 级以上 | ✅ 文档约定 |

## 六、数据库规约(手册五)

### 6.1 建表规约(泰山版核心,文档模板已固化)

| 规则 | 约定 | 脚手架落地 |
| --- | --- | --- |
| 表名 | 小写下划线,模块前缀 `t_`/业务前缀 | `framework/database/model` 文档 |
| 字段名 | 小写下划线,单词全小写 | GORM `column` tag 显式映射 |
| 主键 | `id bigint unsigned` 自增 | model.Model.ID ✅ |
| 创建/修改时间 | `gmt_create`/`gmt_modified`(禁 `create_time` 混用) | model.Model 提供 `CreatedAt/UpdatedAt`(GORM 自动维护) |
| 逻辑删除 | `is_deleted`(1 删除/0 未删除) | model.Model.DeletedAt(GORM 软删除)→ **字段名映射 `is_deleted` 待统一** |
| 唯一索引 | `uk_字段名` | GORM `uniqueIndex:uk_xxx` |
| 普通索引 | `idx_字段名` | GORM `index:idx_xxx` |
| 小数 | `decimal`,禁止 float/double | 文档约束 |
| 布尔 | `is_xxx`(1/0),POJO 属性不加 is | GORM `is_enabled` → `Enabled` |
| 金额 | 最小货币单位整数(`int64` 分) | 模板示例 |

### 6.2 SQL 规约 → GORM 落地

| 手册 | GORM 落地 |
| --- | --- |
| 禁 `count(列名)`,用 `count(*)` | `db.Model(&T{}).Count(&n)` ✅ |
| 分页 count 为 0 直接返回 | `datasource.Page` 先 Count 后 Find ✅ 已落地 |
| 禁外键/级联,应用层解决 | GORM 禁用 `ForeignKey` 自动约束(文档约定) |
| 禁存储过程 | 无 |
| 多表查询加表别名 | GORM 原生 SQL 文档约定 |
| `in` 集合 ≤1000 | 代码评审约定 |
| 查询禁 `*`,显式字段 | GORM `Select("id","name")` 约定 |
| 参数化防注入 | GORM 全部 `?` 参数化 ✅ |
| 更新必须带 `gmt_modified` | GORM `UpdatedAt` 自动维护 ✅ |
| 禁大而全更新接口 | GORM `Select`/`Omit` 局部更新约定 |

### 6.3 索引规约

- 最左前缀、覆盖索引、组合索引区分度最高在前、等号前置、防隐式转换——文档化进 `docs/` 模板,业务查询评审时对照。

## 七、API 设计规约

| 规则 | 落地 |
| --- | --- |
| URL 全小写,连字符 `-` 分隔(Go 社区) | 模板路由示例 `/api/v1/orders` |
| 版本前缀 `/api/v1` | 路由模板 ✅ |
| 统一响应结构 | `{"code","message","data"}` ✅ `webiris.OK/Fail` |
| 错误码分级(手册 A/B/C) | 4.2 待落地 |
| 幂等(写接口) | 文档约定(Idempotency-Key 可选用) |
| 分页参数规范 | `page`/`pageSize` 模板 ✅ `datasource.Page` |
| 认证:JWT Bearer + scope | ✅ `webiris.Auth` |
| 文档 | 待落地:Swagger(计划) |

## 八、二方库与版本规约(手册六(二))→ go.mod 落地

| 手册 | go.mod 落地 | 状态 |
| --- | --- | --- |
| 语义化版本 `主.次.修订`,从 1.0.0 起 | ✅ 已发布 v1.x.x 语义化 | ✅ |
| 禁止 SNAPSHOT 依赖 | ✅ 全部正式版本 | ✅ |
| 版本统一管理 | go.mod 单一来源 ✅ | ✅ |
| 二方库精简可控(不引无用依赖) | 定期 tidy;新增依赖评审 | ✅ |
| 依赖升级保持仲裁稳定 | go.sum 锁定 + 升级评审 | ✅ |
| 版本变化可追溯 | commit + tag + Release notes ✅ | ✅ |
| 接口返回值禁枚举类型 | Go 导出常量返回 OK(Go 惯例不强制) | 文档说明 |

## 九、并发规约(手册一(七))→ Go 落地

| 手册 | Go 落地 |
| --- | --- |
| 单例必须线程安全 | ✅ container(simpleioc):RWMutex + per-bean 锁 |
| 线程/线程池指定有意义名称 | goroutine 无法命名;文档约定:并发组件用 errgroup/受控池,不用裸 goroutine 风暴 |
| 资源必须通过池提供 | 连接池(数据库/Redis)✅;goroutine 池约定 |
| 线程池禁止无界队列 | Go channel 有界 ✅(SSE/WS 队列 64) |
| 时间格式线程安全 | Go `time.Time` 值类型天然安全 ✅ |

## 十、单元测试规约(手册三)

| 手册 | 落地 |
| --- | --- |
| 测试命名:被测类名+Test | Go:`TestXxx` ✅ |
| 单元测试不依赖外部环境 | ✅ 集成测试 env 门控,单元测试零外部依赖 |
| 测试隔离(不共享状态) | ✅ container.Reset、withCleanRegistry |
| 断言清晰 | 标准库 + httptest;错误信息含期望/实际 |
| 真实服务测试与单测分离 | ✅ `GO_BLACKBOX_*` 环境变量门控 |
| 测试必须有注释说明场景 | ✅ 全仓库遵循 |

## 十一、安全规约(手册四)

| 手册 | 落地 |
| --- | --- |
| 用户输入校验(防注入/XSS) | webiris 参数校验约定 + CORS/安全头中间件 ✅ |
| 敏感信息不落日志/响应 | masker + Redact + 日志脱敏 ✅ |
| SQL 注入防护 | GORM 参数化 ✅ |
| 认证/授权 | JWT + scope ✅;RBAC 待落地 |
| 越权防护 | 文档约定(资源级鉴权) |
| 密码存储 | 文档约定(bcrypt/argon2) |
| 限流防刷 | ✅ webiris.Limit |

## 十二、设计规约(手册七)

| 手册 | 落地 |
| --- | --- |
| 单一职责 | 模块划分(分层重构)✅ |
| 组合优于继承 | Go 天然组合 ✅ |
| 依赖倒置(依赖抽象) | 接口 + container 注入 ✅ |
| 开闭原则 | Builder 扩展点 ✅ |
| 公共逻辑抽取,禁重复代码 | framework/component 分层 ✅ |
| 状态>3 用状态图 | 文档约定(业务模板) |
| 设计文档沉淀 | docs/ 规划 ✅ |

## 十三、落地状态总览

| 批次 | 内容 | 版本 |
| --- | --- | --- |
| ✅ 已完成 | 分层目录重构、统一响应、错误码骨架、日志规范、数据库模型基础、测试规范、安全中间件 | v1.3.0-v1.8.0 |
| 🔜 待落地 | 手册错误码常量表(A/B/C)、`is_deleted`/`gmt_create` 字段名统一、模型命名模板(DO/DTO/VO/Query)、数据库规范文档、API 模板示例 | v1.9.0+ |
| 📋 规划 | RBAC、Swagger、OTel | 按需 |

---

*规范文件随脚手架迭代持续更新;代码评审以本文档 + AGENTS.md 为基线。*
