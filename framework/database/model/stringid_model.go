package model

import (
	"time"

	sid "github.com/Connorig/go-blackbox/framework/database/id"
	"gorm.io/gorm"
)

// StringIDModel 使用 UUID v4 字符串主键的公共模型。
// 适合跨系统合并、客户端生成 ID、不可枚举等场景;
// 主键由脚手架在 BeforeCreate 自动生成(仅当 ID 为空)。
//
//	type Order struct {
//	    model.StringIDModel
//	    OrderNo string `gorm:"column:order_no;uniqueIndex:uk_order_no"`
//	}
type StringIDModel struct {
	ID          string    `gorm:"primarykey;column:id;size:36"`      // UUID 主键(自动生成)
	GmtCreate   time.Time `gorm:"column:gmt_create"`                 // 创建时间(自动维护)
	GmtModified time.Time `gorm:"column:gmt_modified"`               // 修改时间(自动维护)
	IsDeleted   int       `gorm:"column:is_deleted;default:0;index"` // 逻辑删除 0/1
}

// BeforeCreate 自动生成 UUID(仅空值时)并填充时间。
func (m *StringIDModel) BeforeCreate(_ *gorm.DB) error {
	if m.ID == "" {
		value, err := sid.UUID()
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
func (m *StringIDModel) BeforeUpdate(tx *gorm.DB) error {
	m.GmtModified = time.Now()
	tx.Statement.SetColumn("gmt_modified", m.GmtModified)
	return nil
}
