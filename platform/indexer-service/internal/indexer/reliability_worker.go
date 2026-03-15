package indexer

import (
	"context"
	"time"
)

type ReliabilityWorker struct {
	store    Storage
	interval time.Duration
}

func NewReliabilityWorker(store Storage, interval time.Duration) *ReliabilityWorker {
	if interval <= 0 {
		interval = 2 * time.Minute
	}
	return &ReliabilityWorker{
		store:    store,
		interval: interval,
	}
}

func (w *ReliabilityWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = w.store.RecomputeReliability()
		}
	}
}
