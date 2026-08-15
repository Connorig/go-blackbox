# ElasticSearch 与对象存储指南(ES_STORAGE_GUIDELINES)

`framework/es`(ElasticSearch)+ `framework/storage`(对象存储,兼容 S3 协议:MinIO/阿里云 OSS/腾讯云 COS)。

## 一、ElasticSearch(EsTemplate)

```go
client, err := es.NewClient(es.Config{
    Addresses: []string{"http://127.0.0.1:9200"},
    Username:  "elastic",        // 可选
    Password:  "***",            // 可选
    Timeout:   10 * time.Second,
})
// 注册进 gbxioc,业务注入使用
gbxioc.RegisterInstance(client)

// 写入/获取/删除
client.Index(ctx, "orders", "1001", order)          // 写入(覆盖)
client.Get(ctx, "orders", "1001")                   // 原始 JSON;不存在返回 nil
client.Delete(ctx, "orders", "1001")

// 查询(原生 DSL)
query := []byte(`{"query":{"match":{"customer":"connor"}}}`)
result, err := client.Search(ctx, "orders", query)
result.Hits.Total.Value   // 命中数
result.Hits.Hits[0].ID    // 文档 ID
result.Hits.Hits[0].Source // _source 原始 JSON

// 索引管理
client.CreateIndex(ctx, "orders", []byte(`{"settings":{...}}`))
client.ExistsIndex(ctx, "orders")

// 原生客户端(高级操作:聚合/滚动/别名...)
raw := client.Client() // *elasticsearch.Client
```

## 二、对象存储(StorageTemplate)

```go
client, err := storage.NewClient(storage.Config{
    Endpoint:  "127.0.0.1:9000",            // MinIO;OSS 填 oss-cn-xxx.aliyuncs.com
    AccessKey: "minioadmin",
    SecretKey: "minioadmin",
    UseSSL:    false,
    Region:    "oss-cn-hangzhou",           // OSS 必需
})
gbxioc.RegisterInstance(client)

// 上传/下载/删除
client.EnsureBucket(ctx, "images")
client.PutBytes(ctx, "images", "2026/08/avatar.png", data, "image/png")
data, _ := client.GetBytes(ctx, "images", "2026/08/avatar.png")
client.Delete(ctx, "images", "2026/08/avatar.png")

// 流式上传(大文件)
file, _ := os.Open("big.zip")
client.Put(ctx, "files", "big.zip", file, fileSize, "application/zip")

// 预签名直传(前端直传,不暴露密钥)
uploadURL, _ := client.PresignedPut(ctx, "images", "tmp/x.png", 15*time.Minute)
downloadURL, _ := client.PresignedGet(ctx, "files", "big.zip", time.Hour)

// 列举/信息
objects, _ := client.List(ctx, "images", "2026/", 100)
info, _ := client.Stat(ctx, "images", "2026/08/avatar.png")

// 原生客户端
raw := client.Client() // *minio.Client
```

## 三、测试

真实服务测试通过环境变量启用(未配置自动跳过,不影响 CI):

| 环境变量 | 服务 |
|---|---|
| `GO_BLACKBOX_ES_ADDR` | ElasticSearch 地址(如 http://127.0.0.1:9200) |
| `GO_BLACKBOX_STORAGE_ENDPOINT` | 对象存储地址(MinIO 默认 minioadmin/minioadmin) |
| `GO_BLACKBOX_STORAGE_ACCESS_KEY` / `_SECRET_KEY` | 对象存储密钥(可选,默认 minioadmin) |

## 四、错误处理约定

- 连接失败:NewClient 即报错(Ping 验证),避免运行期才发现不可用
- ES 文档不存在:`Get` 返回 (nil, nil),不视为错误
- 存储对象不存在:`GetBytes/Stat` 返回错误,调用方按 404 语义处理
- 全部操作支持 context 超时取消
