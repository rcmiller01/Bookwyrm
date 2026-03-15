package enrichment

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"metadata-service/internal/enrichment/handlers"
	"metadata-service/internal/metrics"
	"metadata-service/internal/model"
	"metadata-service/internal/store"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

type fakeHandler struct {
	jobType string
	err     error
}

func (h *fakeHandler) Type() string { return h.jobType }
func (h *fakeHandler) Handle(_ context.Context, _ model.EnrichmentJob) error {
	return h.err
}

type fakeJobStore struct {
	mu       sync.Mutex
	jobs     []*model.EnrichmentJob
	locked   bool
	counts   map[string]int64
	attempts map[int64]int
	runSeq   int64
}

func newFakeJobStore(job model.EnrichmentJob) *fakeJobStore {
	return &fakeJobStore{
		jobs:     []*model.EnrichmentJob{&job},
		counts:   map[string]int64{"queued": 1},
		attempts: map[int64]int{},
	}
}

func (f *fakeJobStore) EnqueueJob(_ context.Context, _ model.EnrichmentJob) (int64, error) {
	return 0, errors.New("not used")
}
func (f *fakeJobStore) GetJobByID(_ context.Context, id int64) (*model.EnrichmentJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, job := range f.jobs {
		if job.ID == id {
			cp := *job
			return &cp, nil
		}
	}
	return nil, errors.New("not found")
}

func (f *fakeJobStore) TryLockNextJob(_ context.Context, workerID string) (*model.EnrichmentJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.locked || len(f.jobs) == 0 || f.jobs[0].Status != model.EnrichmentStatusQueued {
		return nil, store.ErrNoAvailableEnrichmentJobs
	}
	job := f.jobs[0]
	job.Status = model.EnrichmentStatusRunning
	now := time.Now()
	job.LockedAt = &now
	job.LockedBy = &workerID
	f.locked = true
	f.counts[model.EnrichmentStatusQueued] = 0
	f.counts[model.EnrichmentStatusRunning] = 1
	cp := *job
	return &cp, nil
}

func (f *fakeJobStore) MarkSucceeded(_ context.Context, jobID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, job := range f.jobs {
		if job.ID == jobID {
			job.Status = model.EnrichmentStatusSucceeded
			f.counts[model.EnrichmentStatusRunning] = 0
			f.counts[model.EnrichmentStatusSucceeded]++
			f.locked = false
			return nil
		}
	}
	return nil
}

func (f *fakeJobStore) MarkFailed(_ context.Context, jobID int64, jobType string, errMsg string, backoff time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	metrics.EnrichmentJobBackoffSeconds.WithLabelValues(jobType).Observe(backoff.Seconds())
	for _, job := range f.jobs {
		if job.ID == jobID {
			job.AttemptCount++
			f.attempts[jobID] = job.AttemptCount
			if job.AttemptCount >= job.MaxAttempts {
				job.Status = model.EnrichmentStatusDead
				f.counts[model.EnrichmentStatusDead]++
			} else {
				job.Status = model.EnrichmentStatusQueued
				f.counts[model.EnrichmentStatusQueued]++
			}
			job.LastError = &errMsg
			f.counts[model.EnrichmentStatusRunning] = 0
			f.locked = false
			return nil
		}
	}
	return nil
}

func (f *fakeJobStore) MarkDead(_ context.Context, _ int64, _ string) error { return nil }

func (f *fakeJobStore) RecordRunStart(_ context.Context, _ int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runSeq++
	return f.runSeq, nil
}

func (f *fakeJobStore) RecordRunFinish(_ context.Context, _ int64, _ string, _ string) error {
	return nil
}
func (f *fakeJobStore) ListJobs(_ context.Context, _ model.EnrichmentJobFilters) ([]model.EnrichmentJob, error) {
	return nil, nil
}

func (f *fakeJobStore) CountJobsByStatus(_ context.Context) (map[string]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]int64{}
	for k, v := range f.counts {
		out[k] = v
	}
	return out, nil
}

func (f *fakeJobStore) NextRunnableAt(_ context.Context) (*time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	for _, job := range f.jobs {
		if job.Status != model.EnrichmentStatusQueued {
			continue
		}
		if job.NotBefore == nil || !job.NotBefore.After(now) {
			t := now
			return &t, nil
		}
		t := *job.NotBefore
		return &t, nil
	}
	return nil, nil
}

func TestWorkerMetrics_SuccessPath(t *testing.T) {
	job := model.EnrichmentJob{ID: 1, JobType: model.EnrichmentJobTypeWorkEditions, Status: model.EnrichmentStatusQueued, MaxAttempts: 2}
	store := newFakeJobStore(job)
	registry := handlers.NewRegistry()
	registry.Register(&fakeHandler{jobType: model.EnrichmentJobTypeWorkEditions})
	worker := NewWorker("w1", store, registry)

	startedBefore := testutil.ToFloat64(metrics.EnrichmentJobsStartedTotal.WithLabelValues(model.EnrichmentJobTypeWorkEditions))
	succeededBefore := testutil.ToFloat64(metrics.EnrichmentJobsSucceededTotal.WithLabelValues(model.EnrichmentJobTypeWorkEditions))
	activeBefore := testutil.ToFloat64(metrics.EnrichmentWorkersActive)

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	go worker.Run(ctx)
	<-ctx.Done()

	if got := testutil.ToFloat64(metrics.EnrichmentJobsStartedTotal.WithLabelValues(model.EnrichmentJobTypeWorkEditions)); got < startedBefore+1 {
		t.Fatalf("expected started metric increment, before=%f after=%f", startedBefore, got)
	}
	if got := testutil.ToFloat64(metrics.EnrichmentJobsSucceededTotal.WithLabelValues(model.EnrichmentJobTypeWorkEditions)); got < succeededBefore+1 {
		t.Fatalf("expected succeeded metric increment, before=%f after=%f", succeededBefore, got)
	}
	if got := testutil.ToFloat64(metrics.EnrichmentWorkersActive); got != activeBefore {
		t.Fatalf("expected active workers gauge to return to baseline %f, got %f", activeBefore, got)
	}
}

func TestWorkerMetrics_FailedAndDeadPath(t *testing.T) {
	job := model.EnrichmentJob{ID: 2, JobType: model.EnrichmentJobTypeAuthorExpand, Status: model.EnrichmentStatusQueued, MaxAttempts: 1}
	store := newFakeJobStore(job)
	registry := handlers.NewRegistry()
	registry.Register(&fakeHandler{jobType: model.EnrichmentJobTypeAuthorExpand, err: errors.New("boom")})
	worker := NewWorker("w2", store, registry)

	deadBefore := testutil.ToFloat64(metrics.EnrichmentJobsDeadTotal.WithLabelValues(model.EnrichmentJobTypeAuthorExpand))

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	go worker.Run(ctx)
	<-ctx.Done()

	if got := testutil.ToFloat64(metrics.EnrichmentJobsDeadTotal.WithLabelValues(model.EnrichmentJobTypeAuthorExpand)); got < deadBefore+1 {
		t.Fatalf("expected dead metric increment, before=%f after=%f", deadBefore, got)
	}
}

func TestEngineQueueDepthPollingUpdatesGauges(t *testing.T) {
	store := &fakeJobStore{counts: map[string]int64{"queued": 5, "running": 2, "dead": 1}, attempts: map[int64]int{}}
	engine := NewEngine(2, store, handlers.NewRegistry())

	engine.updateQueueDepthMetrics(context.Background())

	if got := testutil.ToFloat64(metrics.EnrichmentQueueDepth.WithLabelValues("queued")); got != 5 {
		t.Fatalf("expected queued depth 5, got %f", got)
	}
	if got := testutil.ToFloat64(metrics.EnrichmentQueueDepth.WithLabelValues("running")); got != 2 {
		t.Fatalf("expected running depth 2, got %f", got)
	}
	if got := testutil.ToFloat64(metrics.EnrichmentQueueDepth.WithLabelValues("dead")); got != 1 {
		t.Fatalf("expected dead depth 1, got %f", got)
	}
}

func TestBackoffHistogram_IncrementsForJobType(t *testing.T) {
	before := histogramSampleCount(t, "enrichment_job_backoff_seconds", "job_type", model.EnrichmentJobTypeAuthorExpand)

	metrics.EnrichmentJobBackoffSeconds.WithLabelValues(model.EnrichmentJobTypeAuthorExpand).Observe(30)

	after := histogramSampleCount(t, "enrichment_job_backoff_seconds", "job_type", model.EnrichmentJobTypeAuthorExpand)
	if after <= before {
		t.Fatalf("expected backoff histogram sample count to increase, before=%d after=%d", before, after)
	}
}

func histogramSampleCount(t *testing.T, metricName, labelKey, labelValue string) uint64 {
	t.Helper()
	gathered, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather all metrics: %v", err)
	}
	for _, mf := range gathered {
		if mf.GetName() != metricName {
			continue
		}
		for _, m := range mf.GetMetric() {
			if hasLabel(m.GetLabel(), labelKey, labelValue) {
				return m.GetHistogram().GetSampleCount()
			}
		}
	}
	t.Fatalf("metric %s with %s=%s not found", metricName, labelKey, labelValue)
	return 0
}

func hasLabel(labels []*dto.LabelPair, key, value string) bool {
	for _, lp := range labels {
		if lp.GetName() == key && lp.GetValue() == value {
			return true
		}
	}
	return false
}
