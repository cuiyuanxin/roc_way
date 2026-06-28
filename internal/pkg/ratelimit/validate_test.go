package ratelimit

import (
	"strings"
	"testing"

	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name       string
		driver     string
		cache      *cache.Client
		deployMode string
		wantErr    string // 子串匹配；空表示期望成功
	}{
		// ---- driver + cache 自洽 ----
		{"memory + nil cache ok", "memory", nil, "single", ""},
		{"redis + nil cache fail", "redis", nil, "single", "requires cache"},
		{"empty driver fail", "", nil, "single", "driver is required"},
		{"unknown driver fail (typo)", "Redis", nil, "single", "invalid driver"},
		{"unknown driver fail (other)", "memcached", nil, "single", "invalid driver"},
		{"redis + non-nil cache ok", "redis", &cache.Client{}, "single", ""},

		// ---- 部署模式约束 ----
		{"single + memory ok", "memory", nil, "single", ""},
		{"single + redis ok", "redis", &cache.Client{}, "single", ""},
		{"empty deploy defaults to single + memory ok", "memory", nil, "", ""},
		{"empty deploy defaults to single + redis ok", "redis", &cache.Client{}, "", ""},
		{"cluster + redis ok", "redis", &cache.Client{}, "cluster", ""},
		{"cluster + memory blocked", "memory", nil, "cluster", "requires driver=redis"},

		// ---- 非法 deploy_mode ----
		{"invalid deploy_mode", "redis", &cache.Client{}, "ha", "invalid deploy_mode"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(config.RateLimitConfig{Driver: tc.driver}, tc.cache, tc.deployMode)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("want nil, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}
