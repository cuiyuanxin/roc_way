// Package model 定义 admin 应用的持久化对象。
//
// 与领域层（domain.User）的区别：
//   - 不含业务行为（不变更方法）
//   - 字段带 GORM tag，专为存储优化
//   - 由 repository 包负责与 domain.User 互转
package model

import (
	"gorm.io/gorm"
)

// User GORM 用户表映射对象。
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"size:64;uniqueIndex;not null;default:'';comment:用户名" json:"username"`
	Email     string         `gorm:"size:128;index;not null;default:'';comment:邮箱" json:"email"`
	Name      string         `gorm:"size:64;not null;default:'';comment:昵称" json:"name"`
	Password  string         `gorm:"size:128;not null;default:'';comment:密码" json:"password"`
	CreatedAt int64          `gorm:"comment:创建时间" json:"created_at"`
	UpdatedAt int64          `gorm:"comment:更新时间" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index;comment:删除时间" json:"-"`
}

// TableName 指定表名。
func (User) TableName() string { return "users" }
