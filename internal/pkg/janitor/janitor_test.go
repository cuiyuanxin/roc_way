package janitor

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunners_StartStop_NoLeak(t *testing.T) {
	var called int32
	j := &Janitor{
		Name: "test", Interval: 50 * time.Millisecond, Retention: time.Hour,
		Cleanup: func(_ context.Context, _ time.Time) (int64, error) {
			atomic.AddInt32(&called, 1)
			return 1, nil
		},
	}
	runners := StartAll(context.Background(), []*Janitor{j}, nil)
	time.Sleep(180 * time.Millisecond)
	runners.Stop()
	// Stop 后 Cancel 应已发出 goroutine 退出；等待清理
	time.Sleep(50 * time.Millisecond)
	count := atomic.LoadInt32(&called)
	if count == 0 {
		t.Fatalf("janitor never called, got 0")
	}
	if count > 5 {
		t.Fatalf("janitor called too often, want <=5, got %d", count)
	}
}

func TestRunners_ErrorCallback(t *testing.T) {
	var errMsg atomic.Value
	j := &Janitor{
		Name: "err_test", Interval: 30 * time.Millisecond, Retention: time.Hour,
		Cleanup: func(_ context.Context, _ time.Time) (int64, error) {
			return 0, errors.New("simulated failure")
		},
	}
	runners := StartAll(context.Background(), []*Janitor{j}, func(err error, name string) {
		errMsg.Store(err.Error() + "/" + name)
	})
	defer runners.Stop()
	time.Sleep(80 * time.Millisecond)
	v := errMsg.Load()
	if v == nil {
		t.Fatal("error callback not invoked")
	}
	if v.(string) != "simulated failure/err_test" {
		t.Fatalf("unexpected error: %v", v)
	}
}

func TestJanitor_ZeroInterval_NoOp(t *testing.T) {
	called := false
	j := &Janitor{
		Name: "noop", Interval: 0,
		Cleanup: func(_ context.Context, _ time.Time) (int64, error) {
			called = true
			return 0, nil
		},
	}
	stop := j.Run(context.Background(), nil)
	defer stop()
	time.Sleep(50 * time.Millisecond)
	if called {
		t.Fatal("zero-interval janitor should not run")
	}
}