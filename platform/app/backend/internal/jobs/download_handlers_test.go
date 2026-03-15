package jobs

import (
	"context"
	"testing"

	"app-backend/internal/domain"
	"app-backend/internal/integration/download"
)

type fakeDownloadClient struct {
	name   string
	status download.DownloadStatus
}

func (f *fakeDownloadClient) Name() string { return f.name }
func (f *fakeDownloadClient) AddDownload(_ context.Context, _ download.AddRequest) (string, error) {
	return "dl-1", nil
}
func (f *fakeDownloadClient) GetStatus(_ context.Context, id string) (download.DownloadStatus, error) {
	s := f.status
	s.ID = id
	return s, nil
}
func (f *fakeDownloadClient) Remove(_ context.Context, _ string, _ bool) error { return nil }

func TestDownloadEnqueueHandler(t *testing.T) {
	svc := download.NewService(&fakeDownloadClient{name: "nzbget"})
	handler := NewDownloadEnqueueHandler(svc)
	out, err := handler.Handle(context.Background(), domain.Job{
		Type: domain.JobTypeEnqueueDownload,
		Payload: map[string]any{
			"client":   "nzbget",
			"uri":      "https://example.invalid/file.nzb",
			"category": "books",
		},
	})
	if err != nil {
		t.Fatalf("enqueue handle failed: %v", err)
	}
	if out["download_id"] != "dl-1" {
		t.Fatalf("unexpected download id: %v", out["download_id"])
	}
}

func TestDownloadPollHandler(t *testing.T) {
	svc := download.NewService(&fakeDownloadClient{
		name:   "nzbget",
		status: download.DownloadStatus{State: "downloading", Progress: 42},
	})
	handler := NewDownloadPollHandler(svc)
	out, err := handler.Handle(context.Background(), domain.Job{
		Type: domain.JobTypePollDownload,
		Payload: map[string]any{
			"client":      "nzbget",
			"download_id": "dl-1",
		},
	})
	if err != nil {
		t.Fatalf("poll handle failed: %v", err)
	}
	if out["state"] != "downloading" {
		t.Fatalf("unexpected state: %v", out["state"])
	}
}
