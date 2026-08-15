# 低代码生成平台指南(GENCODE_GUIDELINES)

`framework/gencode` 提供 Web 化低代码生成平台(对标 RuoYi 代码生成器):
在线查看数据库表/字段、编辑字段、同步表结构、一键生成 DDD 代码(带覆盖保护)。

## 一、接入

```go
builder.EnableWeb(appbox.TimeFormat, ":8080", "info", func(app *iris.Application) {
    // 安全基线...
    gencode.Register(app, "/gencode", gencode.Config{
        DB:         datasource.MustGetDB(),   // 元数据源(必填)
        ModulePath: "github.com/company/demo", // 业务项目 module 路径(生成 import 用)
        OutputDir:  ".",                       // 生成文件输出根目录(默认当前目录)
        Auth: webiris.Auth(webiris.AuthConfig{ // 管理接口认证(推荐)
            Whitelist: []string{"/health", "/gencode"},
        }),
    })
})
```

浏览器访问 `http://localhost:8080/gencode` 进入管理页面。

## 二、页面功能

| 区域 | 功能 |
|---|---|
| 左侧表列表 | 数据库全部业务表(表名/注释/**已生成**标记),搜索过滤 |
| 字段表(展开) | 字段名/类型/长度/可空/默认值/注释/主键;行内删除 |
| ➕ 新增字段 | 在线添加字段(ALTER TABLE ADD COLUMN,类型/长度/默认/注释) |
| 右侧菜单 | **生成 DDD 代码** / 预览代码 / 同步表结构 / 删除字段 |
| 覆盖保护 | 已生成表再次生成 → 弹窗「将覆盖 N 个文件」,确认后覆盖 |

## 三、API

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/gencode/api/tables` | 表列表(+已生成标记) |
| GET | `/gencode/api/tables/{name}` | 表详情(字段) |
| POST | `/gencode/api/tables/{name}/columns` | 新增字段(DDL) |
| DELETE | `/gencode/api/tables/{name}/columns/{col}` | 删除字段(DDL) |
| POST | `/gencode/api/tables/{name}/sync` | 同步表结构 |
| GET | `/gencode/api/tables/{name}/preview` | 代码预览 |
| POST | `/gencode/api/tables/{name}/generate?force=true` | 生成代码(force=覆盖) |

管理 API 挂认证 + 限流(默认 5 QPS);生产建议仅内网/Admin 端口开放(DDL 敏感)。

## 四、生成内容(DDD 五件套)

| 文件 | 内容 |
|---|---|
| `internal/model/{table}.go` | GORM 模型(StandardModel 基础字段 + 表字段映射,类型自动转换) |
| `internal/filter/{table}.go` | 查询过滤(关键字/精确匹配/分页) |
| `internal/repository/{table}.go` | CRUD + WithTx 事务 |
| `internal/service/{table}.go` | 业务接口 + 实现(校验/防越权) |
| `internal/handler/{table}.go` | CRUD API(GET 列表/详情/POST/PUT/DELETE) |

生成完成后页面给出**路由注册代码段**,复制进 main.go 即完成接入。
生成记录持久化于 `gbx_gen_record` 表(表名/时间/文件哈希),用于已生成标记与覆盖保护。

## 五、支持数据库

| 数据库 | 元数据来源 | 字段增删 |
|---|---|---|
| SQLite | sqlite_master + PRAGMA table_info | ✅ ADD/DROP COLUMN |
| PostgreSQL | information_schema + 注释 | ✅ ADD/DROP COLUMN |
| MySQL | information_schema(TABLE_COMMENT) | ✅ ADD/DROP COLUMN(8.0+) |

## 六、典型流程

1. 数据库建表(或在线新增字段)
2. 打开 `/gencode` → 选择表 → 检查字段
3. 点击「生成 DDD 代码」→ 复制路由片段到 main.go
4. 修改生成的代码(文件头标注 DO NOT EDIT manually,重生成会覆盖)
5. 再次生成时弹窗确认覆盖
