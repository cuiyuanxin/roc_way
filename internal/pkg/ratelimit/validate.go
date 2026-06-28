package ratelimit

import (
	"fmt"

	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
)

// validDrivers driver 合法取值集合。
//
// 其它任何取值都属于配置错误，启动期必须 panic，
// 防止 typo（如 "Redis" / "REDIS" / "mem"）被 middleware 静默 fallback 为 memory，
// 进而导致分布式部署限流失效。
var validDrivers = map[string]struct{}{
	"memory": {},
	"redis":  {},
}

// Validate 校验限流配置合法性（启动期一次性校验所有维度）。
//
// 检查项：
//  1. driver 必填且必须在 {memory, redis} 集合内；
//  2. 选 redis 时必须注入 cache 客户端；
//  3. cluster 模式下 driver 必须是 redis（memory 在多实例下计数器不共享，限流形同虚设）；
//  4. deploy_mode 取值必须在 {single, cluster} 集合内（空串按 single 处理）。
//
// deployMode 与 driver 的关系：
//   - single（默认）: driver 任意（单机也能用 redis）
//   - cluster       : driver 必须为 redis
//
// 失败返回 error；调用方按需 panic。
func Validate(cfg config.RateLimitConfig, cache *cache.Client, deployMode string) error {
	// driver 合法性 + cache 注入
	if cfg.Driver == "" {
		return fmt.Errorf("ratelimit: driver is required (memory | redis)")
	}
	if _, ok := validDrivers[cfg.Driver]; !ok {
		return fmt.Errorf("ratelimit: invalid driver %q (allowed: memory | redis)", cfg.Driver)
	}
	if cfg.Driver == "redis" && cache == nil {
		return fmt.Errorf("ratelimit: driver=redis requires cache client")
	}

	// 部署模式约束
	mode := deployMode
	if mode == "" {
		mode = "single"
	}
	switch mode {
	case "single":
		return nil
	case "cluster":
		if cfg.Driver != "redis" {
			return fmt.Errorf(
				"ratelimit: deploy_mode=cluster requires driver=redis (got %q); "+
					"memory driver 在多实例下计数器不共享，限流形同虚设",
				cfg.Driver,
			)
		}
		return nil
	default:
		return fmt.Errorf(
			"ratelimit: invalid deploy_mode %q (allowed: single | cluster)",
			deployMode,
		)
	}
}
