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
