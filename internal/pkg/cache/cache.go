// Package cache 基于 go-redis/v9 提供键前缀、TTL、SCAN 优化。
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
)

// Client 包装 redis.Client，所有操作自动添加 cfg.Prefix 前缀。
type Client struct {
	rdb    *redis.Client
	prefix string
}

// New 创建缓存客户端。
func New(cfg config.CacheConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("cache: ping: %w", err)
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
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, c.k(key)).Result()
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
