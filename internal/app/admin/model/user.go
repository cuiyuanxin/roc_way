// Package model 定义 admin 应用的持久化对象。
//
// 设计：model 同时承担「持久化映射 + 业务行为」两个角色（替代原 domain 层）：
//   - GORM tag 专为存储优化
//   - 业务方法（SetName / SetPasswordHash / NewUser）保留在 model 上，
//     避免 service 散落校验逻辑
//   - 密码哈希字段用 `json:"-"`，handler 出参必须显式映射到 dto 才不会泄漏
package model

import (
	"gorm.io/gorm"
)

// User GORM 用户表映射对象 + 业务行为。
type User struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Username  string         `gorm:"size:64;uniqueIndex;not null;default:'';comment:用户名" json:"username"`
	Email     string         `gorm:"size:128;index;not null;default:'';comment:邮箱" json:"email"`
	NickName  string         `gorm:"size:64;not null;default:'';column:nickname;comment:昵称" json:"nickname"`
	Password  string         `gorm:"size:128;not null;default:'';comment:密码哈希" json:"-"`
	Avatar    string         `gorm:"size:128;not null;default:'';comment:头像" json:"avatar"`
	CreatedAt int64          `gorm:"comment:创建时间" json:"created_at"`
	UpdatedAt int64          `gorm:"comment:更新时间" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index;comment:删除时间" json:"-"`
}

// TableName 指定表名。
func (User) TableName() string { return "users" }
