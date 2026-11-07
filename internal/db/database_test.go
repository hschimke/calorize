package db_test

import (
	"context"
	"testing"
	"time"

	"azule.info/calorize/internal/db"
)

func TestPingContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("expected PingContext to succeed, got %v", err)
	}

	// Test canceled context returns error
	canceledCtx, cancelNow := context.WithCancel(context.Background())
	cancelNow()

	if err := db.PingContext(canceledCtx); err == nil {
		t.Fatal("expected PingContext to fail with canceled context, got nil")
	}
}

func TestSentinelErrors(t *testing.T) {
	errors := []error{
		db.ErrNotFound,
		db.ErrConflict,
		db.ErrInvalidInput,
		db.ErrDatabase,
	}

	// Ensure all errors are defined and non-nil
	for i, err := range errors {
		if err == nil {
			t.Fatalf("error at index %d is nil", i)
		}
	}

	// Ensure sentinel errors are distinct
	for i := range errors {
		for j := i + 1; j < len(errors); j++ {
			if errors[i] == errors[j] {
				t.Fatalf("expected error %v and %v to be distinct sentinels", errors[i], errors[j])
			}
		}
	}
}
