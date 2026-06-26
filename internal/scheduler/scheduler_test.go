package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduler_TicksOnInterval(t *testing.T) {
	var count int32
	s := New(20*time.Millisecond, func(ctx context.Context) {
		atomic.AddInt32(&count, 1)
	})

	ctx, cancel := context.WithCancel(context.Background())
	go s.Start(ctx)
	defer cancel()

	time.Sleep(70 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)

	if got := atomic.LoadInt32(&count); got < 2 {
		t.Errorf("expected at least 2 ticks in 70ms at 20ms interval, got %d", got)
	}
}

func TestScheduler_TriggerNowRunsImmediately(t *testing.T) {
	done := make(chan struct{}, 1)
	s := New(time.Hour, func(ctx context.Context) {
		done <- struct{}{}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Start(ctx)

	s.TriggerNow()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected TriggerNow to cause an immediate sync run")
	}
}
