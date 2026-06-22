// Package logger 基于 Zap + Lumberjack 提供结构化日志与文件轮转。
package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Loggers 持有 API 与 DB 两个独立 SugaredLogger。
type Loggers struct {
	api *zap.SugaredLogger
	db  *zap.SugaredLogger
}

// API 返回 API 访问日志 logger。
func (l *Loggers) API() *zap.SugaredLogger { return l.api }

// DB 返回数据库日志 logger。
func (l *Loggers) DB() *zap.SugaredLogger { return l.db }

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

	api := zap.New(apiCore, zap.AddCaller()).Sugar()
	db := zap.New(dbCore, zap.AddCaller()).Sugar()
	return &Loggers{api: api, db: db}, nil
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
