// Package database 基于 GORM 提供 MySQL 访问、连接重试、读写分离与自动迁移。
package database

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
)

// DB 包装主从 GORM.DB，提供 RW 路由。
type DB struct {
	Write *gorm.DB
	Read  []*gorm.DB
}

// Open 连接主从库，写失败时按指数退避重试最多 3 次。
func Open(ctx context.Context, cfg config.DatabaseConfig) (*DB, error) {
	write, err := openWithRetry(cfg.Write.DSN(), cfg, 3)
	if err != nil {
		return nil, fmt.Errorf("database: connect write: %w", err)
	}
	var reads []*gorm.DB
	for _, r := range cfg.Read {
		if r.Host == "" {
			continue
		}
		db, err := gorm.Open(mysql.Open(r.DSN()), newGormCfg(cfg))
		if err != nil {
			return nil, fmt.Errorf("database: connect read %s: %w", r.Host, err)
		}
		applyPool(db, cfg)
		reads = append(reads, db)
	}
	return &DB{Write: write, Read: reads}, nil
}

// openWithRetry 指数退避重试 open。
func openWithRetry(dsn string, cfg config.DatabaseConfig, retries int) (*gorm.DB, error) {
	var lastErr error
	delay := time.Second
	for i := 0; i < retries; i++ {
		db, err := gorm.Open(mysql.Open(dsn), newGormCfg(cfg))
		if err == nil {
			if cerr := applyPool(db, cfg); cerr == nil {
				return db, nil
			} else {
				lastErr = cerr
			}
		} else {
			lastErr = err
		}
		select {
		case <-time.After(delay):
		}
		delay *= 2
	}
	return nil, lastErr
}

func newGormCfg(cfg config.DatabaseConfig) *gorm.Config {
	return &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}
}

func applyPool(db *gorm.DB, cfg config.DatabaseConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	sqlDB.SetConnMaxLifetime(time.Hour)
	return sqlDB.Ping()
}

// AutoMigrate 在主库上执行表结构迁移。
func (d *DB) AutoMigrate(models ...any) error {
	return d.Write.AutoMigrate(models...)
}

// RO 返回一个走从库的 GORM.DB。若无读节点则降级到主库；事务内调用方应直接用 Write。
func (d *DB) RO() *gorm.DB {
	if len(d.Read) == 0 {
		return d.Write
	}
	return d.Read[rand.Intn(len(d.Read))]
}

// Close 关闭所有连接。
func (d *DB) Close() error {
	var errs []error
	if d.Write != nil {
		if sqlDB, err := d.Write.DB(); err == nil {
			errs = append(errs, sqlDB.Close())
		}
	}
	for _, r := range d.Read {
		if sqlDB, err := r.DB(); err == nil {
			errs = append(errs, sqlDB.Close())
		}
	}
	return errors.Join(errs...)
}
