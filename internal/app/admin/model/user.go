// Package model 定义 admin 应用的领域模型。
//
// 领域模型为纯内部值对象，不参与依赖注入。
package model

import "time"

// User GORM 用户表。
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	Email     string    `gorm:"size:128;uniqueIndex" json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名。
func (User) TableName() string { return "users" }
