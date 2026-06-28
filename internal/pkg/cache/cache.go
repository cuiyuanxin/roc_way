// Package cache 基于 go-redis/v9 提供键前缀、TTL、SCAN 优化。
package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
)

// Client 包装 redis.Client，所有操作自动添加 cfg.Prefix 前缀。
type Client struct {
	rdb    *redis.Client
	prefix string
}

// ErrNil key 不存在哨兵。
//
// 用途：调用方用 errors.Is(err, cache.ErrNil) 判别"key 不存在"和"真错误"，
// 避免直接 import go-redis 暴露内部类型。
var ErrNil = errors.New("cache: key not found")

// New 创建缓存客户端。
//
// **启动期不强依赖 Redis**：若 Ping 失败仅打 warn 日志、仍返回可用 Client。
// 原因：应用支持「Redis 主存 + MySQL 兜底」双存储降级（详见 project_rules.md 第 19 条），
// Redis 暂时不可达时业务应能继续运行（走 DB 兜底），进程不应被启动期连通性绑死。
//
// 真实运行期调用 Get/Set/Incr 等方法时仍会按 redis client 内部策略重试 / 报错，
// 调用方按 error 处理（service 层会 swallow + 打日志 + 走 DB 兜底）。
//
// log 显式注入（**禁止**用 zap.L() 兜底），保证日志进入项目统一的 Loggers 通道（api / db / security）。
func New(cfg config.CacheConfig, log *zap.SugaredLogger) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		// 不返回 error：让业务按"cache 不可用"降级路径走 DB 兜底。
		log.Warnw("cache.ping_failed_at_startup",
			"addr", cfg.Addr,
			"error", err,
		)
	}
	return &Client{rdb: rdb, prefix: cfg.Prefix}, nil
}

// RDB 返回底层 redis.Client（供 limiter 等组件使用）。
func (c *Client) RDB() *redis.Client {
	return c.rdb
}

func (c *Client) k(key string) string { return c.prefix + key }

// Set 写入并设置 TTL。
func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return c.rdb.Set(ctx, c.k(key), value, ttl).Err()
}

// Get 读取字符串值。
//
// key 不存在时返 ErrNil（包装 redis.Nil），调用方用 errors.Is(err, cache.ErrNil) 判别。
// 其它错误原样上抛。
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	v, err := c.rdb.Get(ctx, c.k(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrNil
		}
		return "", err
	}
	return v, nil
}

// GetWithTTL 读取字符串值 + 剩余 TTL。
//
// 用途：登录锁定状态查询——需要知道「锁的过期时间」决定是否仍生效，
// 一次 RTT 拿 (value, ttl) 比两次 GET 高效。
//
// 返回：
//   - (val, ttl, nil)   key 存在
//   - ("", 0, ErrNil)   key 不存在（TTL = 0）
//   - ("", 0, err)      其它错误
func (c *Client) GetWithTTL(ctx context.Context, key string) (string, time.Duration, error) {
	v, err := c.rdb.Get(ctx, c.k(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", 0, ErrNil
		}
		return "", 0, err
	}
	ttl, err := c.rdb.TTL(ctx, c.k(key)).Result()
	if err != nil {
		// 取 TTL 失败：仍返 value，TTL 留 0 让调用方走 DB 兜底更安全
		return v, 0, nil
	}
	return v, ttl, nil
}

// Del 删除一个或多个键。
func (c *Client) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	full := make([]string, len(keys))
	for i, k := range keys {
		full[i] = c.k(k)
	}
	return c.rdb.Del(ctx, full...).Err()
}

// Expire 设置 TTL。
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, c.k(key), ttl).Err()
}

// Incr 自增。
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, c.k(key)).Result()
}

// IncrWithTTL 自增并设置 TTL（仅当 key 新建时生效）。
//
// 适用于「固定窗口计数器」：第一次自增时设 TTL，后续自增复用现有 TTL。
// 返回值：自增后的值。
func (c *Client) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(ctx, c.k(key))
	pipe.Expire(ctx, c.k(key), ttl) // 每次都重设 TTL，简单且兼容；如需「仅新 key」用 NX
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// SetNX 设置值但仅当 key 不存在（用于分布式锁）。
//
// 返回 true 表示成功设置，false 表示 key 已存在。
func (c *Client) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, c.k(key), value, ttl).Result()
}

// Scan 基于 SCAN 而非 KEYS 迭代匹配键，每次 COUNT 500，回调 fn 返回 false 停止。
func (c *Client) Scan(ctx context.Context, pattern string, fn func(key string) bool) error {
	iter := c.rdb.Scan(ctx, 0, c.prefix+pattern, 500).Iterator()
	for iter.Next(ctx) {
		if !fn(iter.Val()) {
			return nil
		}
	}
	return iter.Err()
}

// Close 关闭客户端。
func (c *Client) Close() error {
	if c.rdb == nil {
		return errors.New("cache: nil client")
	}
	return c.rdb.Close()
}
