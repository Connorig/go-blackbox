# 国际化指南(I18N_GUIDELINES)

`component/i18n` 提供轻量国际化组件:多语言消息资源、key 翻译、占位符参数、
请求语言检测。对标 Java ResourceBundle + Spring MessageSource。

## 一、资源文件(langs/ 目录)

```json
// langs/zh-CN.json
{
  "order.created": "订单创建成功,编号 {{order_no}}",
  "greet": "你好, {{name}},你有 {{count}} 条消息"
}

// langs/en-US.json
{
  "order.created": "Order created, id {{order_no}}",
  "greet": "Hello {{name}}, you have {{count}} messages"
}
```

## 二、快速使用

```go
import "github.com/Connorig/go-blackbox/component/i18n"

// ① 加载资源(文件名 = 语言标识)
bundle := i18n.NewBundle()
if err := bundle.LoadDir("langs"); err != nil {
    log.Fatalf("load i18n: %v", err)
}

// ② 翻译:语言缺失回退默认(zh-CN),再缺失返回 key 本身
message := bundle.T("en-US", "order.created", map[string]interface{}{
    "order_no": 1001,
}) // "Order created, id 1001"

// ③ fmt 风格格式化
message = bundle.Tf("zh-CN", "greet", "张三", 3)

// ④ 请求语言检测(Accept-Language)
lang := bundle.DetectLanguage(ctx.GetHeader("Accept-Language"))
message = bundle.T(lang, "order.created", params)
```

## 三、API

| API | 说明 |
| --- | --- |
| `NewBundle()` | 创建;默认回退 zh-CN |
| `SetFallback(lang)` | 自定义回退语言 |
| `Register(lang, messages)` | 注册语言资源(同名覆盖) |
| `LoadDir(dir)` | 目录加载 `langs/*.json`(无文件/解析失败报错) |
| `T(lang, key, params...)` | 翻译;占位符 `{{key}}` 支持嵌套键 `{{user.name}}`;缺失参数保留占位符 |
| `Tf(lang, key, args...)` | fmt.Sprintf 风格格式化 |
| `DetectLanguage(acceptLang)` | 解析 Accept-Language(zh-cn → zh-CN 归一;未匹配回退) |
| `Langs()` / `Has(lang)` | 已注册语言列表 / 判断 |

## 四、与错误码联动(推荐)

```go
// 业务错误消息国际化:错误码 + i18n key 约定
if err := apperr.New(apperr.CodeParamError, bundle.T(lang, "errors.invalid_param")); err != nil {
    return
}
```

建议资源中 `errors.*` 前缀统一管理错误文案,与 apperr 错误码一一对应。


## 六、与 Web 集成(请求语言中间件)

`webiris.Language` 中间件按 Accept-Language 自动检测请求语言并写入上下文:

```go
import "github.com/Connorig/go-blackbox/component/i18n"
import "github.com/Connorig/go-blackbox/framework/web" // 包名 webiris

bundle := i18n.NewBundle()
_ = bundle.LoadDir("langs")

// ① 挂载中间件(建议全局)
app.Use(webiris.Language(bundle))

// ② 业务 handler 内按请求语言翻译
app.Get("/api/order", func(ctx iris.Context) {
    message := bundle.T(webiris.Lang(ctx), "order.created", params)
    webiris.OK(ctx, map[string]interface{}{"message": message})
})
```

| API | 说明 |
| --- | --- |
| `webiris.Language(bundle)` | 中间件;bundle 负责识别与回退,nil 时恒默认语言 |
| `webiris.Lang(ctx)` | 读取当前请求语言(未设置返回默认 zh-CN;nil ctx 安全) |

语言检测规则见「请求语言检测」:`Accept-Language: zh-CN,zh;q=0.9` → `zh-CN`;未注册语言跳过取下一个;全不匹配回退默认。

## 六、注意事项

- 语言标识统一小写-大写格式(zh-CN/en-US),文件名与之一致
- T 的缺失参数**保留占位符不报错**(翻译兜底优先);需要严格校验用 notify.Render
- LoadDir 在启动时调用一次;运行期变更用 Register 覆盖
- 与通知模板(notify.RegisterTemplate)语法一致,可共用占位符习惯


## 六、语言资源热加载(不改代码不重启)

`WatchDir` 周期扫描语言目录,文件变化自动重载:

```go
// ① 启动热加载(默认轮询 30s;变化自动生效并回调)
go bundle.WatchDir(ctx, "langs", 10*time.Second, func() {
    logger.Info("language resources reloaded")
})

// ② 或与配置中心联动:配置变更时手动重载
go cfgClient.Watch(ctx, 15*time.Second, func(content string) {
    _ = bundle.LoadDir("langs") // 同名 key 覆盖,原子生效
})
```

行为说明:
- 指纹 = 文件名 + mtime + size + 内容哈希;任一变化触发重载
- 首次扫描不回调;目录暂不可读或加载失败保留旧资源(下轮重试)
- 同 key 覆盖更新;删除 key 需要重启(或调用方显式 Register 空值)

注意:
- 生产环境语言文案建议走配置中心/发布流程,本地目录热加载适合开发与运营快速调文案
- 多实例部署每实例独立轮询,变更在 interval 内陆续生效(短暂不一致可接受)

