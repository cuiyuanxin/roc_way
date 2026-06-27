// Package model 提供 admin 应用所有持久化对象的 GORM 映射 + 迁移入口。
//
// 设计动机：迁移列表（AutoMigrate 的入参）放在 model 包集中维护，
// 新增持久化对象时只需在本文件加一行，无需改动 app.go 的 Migrate 函数，
// 也避免 app.go 出现"明明不直接用 model.User.Migrate() 却要 import model"的弱依赖。
package model

import "gorm.io/gorm"

// All 返回 admin 应用所有需要迁移的持久化对象。
//
// 顺序无依赖关系（GORM 会按依赖图自动建表）；按业务域分块列出便于阅读。
// 新增实体 → 在本函数追加即可。
func All() []any {
	return []any{
		// 用户聚合
		&User{},

		// 锁定跟踪（与 auth_login_logs 严格区分，详见 LoginAudit.TableName）
		&LoginAudit{},

		// 登录日志（保留 / 删除策略由独立日志管理模块决定，janitor 不自动清理）
		&LoginLog{},
	}
}

// Migrate 调用 GORM AutoMigrate 迁移本包所有持久化对象。
//
// 封装这一层的好处：
//   - app.go 调 model.Migrate(db) 即可，不必关心具体有哪些实体；
//   - 单测可直接用 model.Migrate(testDB) 起一套临时 schema。
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(All()...)
}
