// Package auth 提供 JWT 签发 / 验签 / 黑名单能力。
//
// Phase 2：升级 RS256（公私钥分离）
//   - 私钥（RSA PrivateKey）用于签发 access / refresh token，**只**在签发服务持有
//   - 公钥（RSA PublicKey）用于验签，前端 / 其它服务可拿公钥本地验签
//   - 私钥 / 公钥以 PEM 文件形式存在 configs/keys/（不进 git）
//   - 黑名单（Redis）继续用 jti 维度，实现 token 吊销
//   - Claims 加 DeviceID 字段，登录时绑定设备指纹，JWT 中间件校验
package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
)

// 私钥 / 公钥最小长度（bit）。
//
// OWASP 推荐 RSA ≥ 2048 bit；本项目强制 ≥ 2048，< 4096 给出警告（不强阻）。
const minRSAKeyBits = 2048

// New 创建 JWT 管理器（RS256 模式）。
//
// 强制：
//   - cfg.PrivateKeyPath / cfg.PublicKeyPath 必填
//   - 私钥文件存在且权限 ≤ 0644（更严：≤ 0640 警告，> 0644 阻塞）
//   - 私钥长度 ≥ 2048 bit
//   - 公私钥匹配（同一 key pair）
func New(cfg config.AuthConfig, c *cache.Client) (*Auth, error) {
	priv, err := loadRSAPrivateKey(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("auth: load private key: %w", err)
	}
	pub, err := loadRSAPublicKey(cfg.PublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("auth: load public key: %w", err)
	}
	// 公私钥匹配校验：拿公钥的指数 + 模数对比
	if priv.PublicKey.N.Cmp(pub.N) != 0 {
		return nil, errors.New("auth: public key does not match private key")
	}
	if priv.PublicKey.E != pub.E {
		return nil, errors.New("auth: public key exponent mismatch with private key")
	}

	accessTTL := time.Duration(cfg.AccessTTLSec) * time.Second
	if accessTTL == 0 {
		accessTTL = time.Hour
	}
	refreshTTL := time.Duration(cfg.RefreshTTLSec) * time.Second
	if refreshTTL == 0 {
		refreshTTL = 7 * 24 * time.Hour
	}
	return &Auth{
		priv:       priv,
		pub:        pub,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		issuer:     cfg.Issuer,
		cache:      c,
	}, nil
}

// loadRSAPrivateKey 加载并解析 RSA 私钥 PEM 文件。
//
// 支持 PKCS#1（`RSA PRIVATE KEY`）和 PKCS#8（`PRIVATE KEY`）两种格式。
func loadRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	if path == "" {
		return nil, errors.New("private_key_path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("not a valid PEM file")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS#1: %w", err)
		}
		validateRSAKeyBits(priv.N.BitLen(), true)
		return priv, nil
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS#8: %w", err)
		}
		priv, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("PKCS#8 key is not RSA")
		}
		validateRSAKeyBits(priv.N.BitLen(), true)
		return priv, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}
}

// loadRSAPublicKey 加载并解析 RSA 公钥 PEM 文件。
//
// 支持 `PUBLIC KEY`（PKCS#8 格式公钥） 和 `RSA PUBLIC KEY`（PKCS#1）。
func loadRSAPublicKey(path string) (*rsa.PublicKey, error) {
	if path == "" {
		return nil, errors.New("public_key_path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("not a valid PEM file")
	}
	switch block.Type {
	case "PUBLIC KEY":
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKIX: %w", err)
		}
		pub, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("PKIX key is not RSA")
		}
		validateRSAKeyBits(pub.N.BitLen(), false)
		return pub, nil
	case "RSA PUBLIC KEY":
		pub, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS#1: %w", err)
		}
		validateRSAKeyBits(pub.N.BitLen(), false)
		return pub, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}
}

// validateRSAKeyBits RSA 密钥长度校验。
//
// 强制 ≥ minRSAKeyBits (2048)；< 4096 给 warn。
// 私钥（isPrivate=true）违反时阻塞启动；公钥仅 warn（公钥是公开的）。
func validateRSAKeyBits(bits int, isPrivate bool) {
	if bits < minRSAKeyBits {
		msg := fmt.Sprintf("auth: RSA key length %d bits < %d (OWASP minimum)", bits, minRSAKeyBits)
		if isPrivate {
			panic(msg)
		}
		_, _ = fmt.Fprintf(os.Stderr, "WARN: %s\n", msg)
		return
	}
	if bits < 4096 {
		_, _ = fmt.Fprintf(os.Stderr, "WARN: auth: RSA key length %d bits < 4096, recommend regenerating\n", bits)
	}
}

// Claims 自定义 JWT 声明。
type Claims struct {
	Kind     string `json:"kind"`     // "access" | "refresh"
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

// Auth JWT 管理器。
type Auth struct {
	priv       *rsa.PrivateKey
	pub        *rsa.PublicKey
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
	cache      *cache.Client
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
// 1. 验签 + 解析 refresh token；
// 2. 断言 claims.Kind == "refresh"（拒绝 access 错用）；
// 3. 检查 jti 是否在黑名单；
// 4. 旧 refresh jti 写入黑名单（防止重放）；
// 5. 签发新一对 token（继承 device_id）。
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

// sign 用 RS256 签发 token。
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
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := tok.SignedString(a.priv)
	return signed, exp, err
}

// Parse 解析并验签 token（用公钥）。
func (a *Auth) Parse(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		// 强制 RS256（防 alg=none 攻击 + 防 HS256/RS256 错用）
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.pub, nil
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
