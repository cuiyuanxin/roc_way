// Package config 提供基于 viper 的配置加载、热更新与环境变量覆盖。
//
// 强制使用 viper，**不**自实现 fsnotify 监听。
package config

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Config 聚合全部子配置。
type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Database    DatabaseConfig    `mapstructure:"database"`
	Cache       CacheConfig       `mapstructure:"cache"`
	Auth        AuthConfig        `mapstructure:"auth"`
	Storage     StorageConfig     `mapstructure:"storage"`
	Logger      LoggerConfig      `mapstructure:"logger"`
	LoginPolicy LoginPolicyConfig `mapstructure:"login_policy"`
}

// DeployMode 部署模式。
const (
	DeploySingle  = "single"  // 单实例部署（默认；driver 任意）
	DeployCluster = "cluster" // 多实例部署；driver 必须为 redis
)

// ServerConfig HTTP 服务配置。
type ServerConfig struct {
	DeployMode        string          `mapstructure:"deploy_mode"` // 部署模式：single（默认） | cluster
	Addr              string          `mapstructure:"addr"`
	Mode              string          `mapstructure:"mode"`
	ReadHeaderTimeout int             `mapstructure:"read_header_timeout"` // 秒
	Timeout           int             `mapstructure:"timeout"`             // 请求超时时间（秒），0 表示不启用
	TLS               TLSConfig       `mapstructure:"tls"`
	TrustedProxies    []string        `mapstructure:"trusted_proxies"` // 信任的代理 IP 列表，为空则信任所有
	CORS              CORSConfig      `mapstructure:"cors"`
	RateLimit         RateLimitConfig `mapstructure:"rate_limit"`
}

// TLSConfig HTTPS/TLS 配置。
//
// 设计原则：「HTTP 不强制，HTTPS 优先」：
//   - HTTP 永远由 server.addr 启动（向后兼容内网/调试）
//   - HTTPS 仅在 Enabled=true 时启动，监听独立端口 Addr
//   - HTTP 请求不带 HSTS 头（避免误导浏览器），HTTPS 自动带
//   - HSTS 中间件在 app.go 自动按 c.Request.TLS 判断，无需手动配置
type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`   // 是否启动 HTTPS server（false 时只跑 HTTP）
	Addr     string `mapstructure:"addr"`      // HTTPS 监听地址，例 ":8443"（与 HTTP 端口独立）
	CertFile string `mapstructure:"cert_file"` // PEM 证书路径
	KeyFile  string `mapstructure:"key_file"`  // PEM 私钥路径
}

// CORSConfig CORS 配置（无默认值，按需配置）。
type CORSConfig struct {
	Origins          []string `mapstructure:"origins"`           // 允许的来源列表，为空默认 "*"
	Methods          []string `mapstructure:"methods"`           // 允许的方法
	Headers          []string `mapstructure:"headers"`           // 允许的请求头
	ExposeHeaders    []string `mapstructure:"expose_headers"`    // 允许客户端访问的响应头
	MaxAge           int      `mapstructure:"max_age"`           // 预检请求缓存时间（秒）
	AllowCredentials bool     `mapstructure:"allow_credentials"` // 是否允许携带凭证
}

// RateLimitConfig 限流配置。
//
// driver 与部署模式 (Server.DeployMode) 的关系：
//   - DeploySingle  + driver=memory | redis：均允许
//   - DeployCluster + driver=redis     ：允许
//   - DeployCluster + driver=memory    ：启动 panic（多实例下 memory 计数器不共享，
//     限流形同虚设，配置层强制防御）
type RateLimitConfig struct {
	Enabled   bool    `mapstructure:"enabled"`    // 是否启用限流
	Driver    string  `mapstructure:"driver"`     // 驱动类型：memory（单机）或 redis（分布式）
	RPS       float64 `mapstructure:"rps"`        // 每秒请求数
	Burst     int     `mapstructure:"burst"`      // 突发容量
	KeyPrefix string  `mapstructure:"key_prefix"` // Redis key 前缀（仅 redis 模式）
}

// DatabaseConfig MySQL 配置。
type DatabaseConfig struct {
	Write        DSNConfig   `mapstructure:"write"`
	Read         []DSNConfig `mapstructure:"read"`
	MaxOpenConns int         `mapstructure:"max_open_conns"`
	MaxIdleConns int         `mapstructure:"max_idle_conns"`
}

// DSNConfig 单节点配置。
type DSNConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

// DSN 返回 GORM/MySQL 可直接使用的 DSN 字符串。
//
// 启动期校验：Host / User / DBName 任一为空直接 panic（不允许默认值启动 DB）。
// Port 默认 3306（MySQL 标准端口）。
func (d DSNConfig) DSN() string {
	if d.Host == "" {
		panic("config: Database.Write.Host is required")
	}
	if d.User == "" {
		panic("config: Database.Write.User is required")
	}
	if d.DBName == "" {
		panic("config: Database.Write.DBName is required")
	}
	port := d.Port
	if port == 0 {
		port = 3306
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		d.User, d.Password, d.Host, port, d.DBName)
}

// CacheConfig Redis 配置。
type CacheConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	Prefix   string `mapstructure:"prefix"`
}

// AuthConfig 认证配置。
//
// 签名算法：HS256（单服务后台脚手架首选；行业标准，与 gin-jwt / go-admin / go-zero 一致）。
//
// JWT secret 三级加载顺序（详见 [auth.New]）：
//  1. 环境变量 JWT_SECRET（生产推荐）
//  2. 本字段 JWTSecret（本地 / 单一节点）
//  3. 自动生成到 configs/.jwt_secret（dev 模式专用，文件保留重启不丢）
//
// ProductionMode 强制：
//   - true 时禁止 dev fallback（必须显式提供 secret，env 或 yaml ），启动失败否则
//   - 这是「小白零配置 + 生产强制」的关键开关
type AuthConfig struct {
	// JWTSecret HS256 签名密钥。
	// 生产环境**推荐**走 env 变量 JWT_SECRET（K8s Secret / Vault 注入），
	// 留空则按 ProductionMode 决定是 dev fallback 还是启动失败。
	JWTSecret string `mapstructure:"jwt_secret"`

	// ProductionMode 生产模式开关。
	// true 时：禁止 dev fallback（必须显式提供 secret，否则启动失败）。
	// false 时：允许自动生成 dev secret 到 configs/.jwt_secret。
	ProductionMode bool `mapstructure:"production_mode"`

	// AccessTTL  / RefreshTTL（秒）。0 = 用默认（2h / 7d）。
	AccessTTLSec  int    `mapstructure:"access_ttl_sec"`
	RefreshTTLSec int    `mapstructure:"refresh_ttl_sec"`
	Issuer        string `mapstructure:"issuer"`

	// RBAC（Casbin）模型 / 策略文件路径。
	ModelPath  string `mapstructure:"model_path"`
	PolicyPath string `mapstructure:"policy_path"`
}

// StorageConfig 文件存储配置。
type StorageConfig struct {
	Driver     string `mapstructure:"driver"`
	LocalDir   string `mapstructure:"local_dir"`
	PublicBase string `mapstructure:"public_base"`
	Endpoint   string `mapstructure:"endpoint"`
	Bucket     string `mapstructure:"bucket"`
	AccessKey  string `mapstructure:"access_key"`
	SecretKey  string `mapstructure:"secret_key"`
}

// LoggerConfig 日志配置。
type LoggerConfig struct {
	Level           string        `mapstructure:"level"`
	Dir             string        `mapstructure:"dir"`
	MaxMB           int           `mapstructure:"max_mb"`
	Backup          int           `mapstructure:"backup"`
	DBEnabled       bool          `mapstructure:"db_enabled"`        // 是否把 GORM 查询错误 / 慢查询写入 db.log（opt-in，默认关闭）
	DBSlowThreshold time.Duration `mapstructure:"db_slow_threshold"` // GORM 慢查询阈值；0 表示不记录慢查询
	DBLogLevel      string        `mapstructure:"db_log_level"`      // GORM 日志级别：silent|error|warn|info；默认 warn
}

// LoginPolicyConfig 登录失败锁定策略。
type LoginPolicyConfig struct {
	ShortThreshold  int           `mapstructure:"short_threshold"`  // 短期锁定阈值（连续失败次数）
	ShortDuration   time.Duration `mapstructure:"short_duration"`   // 短期锁定持续时间
	LongThreshold   int           `mapstructure:"long_threshold"`   // 长期锁定阈值
	LongDuration    time.Duration `mapstructure:"long_duration"`    // 长期锁定持续时间
	AuditRetention  time.Duration `mapstructure:"audit_retention"`  // 审计记录保留时间（用于 janitor 清理）
	JanitorInterval time.Duration `mapstructure:"janitor_interval"` // janitor 清理间隔
}

// Manager 持有 viper 实例与当前 Config 快照。
type Manager struct {
	v   *viper.Viper
	mu  sync.RWMutex
	cfg Config
}

// New 构造 Manager 并设置默认值与 ENV 前缀。
func New() *Manager {
	v := viper.New()
	v.SetEnvPrefix("ROCWAY")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	setDefaults(v)
	m := &Manager{v: v}
	_ = m.unmarshal() // 填充默认值快照
	return m
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.read_header_timeout", 10) // 秒
	v.SetDefault("server.tls.enabled", false)      // 默认不启用 HTTPS
	v.SetDefault("database.max_open_conns", 50)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("cache.prefix", "rocway:")
	v.SetDefault("auth.access_ttl_sec", 3600)
	v.SetDefault("auth.refresh_ttl_sec", 604800)
	v.SetDefault("auth.model_path", "configs/rbac_model.conf")
	v.SetDefault("auth.policy_path", "configs/rbac_policy.csv")
	v.SetDefault("storage.driver", "local")
	v.SetDefault("storage.local_dir", "storage")
	v.SetDefault("storage.public_base", "/storage")
	v.SetDefault("logger.level", "info")
	v.SetDefault("logger.dir", "logs")
	v.SetDefault("logger.max_mb", 100)
	v.SetDefault("logger.backup", 7)
	v.SetDefault("login_policy.short_threshold", 5)
	v.SetDefault("login_policy.short_duration", "15m")
	v.SetDefault("login_policy.long_threshold", 10)
	v.SetDefault("login_policy.long_duration", "24h")
	v.SetDefault("login_policy.audit_retention", "24h")
	v.SetDefault("login_policy.janitor_interval", "24h")
}

// Load 从 path 加载配置文件并解析到 Config。
func (m *Manager) Load(path string) error {
	m.v.SetConfigFile(path)
	if err := m.v.ReadInConfig(); err != nil {
		return fmt.Errorf("config: read %s: %w", path, err)
	}
	return m.unmarshal()
}

// Watch 监听配置文件变更，回调中返回最新 Config。
func (m *Manager) Watch(onChange func(Config)) error {
	m.v.WatchConfig()
	m.v.OnConfigChange(func(_ fsnotify.Event) {
		_ = m.unmarshal()
		if onChange != nil {
			m.mu.RLock()
			snap := m.cfg
			m.mu.RUnlock()
			onChange(snap)
		}
	})
	return nil
}

func (m *Manager) unmarshal() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.v.Unmarshal(&m.cfg)
}

// Current 返回当前快照（拷贝）。
func (m *Manager) Current() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}
