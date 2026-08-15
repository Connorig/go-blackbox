# 接口文档指南(APIDOC_GUIDELINES)

`framework/apidoc` 提供接口文档自动生成(对标 Springdoc/Swagger):
注册路由时自动推断文档,输出 OpenAPI 3.0 定义 + 内嵌 API 浏览页面。

## 一、接入

```go
builder.EnableWeb(appbox.TimeFormat, ":8080", "info", func(app *iris.Application) {
    // 业务路由:用 apidoc 包装器注册(替代 app.Get/Post/Put/Delete,行为一致)
    apidoc.GET(app, "/api/v1/orders/{id:int64}", handler.GetOrder)
    apidoc.POST(app, "/api/v1/orders", handler.CreateOrder,
        apidoc.Summary("创建订单(自定义)"),
        apidoc.Responds(&model.Order{}),
    )

    // 标准 CRUD 一次注册 5 个接口 + 完整文档(gencode 风格)
    apidoc.CRUD(app, "/api/v1/products", &model.Product{},
        handler.ListProduct, handler.GetProduct,
        handler.CreateProduct, handler.UpdateProduct, handler.DeleteProduct)

    // 文档服务
    apidoc.Register(app, "/docs", apidoc.Config{
        Title: "订单服务", Version: "v1.0.0", Description: "示例服务",
    })
})
```

启动后:
- `GET /docs` API 浏览页面(内嵌,无外部资源:方法/路径/摘要/参数/响应 分组展示)
- `GET /docs/api.json` OpenAPI 3.0 定义(可接第三方工具)

## 二、约定优先(自动推断,零手写)

| 推断项 | 规则 |
|---|---|
| 摘要 | 函数名动词映射:Get→获取 / List→查询列表 / Create→创建 / Update→更新 / Delete→删除 / Login→登录... |
| 分组 Tag | 路径第一段业务段(如 /api/v1/orders → orders) |
| 路径参数 | 路由模板 `{id:int64}` 自动解析(path, 必填) |
| 默认响应 | 200 通用响应(未指定模型时) |
| 安全 | 默认 BearerAuth |

**只需写差异**:自定义摘要/描述、查询参数、请求体模型、精确响应模型。

```go
apidoc.GET(app, "/api/v1/orders/{id:int64}", GetOrder)   // 一行,全自动
```

## 三、选项 API

| 选项 | 说明 |
|---|---|
| `apidoc.Summary(text)` | 自定义摘要 |
| `apidoc.Description(text)` | 详细描述 |
| `apidoc.Tag(name)` | 分组(可多次) |
| `apidoc.PathParam(name, type, required, desc)` | 路径参数描述 |
| `apidoc.QueryParam(name, type, required, desc)` | 查询参数 |
| `apidoc.Body(model, required, desc)` | 请求体模型(反射生成 Schema) |
| `apidoc.Responds(model)` | 200 响应模型(包装为统一响应 data) |
| `apidoc.Respond(status, desc, model)` | 指定状态码响应 |
| `apidoc.Deprecated()` | 弃用标记 |

## 四、Schema 自动生成(反射)

- 结构体字段 → JSON Schema(类型/嵌套/数组/map)
- `time.Time` → `string(date-time)`
- `json:"x,omitempty"` → 非必填;无 omitempty 非指针 → 必填
- 统一响应包装:`{code, message, data}`(data 为业务模型)
- 模型注册到 `components.schemas` 供引用

## 五、gencode 联动

gencode 生成的 CRUD 路由代码段已改用 `apidoc.CRUD` 注册——生成即带完整文档,业务零成本。

## 六、注意事项

- `apidoc.*` 替代 `app.Get/Post/Put/Delete` 注册路由,行为完全一致(iris.Handle)
- 文档收集器为进程内单例;多服务实例各自生成
- `/docs` 页面建议内网开放;`api.json` 可挂 Auth(如需)
