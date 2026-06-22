package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManager_LoadAndEnvOverride(t *testing.T) {
	dir := t.TempDir()
	yaml := []byte("server:\n  addr: \":9090\"\n  mode: debug\n")
	if err := os.WriteFile(filepath.Join(dir, "c.yaml"), yaml, 0o644); err != nil {
		t.Fatal(err)
	}
	m := New()
	if err := m.Load(filepath.Join(dir, "c.yaml")); err != nil {
		t.Fatal(err)
	}
	if got := m.Current().Server.Addr; got != ":9090" {
		t.Fatalf("yaml addr: want :9090, got %s", got)
	}
	t.Setenv("ROCWAY_SERVER_ADDR", ":7070")
	m2 := New()
	if err := m2.Load(filepath.Join(dir, "c.yaml")); err != nil {
		t.Fatal(err)
	}
	if got := m2.Current().Server.Addr; got != ":7070" {
		t.Fatalf("env override: want :7070, got %s", got)
	}
}

func TestManager_Defaults(t *testing.T) {
	m := New()
	cfg := m.Current()
	if cfg.Cache.Prefix != "rocway:" {
		t.Fatalf("default prefix wrong: %s", cfg.Cache.Prefix)
	}
	if cfg.Server.Addr != ":8080" {
		t.Fatalf("default addr wrong: %s", cfg.Server.Addr)
	}
}
