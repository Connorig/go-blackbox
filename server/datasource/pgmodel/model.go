package pgmodel

import (
	"gorm.io/gorm"
	"time"
)

/**
* @Author: Connor
* @Date:   23.7.24 16:34
* @Description:
 */

// Model gorm默认字段
type Model struct {
	ID        int `gorm:"primarykey"` // 主键ID
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"` // 逻辑删除
}
