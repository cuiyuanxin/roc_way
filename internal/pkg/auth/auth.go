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
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
)

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
	cache      *cache.Client
}

// New 创建 JWT 管理器。
func New(cfg config.AuthConfig, c *cache.Client) *Auth {
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
		cache:      c,
	}
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
