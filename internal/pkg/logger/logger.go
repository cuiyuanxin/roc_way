// Package logger 基于 Zap + Lumberjack 提供结构化日志与文件轮转。
package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Loggers 持有 API / DB / Security 三个独立 SugaredLogger。
type Loggers struct {
	api      *zap.SugaredLogger
	db       *zap.SugaredLogger
	security *zap.SugaredLogger
}

// API 返回 API 访问日志 logger。
func (l *Loggers) API() *zap.SugaredLogger { return l.api }

// DB 返回数据库日志 logger。
func (l *Loggers) DB() *zap.SugaredLogger { return l.db }

// Security 返回安全事件 logger。
//
// 专用于账号锁定、登录失败等安全敏感事件的记录，
// 运维可按 channel 过滤（logs/security.log）。
func (l *Loggers) Security() *zap.SugaredLogger { return l.security }

// Sync 刷盘所有 logger 缓冲（zap.Sync）。
//
// 应在进程退出前调用（如 defer），保证应用退出前缓冲的日志不丢失。
// 多 logger 全部关闭，任一失败合并到 errors 返回。
func (l *Loggers) Sync() error {
	var errs []error
	for _, lg := range []*zap.SugaredLogger{l.api, l.db, l.security} {
		if lg == nil {
			continue
		}
		if err := lg.Sync(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Config 日志配置。
type Config struct {
	Level  string // "debug" | "info" | "warn" | "error"
	Dir    string // 日志目录
	MaxMB  int    // 单文件最大 MB
	Backup int    // 保留份数
}

// New 创建 Loggers。level/dir/maxMB/backup 任意字段为 0 时取合理默认。
func New(cfg Config) (*Loggers, error) {
	if cfg.Dir == "" {
		cfg.Dir = "logs"
	}
	if cfg.MaxMB == 0 {
		cfg.MaxMB = 100
	}
	if cfg.Backup == 0 {
		cfg.Backup = 7
	}
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("logger: mkdir %s: %w", cfg.Dir, err)
	}

	level := zapcore.InfoLevel
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(encoderCfg)

	apiCore := zapcore.NewCore(encoder, newWriter(filepath.Join(cfg.Dir, "api.log"), cfg), level)
	dbCore := zapcore.NewCore(encoder, newWriter(filepath.Join(cfg.Dir, "db.log"), cfg), level)
	securityCore := zapcore.NewCore(encoder, newWriter(filepath.Join(cfg.Dir, "security.log"), cfg), level)

	api := zap.New(apiCore, zap.AddCaller()).Sugar()
	db := zap.New(dbCore, zap.AddCaller()).Sugar()
	security := zap.New(securityCore, zap.AddCaller()).Sugar()
	return &Loggers{api: api, db: db, security: security}, nil
}

func newWriter(path string, cfg Config) zapcore.WriteSyncer {
	lj := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    cfg.MaxMB,
		MaxBackups: cfg.Backup,
		MaxAge:     30,
		Compress:   true,
	}
	return zapcore.AddSync(lj)
}
