// Package logger 基于 Zap + Lumberjack 提供结构化日志与文件轮转。
//
// 设计要点：
//   - 每个 channel（api / db / security）独立文件、独立轮转，运维可按文件过滤。
//   - GORM 通过本包的 [NewGormLogger] 接入，db channel 接收 GORM 错误 / 慢查询。
//   - 所有构造逻辑集中在 [New]；新增 channel 只需在 [channelSpecs] 加一行，
//     [New] 与 [Loggers.Sync] 自动跟随，不再有「两处改漏改一处」的隐患。
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

// 预置 channel 名常量（外部代码按名引用，避免散落硬编码字符串）。
const (
	ChannelAPI      = "api"
	ChannelDB       = "db"
	ChannelSecurity = "security"
)

// channelSpec 描述一个日志通道的构建期元数据。
//
// 字段：
//   - Name: 唯一 key，供 [Loggers.API] 等类型安全方法按名查找；
//   - File: 写入的文件名（位于 [Config.Dir] 下）；
//   - CallerSkip: zap.AddCallerSkip 的层数 —— 0 = 默认栈，>0 = 跳过 N 层 caller。
//
// CallerSkip 选取原则：让 caller 指向「调用方业务代码」而非「日志框架自身」。
// db channel 走 GORM hook，GORM 内部经 1 层 logger.Trace 包装、1 层 finisher、
// 1 层 callback，共 ~3 层框架栈，所以设 3。其它 channel（api / security）由
// 业务代码直接调用 zap.SugaredLogger，caller 即调用方，Skip=0 即可。
type channelSpec struct {
	Name       string
	File       string
	CallerSkip int
	sugared    *zap.SugaredLogger
}

// channelSpecs 预置通道表。新增通道时**只需在此处加一行**，
// [New] 构造与 [Loggers.Sync] 刷盘均自动覆盖。
var channelSpecs = []channelSpec{
	{Name: ChannelAPI, File: "api.log"},
	{Name: ChannelDB, File: "db.log", CallerSkip: 3},
	{Name: ChannelSecurity, File: "security.log"},
}

// Loggers 持有预置 channel 的 SugaredLogger 集合。
//
// 通过类型安全方法 [API] / [DB] / [Security] 获取；通过 [Sync] 一次性刷盘全部。
// 字段为包私有，避免外部代码绕过 channel 直接拿到 logger 引用（统一通过 getter）。
type Loggers struct {
	channels map[string]*channelSpec
}

// API 返回 API 访问日志 logger。
func (l *Loggers) API() *zap.SugaredLogger { return l.find(ChannelAPI) }

// DB 返回数据库日志 logger。
//
// 由 [NewGormLogger] 写入：GORM 查询错误 / 慢查询 / 慢迁移等。
// caller 由 channelSpec.CallerSkip 调整到业务 repository 层。
func (l *Loggers) DB() *zap.SugaredLogger { return l.find(ChannelDB) }

// Security 返回安全事件 logger。
//
// 专用于账号锁定、登录失败等安全敏感事件的记录，
// 运维可按 channel 过滤（logs/security.log）。
func (l *Loggers) Security() *zap.SugaredLogger { return l.find(ChannelSecurity) }

// find 按 channel 名查找 sugared logger。channel 名是构建期常量，未命中只可能
// 是 [channelSpecs] 与 getter 之间的拼写漂移，返回 nil 即可让 zap 内部 noop。
func (l *Loggers) find(name string) *zap.SugaredLogger {
	if c, ok := l.channels[name]; ok {
		return c.sugared
	}
	return nil
}

// Sync 刷盘所有 logger 缓冲（zap.Sync）。
//
// 应在进程退出前调用（如 defer），保证应用退出前缓冲的日志不丢失。
// 多 logger 全部关闭，任一失败合并到 errors 返回。
func (l *Loggers) Sync() error {
	var errs []error
	for _, c := range l.channels {
		if c.sugared == nil {
			continue
		}
		if err := c.sugared.Sync(); err != nil {
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

	channels := make(map[string]*channelSpec, len(channelSpecs))
	for i := range channelSpecs {
		spec := &channelSpecs[i]
		core := zapcore.NewCore(encoder, newWriter(filepath.Join(cfg.Dir, spec.File), cfg), level)
		opts := []zap.Option{zap.AddCaller()}
		if spec.CallerSkip > 0 {
			opts = append(opts, zap.AddCallerSkip(spec.CallerSkip))
		}
		spec.sugared = zap.New(core, opts...).Sugar()
		channels[spec.Name] = spec
	}
	return &Loggers{channels: channels}, nil
}

// newWriter 返回带 lumberjack 切片轮转的 WriteSyncer。
//
// 轮转策略：单文件 cfg.MaxMB MB → 自动滚动到 .1 / .2 ...
// 保留 cfg.Backup 份；30 天后压缩归档。Compress 打开节省磁盘。
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
