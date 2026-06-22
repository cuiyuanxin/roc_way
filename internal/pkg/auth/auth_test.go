package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cuiyuanxin/roc_way/internal/pkg/cache"
	"github.com/cuiyuanxin/roc_way/internal/pkg/config"
)

func newAuth(t *testing.T) *Auth {
	t.Helper()
	cfg := config.AuthConfig{JWTSecret: "secret", AccessTTLSec: 60, RefreshTTLSec: 600}
	c, err := cache.New(config.CacheConfig{Addr: "127.0.0.1:0"})
	if err != nil {
		t.Skip("redis not available:", err)
	}
	return New(cfg, c)
}

func TestIssueParse(t *testing.T) {
	a := newAuth(t)
	pair, err := a.Issue("alice")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pair.Access, ".") || !strings.Contains(pair.Refresh, ".") {
		t.Fatalf("malformed token: %+v", pair)
	}
	c, err := a.Parse(pair.Access)
	if err != nil {
		t.Fatal(err)
	}
	if c.Subject != "alice" || c.Kind != "access" {
		t.Fatalf("claims wrong: %+v", c)
	}
}

func TestRevokeAndIsRevoked(t *testing.T) {
	a := newAuth(t)
	pair, _ := a.Issue("alice")
	c, _ := a.Parse(pair.Access)
	if err := a.Revoke(context.Background(), c.ID, time.Minute); err != nil {
		t.Fatal(err)
	}
	ok, _ := a.IsRevoked(context.Background(), c.ID)
	if !ok {
		t.Fatal("expected revoked")
	}
}
