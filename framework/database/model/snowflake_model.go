package model

import (
	"time"

	sid "github.com/Connorig/go-blackbox/framework/database/id"
	"gorm.io/gorm"
)

// SnowflakeModel 使用雪花算法 ID(int64)的公共模型。
// 嵌入后主键由脚手架在 BeforeCreate 自动生成(仅当 ID 为零值),
// 字段命名对齐阿里手册:gmt_create / gmt_modified / is_deleted。
//
//	type Order struct {
//	    model.SnowflakeModel
//	    OrderNo string `gorm:"column:order_no;uniqueIndex:uk_order_no"`
//	}
//
// 分布式部署时通过 id.SetNode(节点号) 为每个实例分配不同节点,
// 也可在启动前设置环境变量 GOBLACKBOX_SNOWFLAKE_NODE。
type SnowflakeModel struct {
	ID          int64     `gorm:"primarykey;column:id"`              // 雪花主键(自动生成)
	GmtCreate   time.Time `gorm:"column:gmt_create"`                 // 创建时间(自动维护)
	GmtModified time.Time `gorm:"column:gmt_modified"`               // 修改时间(自动维护)
	IsDeleted   int       `gorm:"column:is_deleted;default:0;index"` // 逻辑删除 0/1
}

// BeforeCreate 自动生成雪花 ID(仅零值时)并填充时间。
func (m *SnowflakeModel) BeforeCreate(_ *gorm.DB) error {
	if m.ID == 0 {
		value, err := sid.NextID()
		if err != nil {
			return err
		}
		m.ID = value
	}
	now := time.Now()
	m.GmtCreate = now
	m.GmtModified = now
	return nil
}

// BeforeUpdate 更新时自动维护 gmt_modified。
func (m *SnowflakeModel) BeforeUpdate(tx *gorm.DB) error {
	m.GmtModified = time.Now()
	tx.Statement.SetColumn("gmt_modified", m.GmtModified)
	return nil
}
