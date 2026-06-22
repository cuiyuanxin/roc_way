package errcode

import "testing"

func TestCode_Error(t *testing.T) {
	c := ErrUserNotFound.WithMessage("用户 1 不存在")
	if c.Code != 1001 {
		t.Fatalf("want code 1001, got %d", c.Code)
	}
	if got := c.Error(); got != "code=1001: 用户 1 不存在" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestError_Unwrap(t *testing.T) {
	base := ErrDatabase
	e := New(base, nil)
	if e.Error() == "" {
		t.Fatal("empty error")
	}
}
