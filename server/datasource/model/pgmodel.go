package model

import (
	"gorm.io/gorm"
	"time"
)

/**
* @Author: Connor
* @Date:   23.7.24 16:34
* @Description:
 */

// Model 默认表需携带的必须字段
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
