package notify

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestNoopNotifier_Notify_NoPanic(t *testing.T) {
	log, _ := zap.NewProduction()
	defer log.Sync()
	n := NewNoopNotifier(log.Sugar())

	// 不允许 panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Notify panicked: %v", r)
		}
	}()

	n.Notify(context.Background(), Event{
		Type:        "account_locked_short",
		Username:    "alice_01",
		Level:       "short",
		IP:          "1.2.3.4",
		FailedCount: 5,
	})
}

func TestNoopNotifier_Notify_NilLog(t *testing.T) {
	n := NewNoopNotifier(nil)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Notify panicked with nil log: %v", r)
		}
	}()
	n.Notify(context.Background(), Event{Username: "x"})
}