# Excel 模板导入导出指南(EXCEL GUIDELINES)

gbx framework/excel:基于用户预调好的 .xlsx 模板做泛型导入/导出。
模板的样式、格式、公式由业务提前调好,gbx 只填充/读取数据区域——不自动生成 Excel。

## 一、设计约定

| 项 | 约定 |
|---|---|
| 模板文件 | 业务提供,已调好样式/格式(表头/列宽/颜色/公式) |
| 列映射 | struct tag `excel:"列名"` 优先;无 tag 用字段名匹配表头 |
| 表头行 | HeaderRow(1-based,默认 1) |
| 数据起始行 | DataStartRow(1-based,默认 2) |
| 支持类型 | string / int 系列 / float 系列 / bool / 指针类型结构体 |
| 未匹配列 | 导入忽略、导出留空 |

## 二、导出(泛型列表 → 模板填充)

```go
import "github.com/Connorig/go-blackbox/framework/excel"

type OrderRow struct {
	OrderNo  string  `excel:"订单号"`
	Customer string  `excel:"客户名称"`
	Quantity int     `excel:"数量"`
	Price    float64 `excel:"单价"`
	Shipped  bool    `excel:"已发货"`
}

data := []OrderRow{{OrderNo: "SO-001", Customer: "吉利", Quantity: 12, ...}}

// 按模板导出到文件
err := excel.ExportToFile(excel.Template{FilePath: "./templates/order.xlsx"}, data, "./out/orders.xlsx")

// 或导出到 io.Writer(HTTP 下载)
var buf bytes.Buffer
err = excel.Export(excel.Template{FilePath: "./templates/order.xlsx"}, data, &buf)
```

模板样式全部保留(表头颜色/列宽/公式),数据从 DataStartRow 起逐行覆盖。

## 三、导入(Excel 解析 → 泛型列表)

```go
rows, err := excel.ImportFromFile[OrderRow](
	excel.Template{FilePath: "./templates/order.xlsx"}, // 模板表头作映射参考
	"./uploads/orders.xlsx",                            // 用户上传的文件
)
// rows 为 []OrderRow,类型安全
```

- 空单元格跳过(保持零值)
- 数值列自动 strconv 转换;格式错误保留零值(可后续校验)
- 多行数据、空数据区(返回空列表)均安全

## 四、模板示例(业务先调好)

| 订单号 | 客户名称 | 数量 | 单价 | 已发货 | 备注 |
|---|---|---|---|---|---|
| (数据区) | | | | | |

表头即字段映射依据;`excel:"订单号"` 与表头逐字匹配。

## 五、边界与约定

- 模板表头名必须与 tag/字段名**逐字一致**(含空格)
- 复杂类型(嵌套 struct/数组)请拆为平铺字段或用自定义字符串序列化后映射
- 导入大文件建议按需分批(先 GetRows 分片);当前实现一次性读入
- excelize 依赖:xuri/excelize/v2 v2.8.1,Go 1.21+ 兼容
