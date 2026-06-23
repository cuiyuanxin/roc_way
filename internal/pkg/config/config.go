// Package config 提供基于 viper 的配置加载、热更新与环境变量覆盖。
//
// 强制使用 viper，**不**自实现 fsnotify 监听。
package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Config 聚合全部子配置。
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Cache    CacheConfig    `mapstructure:"cache"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Logger   LoggerConfig   `mapstructure:"logger"`
}

// ServerConfig HTTP 服务配置。
type ServerConfig struct {
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
type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
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
func (d DSNConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		d.User, d.Password, d.Host, d.Port, d.DBName)
}

// CacheConfig Redis 配置。
type CacheConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	Prefix   string `mapstructure:"prefix"`
}

// AuthConfig 认证配置。
type AuthConfig struct {
	JWTSecret     string `mapstructure:"jwt_secret"`
	AccessTTLSec  int    `mapstructure:"access_ttl_sec"`
	RefreshTTLSec int    `mapstructure:"refresh_ttl_sec"`
	ModelPath     string `mapstructure:"model_path"`
	PolicyPath    string `mapstructure:"policy_path"`
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
	Level  string `mapstructure:"level"`
	Dir    string `mapstructure:"dir"`
	MaxMB  int    `mapstructure:"max_mb"`
	Backup int    `mapstructure:"backup"`
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
