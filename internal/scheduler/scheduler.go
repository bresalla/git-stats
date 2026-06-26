package scheduler

import (
	"context"
	"time"
)

type SyncFunc func(ctx context.Context)

type Scheduler struct {
	interval time.Duration
	sync     SyncFunc
	trigger  chan struct{}
}

func New(interval time.Duration, sync SyncFunc) *Scheduler {
	return &Scheduler{
		interval: interval,
		sync:     sync,
		trigger:  make(chan struct{}, 1),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sync(ctx)
		case <-s.trigger:
			s.sync(ctx)
		}
	}
}

func (s *Scheduler) TriggerNow() {
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}
