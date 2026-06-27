package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewUser_Valid(t *testing.T) {
	now := time.Now()
	u, err := NewUser(0, "alice_01", "a@b.com", "alice", "hash", now)
	if err != nil {
		t.Fatalf("want ok, got err=%v", err)
	}
	if u.Username != "alice_01" {
		t.Fatalf("username not preserved: %q", u.Username)
	}
	if u.Email != "a@b.com" {
		t.Fatalf("email not normalized: %q", u.Email)
	}
	if u.Name != "alice" {
		t.Fatalf("name not trimmed: %q", u.Name)
	}
	if u.CreatedAt != now || u.UpdatedAt != now {
		t.Fatalf("timestamps not set: created=%v updated=%v", u.CreatedAt, u.UpdatedAt)
	}
}

func TestNewUser_InvalidUsername(t *testing.T) {
	cases := []string{"", "abc", "1234", "very_long_username_abcdefghijklmnop", "with space"}
	for _, u := range cases {
		_, err := NewUser(0, u, "a@b.com", "alice", "h", time.Now())
		if err == nil {
			t.Fatalf("username=%q want error, got nil", u)
		}
	}
}

func TestNewUser_InvalidEmail(t *testing.T) {
	cases := []string{"abc", "abc@", "@b.com", "a@b"}
	for _, e := range cases {
		_, err := NewUser(0, "valid_user", e, "alice", "h", time.Now())
		if err == nil {
			t.Fatalf("email=%q want error, got nil", e)
		}
	}
}

func TestNewUser_EmptyEmailAllowed(t *testing.T) {
	u, err := NewUser(0, "valid_user", "", "alice", "h", time.Now())
	if err != nil {
		t.Fatalf("empty email should be allowed, got err=%v", err)
	}
	if u.Email != "" {
		t.Fatalf("empty email not preserved: %q", u.Email)
	}
}

func TestNewUser_Normalize(t *testing.T) {
	u, err := NewUser(0, "  alice_01  ", " a@b.com ", " alice ", "h", time.Now())
	if err != nil {
		t.Fatalf("want ok, got err=%v", err)
	}
	if u.Username != "alice_01" {
		t.Fatalf("username trim failed: %q", u.Username)
	}
	if u.Email != "a@b.com" {
		t.Fatalf("email trim failed: %q", u.Email)
	}
	if u.Name != "alice" {
		t.Fatalf("name trim failed: %q", u.Name)
	}
}

func TestSetName(t *testing.T) {
	u, _ := NewUser(0, "alice_01", "a@b.com", "alice", "h", time.Now())
	if err := u.SetName("alice2", time.Now()); err != nil {
		t.Fatalf("want ok, got %v", err)
	}
	if u.Name != "alice2" {
		t.Fatalf("setname failed: %q", u.Name)
	}
	if err := u.SetName("", time.Now()); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("empty name want ErrInvalidName, got %v", err)
	}
}
