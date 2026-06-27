// Package auth 提供 JWT 签发 / 验签 / 黑名单能力。
//
// 签名算法：HS256（行业标准；单服务后台脚手架首选）
//   - 单一 secret（≥ 32 字节随机），签发 + 验签都用它
//   - 不需要 PEM 文件，secret 通过 env / yaml / 自动生成文件三级回退加载
//   - 详见 configs/config.yaml 的 auth 段注释 & docs/production.md
//
// 安全特性：
//   - AccessToken TTL 短（默认 2h）+ RefreshToken TTL 中（默认 7d）
//   - Refresh Token Rotation：每次 refresh 换新一对，旧 refresh 立即进黑名单
//   - 黑名单（Redis）按 jti 维度，可吊销单 token
//   - DeviceID 绑定：登录时把设备指纹写入 claims，中间件校验 X-Device-ID
package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
)

// minSecretBytes JWT secret 最小字节数（OWASP HS256 推荐 ≥ 256 bit = 32 字节）。
const minSecretBytes = 32

// devSecretPath dev 模式自动生成 secret 的默认路径。
const devSecretPath = "configs/.jwt_secret"

// devSecretBytes dev 模式自动生成的随机字节数（48 字节 = 384 bit，HS256 推荐）。
const devSecretBytes = 48

// secretSource secret 来源标识（用于启动横幅审计）。
type secretSource string

const (
	sourceEnv     secretSource = "env:JWT_SECRET"
	sourceConfig  secretSource = "config:auth.jwt_secret"
	sourceDevFile secretSource = "file:configs/.jwt_secret (auto-generated DEV ONLY)"
)

// Auth JWT 管理器（HS256）。
type Auth struct {
	secret     []byte
	alg        jwt.SigningMethod
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
	cache      *cache.Client
}

// New 创建 JWT 管理器（HS256 模式）。
//
// secret 加载顺序（三级回退，前者优先）：
//  1. 环境变量 JWT_SECRET（生产推荐）
//  2. 配置 cfg.JWTSecret（本地 / 单一节点）
//  3. 配置文件 configs/.jwt_secret（不存在则自动生成；dev 模式专用）
//
// 强制：
//   - secret ≥ 32 字节（启动时校验，< 32 直接 panic）
//   - cfg.ProductionMode=true 时禁止 dev 自动生成（必须显式提供 secret）
func New(cfg config.AuthConfig, c *cache.Client, log *zap.SugaredLogger) (*Auth, error) {
	secret, source, err := resolveSecret(cfg)
	if err != nil {
		return nil, err
	}
	if len(secret) < minSecretBytes {
		return nil, fmt.Errorf("auth: jwt secret too short (%d bytes, require ≥ %d bytes); "+
			"generate one with: openssl rand -base64 48", len(secret), minSecretBytes)
	}

	accessTTL := time.Duration(cfg.AccessTTLSec) * time.Second
	if accessTTL == 0 {
		accessTTL = 2 * time.Hour
	}
	refreshTTL := time.Duration(cfg.RefreshTTLSec) * time.Second
	if refreshTTL == 0 {
		refreshTTL = 7 * 24 * time.Hour
	}

	printSecretBanner(log, source, len(secret), cfg.ProductionMode)

	return &Auth{
		secret:     secret,
		alg:        jwt.SigningMethodHS256,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		issuer:     cfg.Issuer,
		cache:      c,
	}, nil
}

// resolveSecret 三级回退解析 secret。
func resolveSecret(cfg config.AuthConfig) ([]byte, secretSource, error) {
	// 1) env 优先（生产推荐；K8s Secret 注入的 secret 走 env）
	if v := os.Getenv("JWT_SECRET"); v != "" {
		return []byte(v), sourceEnv, nil
	}
	// 2) yaml 配置（本地 / 单一节点）
	if cfg.JWTSecret != "" {
		return []byte(cfg.JWTSecret), sourceConfig, nil
	}
	// 3) dev fallback：自动生成 / 读取文件
	if cfg.ProductionMode {
		return nil, "", errors.New("auth: production_mode=true requires explicit jwt_secret; " +
			"set env JWT_SECRET or config:auth.jwt_secret")
	}
	secret, err := loadOrGenerateDevSecret(devSecretPath)
	if err != nil {
		return nil, "", fmt.Errorf("auth: dev secret: %w", err)
	}
	return secret, sourceDevFile, nil
}

// loadOrGenerateDevSecret 读取或生成 dev 模式 secret 文件。
//
// 文件不存在或 secret < 32 字节：生成新 secret 写入文件（mode 0600）。
// 文件已存在且 secret 足够长：直接返回（保证重启后 token 不失效）。
func loadOrGenerateDevSecret(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		secret := bytes.TrimSpace(data)
		if len(secret) >= minSecretBytes {
			return secret, nil
		}
	}

	// 生成新 secret
	buf := make([]byte, devSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("rand: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(buf)

	// 写入文件（确保目录存在 + 收紧权限）
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(path, []byte(encoded), 0o600); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}
	return []byte(encoded), nil
}

// printSecretBanner 启动时打印 secret 来源横幅（不打印 secret 内容）。
func printSecretBanner(log *zap.SugaredLogger, source secretSource, length int, productionMode bool) {
	if log == nil {
		return
	}
	if productionMode {
		log.Infow("jwt.secret_loaded",
			"source", string(source),
			"length_bytes", length,
			"mode", "PRODUCTION",
		)
		return
	}
	// dev 模式：明确警告
	if source == sourceDevFile {
		log.Warnw("⚠️  jwt.secret DEV MODE (auto-generated)",
			"source", string(source),
			"length_bytes", length,
			"warning", "for local dev ONLY; restart will keep same secret (file), but DO NOT use in production",
			"fix", "set env JWT_SECRET or config:auth.jwt_secret + production_mode=true for production",
		)
		return
	}
	log.Infow("jwt.secret_loaded",
		"source", string(source),
		"length_bytes", length,
		"mode", "DEV",
	)
}

// Claims 自定义 JWT 声明。
type Claims struct {
	Kind     string `json:"kind"`      // "access" | "refresh"
	DeviceID string `json:"device_id"` // 设备指纹（登录时绑定，中间件校验）
	jwt.RegisteredClaims
}

// TokenPair 包含 access 与 refresh 两个 token。
type TokenPair struct {
	Access     string `json:"access"`
	Refresh    string `json:"refresh"`
	AccessExp  int64  `json:"access_exp"`
	RefreshExp int64  `json:"refresh_exp"`
}

// Issue 签发一对 token（不绑定 device id，向后兼容）。
func (a *Auth) Issue(subject string) (*TokenPair, error) {
	return a.IssueWithDevice(subject, "")
}

// IssueWithDevice 签发一对 token，并把 deviceID 写入 claims。
//
// deviceID 为空时跳过绑定（适用于后台任务 / 系统调用）；
// 中间件在校验时如果 token claim 里有 deviceID 而请求头没有，会拒绝。
func (a *Auth) IssueWithDevice(subject, deviceID string) (*TokenPair, error) {
	access, accessExp, err := a.sign(subject, "access", a.accessTTL, deviceID)
	if err != nil {
		return nil, err
	}
	refresh, refreshExp, err := a.sign(subject, "refresh", a.refreshTTL, deviceID)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		Access:     access,
		Refresh:    refresh,
		AccessExp:  accessExp.Unix(),
		RefreshExp: refreshExp.Unix(),
	}, nil
}

// Refresh 用 refresh token 换一对新的 access + refresh（rotation 机制）。
//
//  1. 验签 + 解析 refresh token；
//  2. 断言 claims.Kind == "refresh"（拒绝 access 错用）；
//  3. 检查 jti 是否在黑名单；
//  4. 旧 refresh jti 写入黑名单（防止重放）；
//  5. 签发新一对 token（继承 device_id）。
func (a *Auth) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := a.Parse(refreshToken)
	if err != nil {
		return nil, errcode.New(errcode.ErrTokenInvalid, err)
	}
	if claims.Kind != "refresh" {
		return nil, errcode.ErrTokenInvalid.WithMessage("not a refresh token")
	}
	revoked, _ := a.IsRevoked(ctx, claims.ID)
	if revoked {
		return nil, errcode.ErrTokenInvalid.WithMessage("refresh token revoked")
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl > 0 {
		_ = a.Revoke(ctx, claims.ID, ttl)
	}
	// 继承 deviceID
	return a.IssueWithDevice(claims.Subject, claims.DeviceID)
}

// sign 用 HS256 签发 token。
func (a *Auth) sign(subject, kind string, ttl time.Duration, deviceID string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(ttl)
	jti, err := randHex(16)
	if err != nil {
		return "", time.Time{}, err
	}
	claims := Claims{
		Kind:     kind,
		DeviceID: deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    a.issuer,
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        jti,
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(a.secret)
	return signed, exp, err
}

// Parse 解析并验签 token。
//
// 强制 HS256（防 alg=none 攻击 + 防 RS256/HS256 错用）。
func (a *Auth) Parse(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("auth: parse: %w", err)
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("auth: invalid claims")
	}
	return claims, nil
}

// Revoke 将 jti 写入黑名单，TTL = token 剩余有效期。
func (a *Auth) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	if a.cache == nil {
		return errors.New("auth: cache not configured")
	}
	return a.cache.Set(ctx, blacklistKey(jti), "1", ttl)
}

// IsRevoked 查询黑名单。
func (a *Auth) IsRevoked(ctx context.Context, jti string) (bool, error) {
	if a.cache == nil {
		return false, nil
	}
	_, err := a.cache.Get(ctx, blacklistKey(jti))
	if err == nil {
		return true, nil
	}
	return false, nil
}

func blacklistKey(jti string) string { return "auth:blacklist:" + jti }

func randHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
