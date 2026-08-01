package service

import (
	"context"
	"errors"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestMediaProbeWarmupInvokesStartupSubmissionHook(t *testing.T) {
	repo := &fakeProbeWarmupRepo{rows: []model.Media{{
		ID:        "media-1",
		LibraryID: "library-1",
		FilePath:  "/media/movie.mkv",
	}}}
	provider := &fakeProbeWarmupProvider{calls: make(map[string]int)}
	service := NewMediaProbeWarmupService(repo, provider, nil)
	defer service.Shutdown(context.Background())

	calls := 0
	service.SetOnProbed(func(media *model.Media, probe *model.MediaProbeRecord) (bool, error) {
		calls++
		if media.VideoCodec != "h264" || media.AudioCodec != "aac" || media.Resolution != "1080p" {
			t.Fatalf("hook observed stale technical summary: media=%+v", media)
		}
		if probe.DurationMS != 60_000 {
			t.Fatalf("hook received unexpected probe: %+v", probe)
		}
		return true, nil
	})
	if err := service.warmLibrary("library-1"); err != nil {
		t.Fatal(err)
	}
	stats := service.Stats()
	if calls != 1 || stats.StartupSubmitted != 1 || stats.StartupSkipped != 0 || stats.StartupFailed != 0 {
		t.Fatalf("unexpected startup hook stats calls=%d stats=%+v", calls, stats)
	}
}

func TestMediaProbeWarmupTracksStartupSkipAndFailure(t *testing.T) {
	repo := &fakeProbeWarmupRepo{rows: []model.Media{
		{ID: "media-1", LibraryID: "library-1", FilePath: "/media/1.mkv"},
		{ID: "media-2", LibraryID: "library-1", FilePath: "/media/2.mkv"},
	}}
	provider := &fakeProbeWarmupProvider{calls: make(map[string]int)}
	service := NewMediaProbeWarmupService(repo, provider, nil)
	defer service.Shutdown(context.Background())

	service.SetOnProbed(func(media *model.Media, _ *model.MediaProbeRecord) (bool, error) {
		if media.ID == "media-1" {
			return false, nil
		}
		return false, errors.New("queue unavailable")
	})
	if err := service.warmLibrary("library-1"); err != nil {
		t.Fatal(err)
	}
	stats := service.Stats()
	if stats.StartupSkipped != 1 || stats.StartupFailed != 1 || stats.StartupSubmitted != 0 {
		t.Fatalf("unexpected startup skip/failure stats: %+v", stats)
	}
	if stats.ProcessedMedia != 2 {
		t.Fatalf("startup failure must not abort probe warmup: %+v", stats)
	}
}
