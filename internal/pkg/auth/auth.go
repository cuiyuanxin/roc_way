// Package auth 提供 JWT 签发 / 验签 / 黑名单能力。
//
// 黑名单通过缓存（Redis）实现：Revoke 时写入 jti -> 1，TTL = token 剩余有效期。
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
)

// insecureJWTSecrets 不允许在 release 模式使用的默认 / 弱 secret。
//
// 防止运维忘记改默认值导致「任意 token 可伪造」的安全事故。
var insecureJWTSecrets = map[string]struct{}{
	"":                          {},
	"change-me":                 {},
	"change-me-in-production":   {},
	"secret":                    {},
	"jwt-secret":                {},
	"please-change-me":          {},
	"rocway-default-jwt-secret": {},
}

// minJWTSecretLen JWT secret 最小长度（字节）。
//
// HS256 推荐 ≥ 32 字节（256 bit），与 HMAC-SHA256 内部块大小一致。
const minJWTSecretLen = 32

// New 创建 JWT 管理器。
//
// 修复 [C3]：启动期校验 secret 强度，release 模式拒绝不安全 secret。
// 强制：HS256 secret 长度 ≥ 32 字节，且不能在 insecureJWTSecrets 黑名单中。
func New(cfg config.AuthConfig, c *cache.Client) *Auth {
	validateJWTSecret(cfg.JWTSecret)

	ttl := time.Duration(cfg.AccessTTLSec) * time.Second
	if ttl == 0 {
		ttl = time.Hour
	}
	refreshTTL := time.Duration(cfg.RefreshTTLSec) * time.Second
	if refreshTTL == 0 {
		refreshTTL = 7 * 24 * time.Hour
	}
	return &Auth{
		secret:     []byte(cfg.JWTSecret),
		accessTTL:  ttl,
		refreshTTL: refreshTTL,
		issuer:     cfg.Issuer,
		cache:      c,
	}
}

// validateJWTSecret 校验 JWT secret 强度。release 模式发现不安全 secret 直接 panic。
func validateJWTSecret(secret string) {
	if _, bad := insecureJWTSecrets[strings.TrimSpace(strings.ToLower(secret))]; bad {
		// 调试 / 测试模式仅打 warn，避免本地开发卡死；
		// 生产环境（release mode）必须 panic。
		if gin.Mode() == gin.ReleaseMode {
			panic("auth: JWT secret is a known-insecure default; set ROCWAY_AUTH_JWT_SECRET env to a strong value (>= 32 bytes)")
		}
		_, _ = fmt.Fprintf(os.Stderr, "WARN: auth: JWT secret is a known-insecure default; set ROCWAY_AUTH_JWT_SECRET env to a strong value (>= 32 bytes)\n")
		return
	}
	if len(secret) < minJWTSecretLen {
		if gin.Mode() == gin.ReleaseMode {
			panic(fmt.Sprintf("auth: JWT secret must be >= %d bytes (got %d); generate with: openssl rand -base64 48", minJWTSecretLen, len(secret)))
		}
		_, _ = fmt.Fprintf(os.Stderr, "WARN: auth: JWT secret length %d < %d; recommend setting ROCWAY_AUTH_JWT_SECRET\n", len(secret), minJWTSecretLen)
	}
}

// Claims 自定义 JWT 声明。
type Claims struct {
	Kind string `json:"kind"` // "access" | "refresh"
	jwt.RegisteredClaims
}

// TokenPair 包含 access 与 refresh 两个 token。
type TokenPair struct {
	Access     string `json:"access"`
	Refresh    string `json:"refresh"`
	AccessExp  int64  `json:"access_exp"`
	RefreshExp int64  `json:"refresh_exp"`
}

// Auth JWT 管理器。
type Auth struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
	cache      *cache.Client
}

// Issue 签发一对 token。
func (a *Auth) Issue(subject string) (*TokenPair, error) {
	access, accessExp, err := a.sign(subject, "access", a.accessTTL)
	if err != nil {
		return nil, err
	}
	refresh, refreshExp, err := a.sign(subject, "refresh", a.refreshTTL)
	if err != nil {
		return nil, err
	}
	return &TokenPair{Access: access, Refresh: refresh, AccessExp: accessExp.Unix(), RefreshExp: refreshExp.Unix()}, nil
}

// Refresh 用 refresh token 换一对新的 access + refresh（rotation 机制）。
//
// 修复 [C4]：原 handler.refresh 仅回显入参；现实现真校验：
//  1. 验签 + 解析 refresh token；
//  2. 断言 claims.Kind == "refresh"（拒绝 access 错用）；
//  3. 检查 jti 是否在黑名单；
//  4. 旧 refresh jti 写入黑名单（防止重放）；
//  5. 签发新一对 token。
func (a *Auth) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := a.Parse(refreshToken)
	if err != nil {
		return nil, errcode.New(errcode.ErrTokenInvalid, err)
	}
	if claims.Kind != "refresh" {
		return nil, errcode.ErrTokenInvalid.WithMessage("not a refresh token")
	}
	// 黑名单检查（防止被吊销的 refresh 继续换 token）
	revoked, _ := a.IsRevoked(ctx, claims.ID)
	if revoked {
		return nil, errcode.ErrTokenInvalid.WithMessage("refresh token revoked")
	}
	// rotation：旧 refresh jti 加入黑名单，TTL = 剩余有效期
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl > 0 {
		_ = a.Revoke(ctx, claims.ID, ttl)
	}
	return a.Issue(claims.Subject)
}

func (a *Auth) sign(subject, kind string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(ttl)
	jti, err := randHex(16)
	if err != nil {
		return "", time.Time{}, err
	}
	claims := Claims{
		Kind: kind,
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
func (a *Auth) Parse(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(_ *jwt.Token) (any, error) {
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
