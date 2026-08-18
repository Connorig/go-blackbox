# 直播对接指南(LIVE_GUIDELINES)

`framework/live` 是 SRS 流媒体服务器对接层:**回调接收(SRS → gbx)+ API 客户端(gbx → SRS)+ 录制(DVR)+ 回调签名**。
模块只做"对接",不内置业务规则——鉴权裁决由业务注入。

## 一、接入(一行启用)

```go
builder.EnableWeb(appbox.TimeFormat, ":8080", "info", func(app *iris.Application) {
    live.Provide(app, live.Config{
        APIBase:       "http://127.0.0.1:1985", // SRS HTTP API(容器内用 host.docker.internal)
        CallbackMount: "/api/live",             // 需与 SRS http_hooks 配置一致
        CallbackSecret: os.Getenv("SRS_CALLBACK_SECRET"), // 回调签名共享密钥(可选)
        Handlers: &live.Handlers{
            OnPublish: func(ctx iris.Context, info *live.PublishInfo) error {
                // 业务鉴权:密钥校验/封禁名单/开播权限
                if info.Param != "?key=***" {
                    return errors.New("invalid stream key") // 返回 error = 拒流
                }
                return nil // nil = 放行
            },
        },
    })
})
```

SRS 侧配置(参考):

```conf
vhost __defaultVhost__ {
    http_hooks {
        enabled on;
        on_publish   http://host.docker.internal:8080/api/live/on_publish;
        on_play      http://host.docker.internal:8080/api/live/on_play;
        on_unpublish http://host.docker.internal:8080/api/live/on_unpublish;
        on_connect   http://host.docker.internal:8080/api/live/on_connect;
        on_dvr       http://host.docker.internal:8080/api/live/on_dvr;
        on_hls       http://host.docker.internal:8080/api/live/on_hls;
    }
    dvr {
        enabled on;
        dvr_path ./objs/nginx/html/[app]/[stream].[timestamp].flv;
        dvr_plan session;   # 会话录制:推流开始即录
    }
}
```

## 二、回调(SRS → gbx,6 类)

| 路由 | 类型 | 注入函数 | 裁决 |
|---|---|---|---|
| `{mount}/on_publish` | 推流鉴权(最关键) | `OnPublish(ctx, *PublishInfo) error` | nil 放行 / error 拒绝(403) |
| `{mount}/on_play` | 播放鉴权 | `OnPlay(ctx, *PlayInfo) error` | 同上 |
| `{mount}/on_connect` | 连接鉴权(可选) | `OnConnect(ctx, *ConnectInfo) error` | 同上;未注入默认放行 |
| `{mount}/on_unpublish` | 下播通知 | `OnUnpublish(ctx, *UnpublishInfo)` | 无裁决 |
| `{mount}/on_dvr` | 录制完成 | `OnDvr(ctx, *DvrInfo)` | 无裁决 |
| `{mount}/on_hls` | 切片生成 | `OnHls(ctx, *HlsInfo)` | 无裁决 |

**关键约定**(SRS 5 实测):
- 推流 URL 的 query(如 `?key=***`)在 **`param` 字段**(带 `?` 前缀),**不在 stream 字段**
- 放行响应 `{"code":0}`;拒绝响应 403 + `{"code":1,"msg":"..."}`
- 回调必须快速返回(<1s),慢响应 = 断流
- body 非 JSON 容错:on_publish/on_play 解析失败拒绝;on_connect 解析失败放行

### 回调签名(防伪造回调)

配置 `CallbackSecret` 后,全部回调请求必须携带签名头,否则 403:

```go
// 签名算法:HMAC-SHA256(hex),签名内容 = 大写方法 + 路径 + 原始 body
verify := live.NewCallbackSignature(secret, "X-SRS-Signature") // 自定义 headerKey
app.Use(verify.Wrap()) // 或单独 party 使用
// 工具函数(测试/文档示例):live.SignPayload(secret, "POST", "/api/live/on_publish", body)
```

- 未配置 secret 时验签关闭(对接期友好)
- 伪造回调(无签名/签名不匹配)直接 403,不进入业务 handler
- 校验用 `hmac.Equal` 常量时间比较

## 三、API 客户端(gbx → SRS)

```go
client := live.NewClient("http://127.0.0.1:1985", 5*time.Second)
// 或 Provide 后 live.Get()

version, _ := client.Version(ctx)                    // GET /api/v1/versions
streams, _ := client.ListStreams(ctx)                // GET /api/v1/streams/(尾斜杠!)
clients, _ := client.ListClients(ctx)                // GET /api/v1/clients/(尾斜杠!)
client.KickStream(ctx, "demo")                       // 踢流:两步(查流→取 cid→DELETE)
client.KickClient(ctx, "cid-123")                    // DELETE /api/v1/clients/{cid}(无尾斜杠)
```

**Stream 字段**(直观化):`Name/App/Vhost/URL/PublishCID/VideoCodec/AudioCodec/Width/Height`。

**硬性约定**:
- streams/clients 列表 API **必须带尾部斜杠**(不带 301),删除客户端**不带**
- 踢流两步:ListStreams 按 name 取 `publish_cid` → DELETE `/api/v1/clients/{cid}`
- 非 JSON 响应(如 405 "Method Not Allowed")容错为结构化错误,不 panic
- 流不存在/无活跃推流返回明确错误(业务可区分)

## 四、录制 DVR(gbx → SRS)

SRS 配置 `dvr`(session 模式)后,通过 API 客户端管理录制:

```go
taskID, err := client.StartRecord(ctx, "demo", "/data/dvr/demo.flv") // POST /api/v1/dvr/start
err = client.StopRecord(ctx, "demo")                                 // POST /api/v1/dvr/stop
```

- 录制完成由 SRS 回调 `on_dvr` 通知业务(携带文件路径/大小/时长)
- 响应非 JSON 或 code 非 0 均转换为结构化错误,不 panic
- 结合 `taskqueue`(延迟任务)可实现「自动下播」:鉴权通过后 `Submit(时长, 下播)` 延迟踢流

## 五、测试与验证

- 单测 11 组全绿:回调放行/拒绝/默认放行/非 JSON 容错/通知回调;API 版本/流列表字段提取/踢流两步/客户端列表/405 容错(mock SRS)
- 签名单测:验签通过/拒绝/未配置放行/算法一致性
- 真实联调:本地 Docker SRS 5.0.213(1935/1985/8080)+ ffmpeg 推流:
  `ffmpeg -re -i test.mp4 -c copy -f flv rtmp://127.0.0.1:1935/live/demo?key=***`

## 六、本期不做(二期预留)

WebSocket 弹幕信令、转码编排、审核联动(业务侧)。
