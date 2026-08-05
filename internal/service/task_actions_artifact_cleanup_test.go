package service

import (
	"errors"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

type fakeArtifactCleanupLookup struct {
	artifact *model.TranscodeArtifactRecord
}

func (f *fakeArtifactCleanupLookup) FindArtifactCleanupOperation(string) (*model.TranscodeArtifactRecord, error) {
	if f.artifact == nil {
		return nil, errors.New("not found")
	}
	return f.artifact, nil
}

type fakeArtifactCleanupActions struct {
	retried string
	err     error
}

func (f *fakeArtifactCleanupActions) RetryArtifactCleanup(id string) error {
	f.retried = id
	return f.err
}

func (f *fakeArtifactCleanupActions) RollbackLegacyArtifactMigration(string) error {
	return f.err
}

func TestTaskActionDispatcherRetriesBlockedArtifactCleanup(t *testing.T) {
	actions := &fakeArtifactCleanupActions{}
	dispatcher := &TaskActionDispatcher{
		artifactCleanup: actions,
		artifactLookup: &fakeArtifactCleanupLookup{artifact: &model.TranscodeArtifactRecord{
			ID:           "artifact-1",
			CleanupState: repository.ArtifactCleanupBlocked,
		}},
	}

	result, err := dispatcher.Execute(TaskKindArtifactCleanup, "artifact-1", TaskActionRetry, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if actions.retried != "artifact-1" || !result.Accepted || result.Kind != TaskKindArtifactCleanup {
		t.Fatalf("unexpected cleanup retry result=%+v retried=%q", result, actions.retried)
	}
}

func TestTaskActionDispatcherRetriesScheduledArtifactCleanupEarly(t *testing.T) {
	actions := &fakeArtifactCleanupActions{}
	dispatcher := &TaskActionDispatcher{
		artifactCleanup: actions,
		artifactLookup: &fakeArtifactCleanupLookup{artifact: &model.TranscodeArtifactRecord{
			ID:           "artifact-2",
			CleanupState: repository.ArtifactCleanupRetryWait,
		}},
	}

	if _, err := dispatcher.Execute(TaskKindArtifactCleanup, "artifact-2", TaskActionRetry, "admin"); err != nil {
		t.Fatal(err)
	}
	if actions.retried != "artifact-2" {
		t.Fatalf("scheduled cleanup was not retried: %q", actions.retried)
	}
}

func TestTaskActionDispatcherProtectsLiveArtifactCleanup(t *testing.T) {
	dispatcher := &TaskActionDispatcher{
		artifactCleanup: &fakeArtifactCleanupActions{},
		artifactLookup: &fakeArtifactCleanupLookup{artifact: &model.TranscodeArtifactRecord{
			ID:           "artifact-live",
			CleanupState: repository.ArtifactCleanupClaimed,
		}},
	}

	_, err := dispatcher.Execute(TaskKindArtifactCleanup, "artifact-live", TaskActionRetry, "admin")
	if !errors.Is(err, ErrTaskActionConflict) {
		t.Fatalf("expected live cleanup conflict, got %v", err)
	}
}

func TestTaskActionDispatcherRejectsUnsafeCleanupAction(t *testing.T) {
	dispatcher := &TaskActionDispatcher{
		artifactCleanup: &fakeArtifactCleanupActions{},
		artifactLookup: &fakeArtifactCleanupLookup{artifact: &model.TranscodeArtifactRecord{
			ID:           "artifact-blocked",
			CleanupState: repository.ArtifactCleanupBlocked,
		}},
	}

	_, err := dispatcher.Execute(TaskKindArtifactCleanup, "artifact-blocked", "cancel", "admin")
	if !errors.Is(err, ErrTaskActionUnsupported) {
		t.Fatalf("expected unsupported cleanup action, got %v", err)
	}
}
