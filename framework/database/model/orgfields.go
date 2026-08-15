package model

// OrgFields 组织/部门公共字段(多租户与数据权限基础)。
// 叠加在任意 ID 模型之上;由业务侧在写入时显式赋值
// (例如从 JWT 上下文或组织中间件填充),脚手架不自动维护。
//
//	type Order struct {
//	    model.SnowflakeModel
//	    model.OrgFields
//	    OrderNo string `gorm:"column:order_no;uniqueIndex:uk_order_no"`
//	}
type OrgFields struct {
	OrgID  int64 `gorm:"column:org_id;index"`  // 组织 ID
	DeptID int64 `gorm:"column:dept_id;index"` // 部门 ID
}
