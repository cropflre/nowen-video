package service

import (
	"strings"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

func TestArtifactCleanupTaskProjectionExposesBlockedDiagnostics(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	artifact := &model.TranscodeArtifactRecord{
		ID:                  "blocked-artifact",
		MediaID:             "media-1",
		ProfileID:           "1080p",
		Path:                "/cache/transcode/artifacts/media-1/1080p/blocked-artifact",
		CleanupState:        repository.ArtifactCleanupBlocked,
		CleanupAttempts:     4,
		CleanupErrorCode:    "cleanup_invariant_violation",
		CleanupErrorMessage: "artifact path escapes store root",
		CreatedAt:           now.Add(-time.Hour),
		UpdatedAt:           now,
	}

	task := artifactCleanupToUnifiedTask(artifact)
	if task.ID != TaskKindArtifactCleanup+":"+artifact.ID || task.Kind != TaskKindArtifactCleanup {
		t.Fatalf("unexpected cleanup identity: %+v", task)
	}
	if task.Status != TaskStatusFailed {
		t.Fatalf("blocked cleanup status=%q, want failed", task.Status)
	}
	for _, expected := range []string{"已阻断", "第 4 次尝试", artifact.CleanupErrorCode, artifact.CleanupErrorMessage, artifact.Path} {
		if !strings.Contains(task.Subtitle+" "+task.Message, expected) {
			t.Fatalf("cleanup projection missing %q: %+v", expected, task)
		}
	}
	if actions := AvailableTaskActions(task.Kind, task.Status); len(actions) != 1 || actions[0] != TaskActionRetry {
		t.Fatalf("blocked cleanup actions=%v, want retry", actions)
	}
}

func TestArtifactCleanupTaskProjectionShowsRetrySchedule(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	next := now.Add(15 * time.Minute)
	artifact := &model.TranscodeArtifactRecord{
		ID:                   "retry-artifact",
		MediaID:              "media-2",
		ProfileID:            "720p",
		CleanupState:         repository.ArtifactCleanupRetryWait,
		CleanupAttempts:      2,
		CleanupNextAttemptAt: &next,
		CleanupErrorCode:     "filesystem_busy",
		CreatedAt:            now.Add(-time.Hour),
		UpdatedAt:            now,
	}

	task := artifactCleanupToUnifiedTask(artifact)
	if task.Status != TaskStatusFailed || !strings.Contains(task.Message, "下次重试") {
		t.Fatalf("retry cleanup projection lost schedule: %+v", task)
	}
}

func TestArtifactCleanupTaskProjectionMapsClaimToRunning(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	artifact := &model.TranscodeArtifactRecord{
		ID:               "claimed-artifact",
		CleanupState:     repository.ArtifactCleanupClaimed,
		CleanupClaimedAt: &now,
		CreatedAt:        now.Add(-time.Hour),
		UpdatedAt:        now,
	}

	task := artifactCleanupToUnifiedTask(artifact)
	if task.Status != TaskStatusRunning || task.StartedAt == nil || !task.StartedAt.Equal(now) {
		t.Fatalf("claimed cleanup projection=%+v", task)
	}
	if actions := AvailableTaskActions(task.Kind, task.Status); len(actions) != 0 {
		t.Fatalf("live cleanup claim must not expose actions: %v", actions)
	}
}
