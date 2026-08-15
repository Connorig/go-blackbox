# 安全防护指南(SECURITY_GUIDELINES)

go-blackbox 接口前置安全防护能力:SQL 注入检测、DoS 防护、身份认证,以及常用工具集。
目标:业务项目按本文档组合中间件,即可获得完整的安全基线。

## 一、中间件组合(推荐全量启用)

```go
builder.EnableWeb(appbox.TimeFormat, ":8080", "info", func(app *iris.Application) {
    // ① 基础链路
    app.Use(webiris.RequestID, webiris.AccessLog, webiris.SecurityHeaders, webiris.ErrorHandler)

    // ② DoS 防护:限流(全局限流)+ 请求体上限 + 超时
    app.Use(webiris.Limit(100, 200, nil))          // 令牌桶:100 QPS,突发 200
    app.Use(webiris.BodyLimit(1 << 20))            // 请求体上限 1MB(超限 413)
    app.Use(webiris.Timeout(10 * time.Second))     // 慢请求超时(超时 504)

    // ③ SQL 注入前置拦截(query + body 特征检测,命中 400)
    app.Use(webiris.SQLGuard())

    // ④ 身份认证(内部接口;登录/健康探针白名单放行)
    app.Use(webiris.Auth(webiris.AuthConfig{
        Whitelist: []string{"/health", "/api/v1/login"},
        Scope:     "user:read",
    }))
})
```

| 中间件 | 防护目标 | 失败响应 |
|---|---|---|
| `webiris.Limit` | 流量洪峰(DoS 限流) | 429 `B0210` |
| `webiris.BodyLimit` | 超大请求体耗尽内存 | 413 `A0400` |
| `webiris.Timeout` | 慢速请求占用连接 | 504 `B0100` |
| `webiris.SQLGuard` | SQL 注入(query/body 特征) | 400 `A0400` |
| `webiris.Auth` | 用户 token 身份校验(JWT + scope) | 401 `A0301` / 403 `A0312` |
| `webiris.CORS` | 跨域白名单 | — |
| `webiris.SecurityHeaders` | 安全响应头(XSS/点击劫持等) | — |

### SQL 注入检测覆盖(component/security)

- 联合查询:`UNION SELECT`
- 布尔盲注:`' OR '1'='1`、`OR 1=1`
- 注释注入:`--`、`#`、`/* */`(MySQL 版本注释)
- 堆叠语句:`; DROP TABLE`、`; SELECT`
- 危险操作:`DROP/TRUNCATE TABLE`、`DROP DATABASE`
- 探测/注入函数:`information_schema`、`SLEEP()`、`BENCHMARK()`、`UPDATEXML()`、`LOAD_FILE()`、`INTO OUTFILE`

业务代码也可直接调用:

```go
if security.IsSQLInjection(userInput) {
    return apperr.New(apperr.CodeRequestParamError, "invalid parameter content")
}
```

⚠️ 前置拦截只是第一道防线,**查询必须使用参数化/GORM 预编译**,不能依赖黑名单过滤。

## 二、token 身份校验体系

| 环节 | 组件 | 说明 |
|---|---|---|
| 签发 | `apptoken.GenTokenFull(userID, email, scope, orgID, deptID)` | 携带权限 + 组织身份 |
| 校验 | `webiris.Auth` | Bearer token,算法白名单 HS256,issuer 校验 |
| 权限 | `AuthConfig.Scope` | 逗号分隔权限,缺失返回 403 |
| 数据范围 | `webiris.DataScope(ctx)` | 组织/部门隔离(见 DATABASE_STANDARDS 数据权限章节) |
| 轮换 | `apptoken.SetSecretKeys` | 多密钥宽限期,撤销旧密钥即时生效 |

## 三、常用工具集(component/util)

对标 Java commons/hutool 的常用能力:

| 函数 | 用途 | 对标 |
|---|---|---|
| `util.CopyProperties(dst, src)` | 结构体同名字段拷贝(数值/字符串自动转换) | BeanUtils.copyProperties |
| `util.DeepCopy(src)` | 深拷贝(struct/map/slice/指针嵌套) | 序列化深拷贝 |
| `util.FieldValue / SetFieldValue` | 按字段名读写(支持 `User.Name` 嵌套) | 反射字段访问 |
| `util.MD5 / SHA1 / SHA256` | 摘要计算 | commons-codec |
| `util.RandomString(n)` | 随机字母数字串 | RandomStringUtils |
| `util.UUID / UUIDOrEmpty` | UUID v4 | java.util.UUID |
| `util.Date / StrToTime / WeekAround` | 时间戳格式化/解析/周界 | DateUtils |
| `util.IP2Long / Long2IP` | IP 数值互转 | InetAddress |
| `util.SliceUniq / SliceRand` | 切片去重/随机取 | CollectionUtils |
| `util.MarshalNoEscapeHTML` | JSON 序列化(不转义 HTML) | JSON.toJSONString |
| `util.AddSlashes / StripSlashes` | 斜杠转义 | addslashes |
| `util.SetTimezone` | 全局时区设置 | TimeZone.setDefault |

```go
// 示例:DTO → Entity 拷贝(常用)
var entity UserEntity
util.CopyProperties(&entity, userDTO)   // Age int → int64 自动转换

// 深拷贝后修改不影响原对象
clone, _ := util.DeepCopy(original)

// 通用工具
hash := util.MD5(util.RandomStringMust(16))
```

## 四、日志与审计安全

- `security.SanitizeLog(value)`:清除控制字符/换行,防日志注入(CRLF)
- `security.ContainsControlChars(value)`:检测不可见控制字符
- `webiris.RequestID`:链路追踪,审计日志关联
