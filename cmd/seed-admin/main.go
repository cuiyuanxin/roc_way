// Package main 提供一次性「写入管理员账号」seed 工具。
//
// 用途：初始化 / 重建数据库时快速创建超级管理员账号。
// 用法（项目根目录执行）：
//
//	go run ./cmd/seed-admin \
//	    -username admin \
//	    -password "Admin123456~" \
//	    -name "超级管理员" \
//	    -email "admin@example.com" \
//	    -config configs/config.yaml
//
// 设计要点：
//   - **复用项目代码**：`utils.Hash` 走 `bcrypt.DefaultCost`，
//     与 service/user.go 注册路径完全一致，避免「seed 工具 hash ≠ 运行时 hash」事故。
//   - **复用配置**：`config.New().Load(configs/config.yaml)` 走项目同一套 viper。
//   - **不引入 wire**：seed 工具只关心「开 DB + 写一条 user」，全量 wire 装配过重。
//   - **幂等性**：用 username 查重，存在则更新 password / email / name，不存在则创建。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/gorm"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
	"github.com/cuiyuanxin/roc_way/internal/pkg/database"
	"github.com/cuiyuanxin/roc_way/internal/pkg/utils"
)

func main() {
	// 1. CLI 参数
	var (
		username   = flag.String("username", "admin", "用户名（5-24 位字母数字下划线短横线）")
		password   = flag.String("password", "", "明文密码（必须，会被 bcrypt 哈希）")
		name       = flag.String("name", "超级管理员", "昵称")
		email      = flag.String("email", "admin@example.com", "邮箱（可空）")
		configPath = flag.String("config", "configs/config.yaml", "配置文件路径")
	)
	flag.Parse()

	if *username == "" || *password == "" {
		log.Fatal("❌ -username 和 -password 都必须提供")
	}

	// 2. 加载配置
	cfgMgr := config.New()
	if err := cfgMgr.Load(*configPath); err != nil {
		log.Fatalf("❌ 加载配置 %s 失败: %v", *configPath, err)
	}
	cfg := cfgMgr.Current()

	// 3. 打开数据库（10s 超时，gormLog=nil 走默认 logger）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.Open(ctx, cfg.Database, nil)
	if err != nil {
		log.Fatalf("❌ 连接数据库失败: %v", err)
	}
	defer func() {
		if cerr := db.Close(); cerr != nil {
			log.Printf("⚠️ 关闭数据库连接: %v", cerr)
		}
	}()

	// 4. bcrypt 哈希（与运行时注册路径完全一致：bcrypt.DefaultCost）
	hash, err := utils.Hash(*password)
	if err != nil {
		log.Fatalf("❌ 密码哈希失败: %v", err)
	}
	fmt.Printf("🔑 username:   %s\n", *username)
	fmt.Printf("🔑 bcrypt:     %s\n", hash[:29]+"...") // 不全打印，避免日志泄漏完整 hash

	// 5. 幂等写入：username 查重 + Create or Update
	now := time.Now().Unix()
	if err := upsertAdmin(ctx, db.Write, *username, *email, *name, hash, now); err != nil {
		log.Fatalf("❌ 写入失败: %v", err)
	}

	fmt.Println("✅ admin 账号写入成功（id 与 password 已就绪，可用 -username + -password 登录）")
	os.Exit(0)
}

// upsertAdmin 按 username 查重，存在则更新 password / email / name / updated_at，
// 不存在则创建。BeforeCreate / BeforeUpdate hook 会自动填 int64 时间戳。
func upsertAdmin(ctx context.Context, gdb *gorm.DB, username, email, name, hash string, now int64) error {
	var existing model.User
	err := gdb.WithContext(ctx).Where("username = ?", username).First(&existing).Error
	if err == nil {
		// 已存在 → 更新
		updates := map[string]any{
			"password":   hash,
			"email":      email,
			"name":       name,
			"updated_at": now,
		}
		if err := gdb.WithContext(ctx).
			Model(&model.User{}).
			Where("username = ?", username).
			Updates(updates).Error; err != nil {
			return fmt.Errorf("update: %w", err)
		}
		fmt.Printf("📝 已存在 id=%d，更新 password / email / name / updated_at\n", existing.ID)
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query: %w", err)
	}

	// 不存在 → 创建
	u := &model.User{
		Username: username,
		Email:    email,
		Name:     name,
		Password: hash,
	}
	if err := gdb.WithContext(ctx).Create(u).Error; err != nil {
		return fmt.Errorf("create: %w", err)
	}
	fmt.Printf("📝 新建成功 id=%d\n", u.ID)
	return nil
}
