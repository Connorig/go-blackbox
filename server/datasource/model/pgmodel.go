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

// Model gorm默认字段
type Model struct {
	ID        int `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
