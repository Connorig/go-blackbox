package model

import (
	"time"

	"gorm.io/gorm"
)

/**
* @Author: Connor
* @Date:   23.7.24 16:34
* @Description:
 */

// Model 默认表需携带的必须字段（GORM 惯例字段名）。
type Model struct {
	ID        int `gorm:"primarykey"` // 主键
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"` // 逻辑删除
}

// AuditModel 在基础字段上增加创建人/更新人审计字段。
// 审计字段需要业务在写入时显式赋值（例如通过中间件从用户上下文填充）。
type AuditModel struct {
	Model
	CreatedBy string `gorm:"size:64"` // 创建人标识（用户 ID 或账号）
	UpdatedBy string `gorm:"size:64"` // 更新人标识（用户 ID 或账号）
}

// StandardModel 遵循阿里开发手册（泰山版）数据库字段命名规范：
//   - 主键: id
//   - 创建时间: gmt_create（禁止 create_time 混用）
//   - 修改时间: gmt_modified（更新记录必须同步维护）
//   - 逻辑删除: is_deleted（0 未删除 / 1 已删除），布尔属性在模型中去掉 is 前缀
//
// 时间字段由 GORM hooks 自动维护，无需业务赋值。
// 索引命名规范：普通索引 idx_ 前缀、唯一索引 uk_ 前缀（如 uk_order_no）。
type StandardModel struct {
	ID          int64     `gorm:"primarykey;column:id"`              // 主键
	GmtCreate   time.Time `gorm:"column:gmt_create"`                 // 创建时间（自动维护）
	GmtModified time.Time `gorm:"column:gmt_modified"`               // 修改时间（自动维护）
	IsDeleted   int       `gorm:"column:is_deleted;default:0;index"` // 逻辑删除 0/1
}

// BeforeCreate 写入时自动填充创建/修改时间。
func (m *StandardModel) BeforeCreate(_ *gorm.DB) error {
	now := time.Now()
	m.GmtCreate = now
	m.GmtModified = now
	return nil
}

// BeforeUpdate 更新时自动维护修改时间。
// 使用 SetColumn 强制写入，保证 map 与 struct 两种更新方式下 gmt_modified 都会被刷新
// （GORM 的 map 更新会忽略 hook 中直接修改的字段值）。
func (m *StandardModel) BeforeUpdate(tx *gorm.DB) error {
	m.GmtModified = time.Now()
	tx.Statement.SetColumn("gmt_modified", m.GmtModified)
	return nil
}
