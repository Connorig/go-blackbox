# API 设计规范（go-blackbox）

> 依据:阿里巴巴《Java开发手册(泰山版)》开放接口层/错误码规约 + 行业 RESTful 实践。
> 适用范围:go-blackbox 脚手架生成的业务 API。

## 一、URL 与路由

| 规则 | 约定 | 示例 |
| --- | --- | --- |
| URL 全小写 | 单词间用连字符 `-` | `/api/v1/order-items` |
| 版本前缀 | `/api/v{version}`，当前 v1 | `/api/v1/orders` |
| 资源命名 | 复数名词 | `/api/v1/orders`、`/api/v1/orders/{id}` |
| 动作 | 用 HTTP 方法表达，不在 URL 加动词 | `POST /orders`(创建)而非 `/createOrder` |
| 查询参数 | 过滤/分页参数 | `?status=paid&page=1&pageSize=20` |

## 二、统一响应结构

```json
{
  "code": "00000",
  "message": "ok",
  "data": { }
}
```

- `code`:业务错误码(阿里 A/B/C 分级,成功 `00000`)
- `message`:可读信息(成功 `ok`;失败为具体原因,不得含敏感信息)
- `data`:业务负载;失败时省略
- 实现:`webiris.OK(ctx, data)` / `webiris.Fail(ctx, status, apperr.CodeXxx, msg)` / `webiris.RespondError(ctx, err)` ✅

## 三、错误码规约(阿里泰山版)

| 码段 | 含义 | 常用码 |
| --- | --- | --- |
| `A0001` 用户端错误 | 参数/认证/权限/资源 | `A0400` 参数错误 · `A0301` 未授权 · `A0312` 无权限 · `A0427` JSON 解析失败 · `A0501` 请求超限 |
| `B0001` 系统端错误 | 超时/限流/资源 | `B0210` 系统限流 · `B0100` 执行超时 · `B0314` 连接池耗尽 |
| `C0001` 第三方错误 | 数据库/缓存/消息/通知 | `C0300` 数据库出错 · `C0130` 缓存出错 · `C0121` 消息投递失败 · `C0503` 邮件失败 |

- 业务扩展:在一级码下自定义二级码(如 `A0121` 密码长度不够),保持前缀语义
- 常量集中在 `component/error/codes.go`,禁止魔法码 ✅
- 未知错误统一转 `B0001`,服务端日志保留原始错误 ✅

## 四、认证与权限

| 规则 | 落地 |
| --- | --- |
| JWT Bearer 认证 | `webiris.Auth(Whitelist, Scope)` ✅ |
| scope 细粒度权限 | `GenTokenWithScope` + `Auth(Scope:)` ✅ |
| 白名单 | 登录/健康探针等公开接口放行 ✅ |
| 资源级鉴权(越权防护) | 业务层校验资源归属,评审要求 |
| 敏感接口限流 | `webiris.Limit` ✅ |

## 五、分页

- 请求参数:`page`(从 1 开始)、`pageSize`(默认 10,上限 100)
- 响应:`data` 为列表,分页元数据:

```json
{ "code": "00000", "message": "ok",
  "data": { "list": [...], "total": 57, "page": 2, "pageSize": 10, "totalPages": 6 } }
```

- 实现:`datasource.Page(ctx, query, page, pageSize, &list)` ✅(count=0 直接返回)

## 六、幂等与并发

- 写接口建议幂等:客户端携带 `Idempotency-Key` 头,服务端去重
- 重复请求错误码 `A0506`
- 分布式互斥:`cache.TryLock/Lock` ✅

## 七、参数校验

- 必填/格式校验在 Web 层完成(手册分层规约:Web 层做基本参数校验)
- 校验失败返回 `A0400` 参数错误族
- 金额参数以最小货币单位整数传递(分),禁止浮点

## 八、接口文档

- 计划:Swagger 集成(roadmap)
- 当前:README 示例 + `examples/web-basic` 为最小可运行参考

---

*配套:docs/DEVELOPMENT_STANDARDS.md 全文规范 · docs/DATABASE_STANDARDS.md 数据库规范*
