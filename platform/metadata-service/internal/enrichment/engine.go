package enrichment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"metadata-service/internal/enrichment/handlers"
	"metadata-service/internal/metrics"
	"metadata-service/internal/store"
)

// Engine starts and supervises N enrichment workers.
type Engine struct {
	workerCount            int
	queueDepthPollInterval time.Duration
	store                  store.EnrichmentJobStore
	handlers               *handlers.Registry
}

func NewEngine(workerCount int, jobStore store.EnrichmentJobStore, registry *handlers.Registry) *Engine {
	if workerCount <= 0 {
		workerCount = 1
	}
	return &Engine{
		workerCount:            workerCount,
		queueDepthPollInterval: 10 * time.Second,
		store:                  jobStore,
		handlers:               registry,
	}
}

// Start blocks until ctx is cancelled and all workers exit.
func (e *Engine) Start(ctx context.Context) {
	metrics.EnrichmentWorkersTotal.Set(float64(e.workerCount))
	go e.runQueueDepthPoller(ctx)

	var wg sync.WaitGroup
	for i := 0; i < e.workerCount; i++ {
		wg.Add(1)
		worker := NewWorker(fmt.Sprintf("enrichment-worker-%d", i+1), e.store, e.handlers)
		go func() {
			defer wg.Done()
			worker.Run(ctx)
		}()
	}
	wg.Wait()
}

func (e *Engine) runQueueDepthPoller(ctx context.Context) {
	ticker := time.NewTicker(e.queueDepthPollInterval)
	defer ticker.Stop()

	e.updateQueueDepthMetrics(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.updateQueueDepthMetrics(ctx)
		}
	}
}

func (e *Engine) updateQueueDepthMetrics(ctx context.Context) {
	counts, err := e.store.CountJobsByStatus(ctx)
	if err != nil {
		return
	}

	knownStatuses := []string{"queued", "running", "succeeded", "failed", "dead", "cancelled"}
	for _, status := range knownStatuses {
		metrics.EnrichmentQueueDepth.WithLabelValues(status).Set(float64(counts[status]))
	}
}
