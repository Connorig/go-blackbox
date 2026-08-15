# 数据库开发规范（go-blackbox）

> 依据:阿里巴巴《Java开发手册(泰山版)》MySQL 数据库规约 + GORM 落地映射。
> 适用范围:使用 go-blackbox 脚手架的业务项目建表与数据访问。

## 一、建表规范

### 1.1 表与字段

| 规则 | 约定 | GORM 落地 |
| --- | --- | --- |
| 表名 | 小写下划线，建议业务前缀（如 `t_order`） | `TableName()` 方法或 `gorm:"tableName"` 配置 |
| 字段名 | 小写下划线，单词全小写 | `gorm:"column:xxx"` 显式映射 |
| 主键 | `id`（bigint/自增或雪花） | `StandardModel.ID` |
| 创建时间 | `gmt_create`（禁止 `create_time` 混用） | `StandardModel.GmtCreate`（自动维护） |
| 修改时间 | `gmt_modified` | `StandardModel.GmtModified`（自动维护） |
| 逻辑删除 | `is_deleted`（0 未删/1 已删） | `StandardModel.IsDeleted`（模型属性不加 is 前缀） |
| 布尔字段 | `is_xxx`（1/0），模型属性 `Xxx` | `Enabled bool gorm:"column:is_enabled"` |
| 金额 | `decimal` 或最小货币单位整数 `int64`（分） | 禁止 float/double |
| 小数 | `decimal(M,D)` | 禁止 float/double |
| 字符集 | utf8mb4（支持表情） | 连接串/建表 DDL 配置 |

### 1.2 索引命名

| 索引类型 | 命名 | GORM 落地 |
| --- | --- | --- |
| 主键 | `pk_字段` 或默认 `id` | `primarykey` |
| 唯一索引 | `uk_字段名` | `gorm:"uniqueIndex:uk_order_no"` |
| 普通索引 | `idx_字段名` | `gorm:"index:idx_status"` |
| 组合索引 | `idx_字段1_字段2`，区分度最高列在前 | `gorm:"index:idx_a_b,priority:1"` |

### 1.3 必备字段（推荐使用 `model.StandardModel`）

```go
type Order struct {
    model.StandardModel
    OrderNo string `gorm:"column:order_no;size:32;uniqueIndex:uk_order_no"`
    Amount  int64  `gorm:"column:amount"` // 金额单位：分
    Status  string `gorm:"column:status;size:16;index:idx_status"`
}
```

## 二、SQL 规约 → GORM

| 手册规则 | GORM 落地 |
| --- | --- |
| 禁止 `count(列名)`，使用 `count(*)` | `db.Model(&T{}).Count(&total)` |
| 分页 count 为 0 直接返回 | `datasource.Page` / `PageOn`（先 Count 后 Find）✅ |
| 禁止外键与级联，应用层解决 | 不配置 GORM 外键约束 |
| 禁止存储过程 | — |
| 数据订正先 select 再 update | 代码评审要求 |
| 多表查询必须加表别名 | 原生 SQL 约定：`SELECT t1.* FROM t_order AS t1 ...` |
| `in` 集合 ≤ 1000 | 代码评审约定，超限分批 |
| 查询禁止 `*`，显式字段 | `db.Select("id", "order_no")` |
| 参数化防 SQL 注入 | GORM 全部 `?` 占位 ✅（禁止字符串拼接 SQL） |
| 更新必须带 `gmt_modified` | GORM hooks 自动维护 ✅ |
| 禁止大而全更新 | `db.Select("status").Updates(...)` 局部更新 |
| 事务不滥用，评估回滚方案 | `instance.WithTx` / `datasource.WithTx` ✅ |

## 三、ORM 映射规约

| 手册规则 | 落地 |
| --- | --- |
| 布尔字段库 `is_xxx`、模型去 is | `Enabled bool gorm:"column:is_enabled"` |
| DO 字段类型与库字段匹配 | `int64 ↔ bigint`、`time.Time ↔ datetime` |
| 禁止 HashMap 接收结果集 | Go 中禁止 `map[string]interface{}` 作为查询结果；定义结构体 |
| 查询结果模型与表结构一一对应 | `model` 包定义 DO；DTO/VO 单独定义，不混用 |

## 四、索引规约要点

- 最左前缀:组合索引按查询条件顺序建立
- 覆盖索引:查询列包含在索引中,避免回表
- 区分度最高列在前;等值条件列前置(范围条件后置)
- 防止隐式转换(字段类型不一致导致索引失效)
- `order by` 字段利用索引有序性
- 深分页:offset 过大时用延迟关联/游标改写

## 五、迁移与版本管理

- 结构变更走 `datasource.Migrator` 版本化迁移(每个迁移独立事务)✅
- 禁止直接修改已发布迁移;新增迁移追加
- 生产变更评审 + double check(手册设计规约)

---

*配套:docs/DEVELOPMENT_STANDARDS.md 全文规范 · docs/API_GUIDELINES.md API 规范*

## 数据权限(组织/部门隔离)

公共模型 OrgFields(org_id/dept_id)配套的数据隔离机制,由 framework/database 的 DataScope 提供:

1. 登录签发 token 时写入组织身份:pptoken.GenTokenFull(userID, email, scope, orgID, deptID)(老接口 GenTokenWithScope 行为不变)
2. webiris.Auth 认证后自动注入,业务通过 webiris.DataScope(ctx) 取回:
3. 查询自动过滤: db.WithContext(ctx).Scopes(webiris.DataScope(ctx).Condition()).Find(&list)

`go
// 业务查询(组织内数据隔离)
scope := webiris.DataScope(ctx)   // OrgID/DeptID 来自 JWT claim
var orders []Order
if err := datasource.MustGet().WithContext(ctx).
    Scopes(scope.Condition()).     // 自动追加 org_id/dept_id 条件(零值字段不限制)
    Find(&orders).Error; err != nil {
    return err
}
`

规则:
- 仅设置 OrgID → 按 org_id 过滤;仅设置 DeptID → 按 dept_id 过滤;都设置 → 组合过滤
- 未登录/老 token(无组织字段) → 空范围,不产生过滤条件(业务自行决定是否拒绝)
- 自定义列名: scope.ConditionFor("tenant_id", "dept_id")
- 超管/跨组织场景: 不调用 Scopes 或使用空 DataScope(全量可见)
