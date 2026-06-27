// `rocway-cli migrate` 执行 admin 应用所有持久化对象的 GORM AutoMigrate。
//
// 用法：
//
//	rocway-cli migrate -c configs/config.yaml
//
// 设计：
//   - 不经过 wire.InitApp（避免触发 casbin / cache / auth 等非必要依赖）；
//   - 仅装配 database + logger，跑完即退出；
//   - 迁移列表由 internal/app/admin/model.All() 集中维护，新增表无需改本文件。
package cmd

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	gormlogger "gorm.io/gorm/logger"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
	"github.com/cuiyuanxin/roc_way/internal/pkg/database"
	"github.com/cuiyuanxin/roc_way/internal/pkg/logger"
)

// MigrateCmd 返回 migrate 子命令。
func MigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "AutoMigrate admin 应用的持久化对象",
		Long: "读取指定配置文件，连接 MySQL 写库，执行 model.All() 集中维护的迁移列表。\n" +
			"用法：rocway-cli migrate -c configs/config.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
			configPath := fs.String("c", "configs/config.yaml", "path to config file")
			if err := fs.Parse(cmd.Flags().Args()); err != nil {
				return err
			}
			return runMigrate(*configPath)
		},
	}
}

func runMigrate(configPath string) error {
	// 1. 加载配置
	cfgMgr := config.New()
	if err := cfgMgr.Load(configPath); err != nil {
		return fmt.Errorf("load config %q: %w", configPath, err)
	}
	cfg := cfgMgr.Current()

	// 2. 起 logger（便于看 GORM 输出）
	logs, err := logger.New(logger.Config{
		Level:  cfg.Logger.Level,
		Dir:    cfg.Logger.Dir,
		MaxMB:  cfg.Logger.MaxMB,
		Backup: cfg.Logger.Backup,
	})
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	// 3. 装配 GORM logger（按 cfg.Logger.DBEnabled 决定是否走 db 通道）
	gormLog := logger.NewGormLogger(logs, parseGormLevel(cfg.Logger.DBLogLevel), cfg.Logger.DBSlowThreshold)
	if !cfg.Logger.DBEnabled {
		gormLog = nil
	}

	// 4. 连 MySQL 写库
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg.Database, gormLog)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	// 5. 执行迁移
	entities := model.All()
	fmt.Printf("→ migrate %d entities: ", len(entities))
	for i, e := range entities {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("%T", e)
	}
	fmt.Println()

	if err := model.Migrate(db.Write); err != nil {
		return fmt.Errorf("automigrate: %w", err)
	}

	fmt.Println("✔ migrate done")
	return nil
}

// parseGormLevel 把 yaml 字符串映射为 gorm logger 级别（与 wire 装配保持一致）。
func parseGormLevel(s string) gormlogger.LogLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "silent":
		return gormlogger.Silent
	case "error":
		return gormlogger.Error
	case "info":
		return gormlogger.Info
	default:
		return gormlogger.Warn
	}
}
