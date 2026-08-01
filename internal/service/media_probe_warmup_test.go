package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

type fakeProbeWarmupRepo struct {
	mu      sync.Mutex
	rows    []model.Media
	updates []string
}

func (r *fakeProbeWarmupRepo) ListProbeCandidatesByLibrary(libraryID, afterID string, limit int) ([]model.Media, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]model.Media, 0, limit)
	for _, media := range r.rows {
		if media.LibraryID != libraryID || (afterID != "" && media.ID <= afterID) {
			continue
		}
		result = append(result, media)
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

func (r *fakeProbeWarmupRepo) UpdateTechnicalSummary(mediaID, videoCodec, audioCodec, resolution string, duration float64, fileSize int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, fmt.Sprintf("%s|%s|%s|%s|%.0f|%d", mediaID, videoCodec, audioCodec, resolution, duration, fileSize))
	return nil
}

type fakeProbeWarmupProvider struct {
	mu    sync.Mutex
	calls map[string]int
}

func (p *fakeProbeWarmupProvider) Probe(_ context.Context, media *model.Media) (*model.MediaProbeRecord, error) {
	p.mu.Lock()
	p.calls[media.ID]++
	p.mu.Unlock()
	record := &model.MediaProbeRecord{
		MediaID:    media.ID,
		VideoCodec: "h264",
		Width:      1920,
		Height:     1080,
		DurationMS: 60000,
		SourceSize: 12345,
	}
	_ = record.SetAudioStreams([]model.MediaProbeAudioStream{{Codec: "aac", Default: true}})
	return record, nil
}

func TestMediaProbeWarmupPagesAndSynchronizesTechnicalSummary(t *testing.T) {
	repo := &fakeProbeWarmupRepo{}
	for index := 0; index < defaultProbeWarmupPageSize+5; index++ {
		repo.rows = append(repo.rows, model.Media{
			ID:        fmt.Sprintf("%04d", index),
			LibraryID: "library-1",
			FilePath:  fmt.Sprintf("/media/%04d.mkv", index),
		})
	}
	provider := &fakeProbeWarmupProvider{calls: make(map[string]int)}
	service := NewMediaProbeWarmupService(repo, provider, nil)
	defer service.Shutdown(context.Background())

	if err := service.warmLibrary("library-1"); err != nil {
		t.Fatal(err)
	}
	provider.mu.Lock()
	callCount := len(provider.calls)
	provider.mu.Unlock()
	if callCount != defaultProbeWarmupPageSize+5 {
		t.Fatalf("unexpected probe count: %d", callCount)
	}
	repo.mu.Lock()
	updates := append([]string(nil), repo.updates...)
	repo.mu.Unlock()
	if len(updates) != defaultProbeWarmupPageSize+5 {
		t.Fatalf("technical summaries were not synchronized: %d", len(updates))
	}
	if updates[0] != "0000|h264|aac|1080p|60|12345" {
		t.Fatalf("unexpected summary projection: %s", updates[0])
	}
}

func TestMediaProbeWarmupSubmitIsDeduplicatedAndObservable(t *testing.T) {
	repo := &fakeProbeWarmupRepo{rows: []model.Media{{ID: "1", LibraryID: "library-1", FilePath: "/media/1.mkv"}}}
	provider := &fakeProbeWarmupProvider{calls: make(map[string]int)}
	service := NewMediaProbeWarmupService(repo, provider, nil)

	submitted, err := service.SubmitLibrary("library-1")
	if err != nil || !submitted {
		t.Fatalf("first submit failed: submitted=%v err=%v", submitted, err)
	}
	if duplicate, duplicateErr := service.SubmitLibrary("library-1"); duplicateErr != nil || duplicate {
		t.Fatalf("duplicate submit was not coalesced: submitted=%v err=%v", duplicate, duplicateErr)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stats := service.Stats()
		if stats.CompletedRuns == 1 && stats.ProcessedMedia == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	stats := service.Stats()
	if stats.SubmittedRuns != 1 || stats.CompletedRuns != 1 || stats.ProcessedMedia != 1 || stats.FailedRuns != 0 {
		t.Fatalf("unexpected warmup stats: %+v", stats)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := service.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	if submitted, err := service.SubmitLibrary("library-2"); err == nil || submitted {
		t.Fatalf("closed warmup accepted new work: submitted=%v err=%v", submitted, err)
	}
}
