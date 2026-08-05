package service

import (
	"errors"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

type fakeScrapeLookup struct{ task *model.ScrapeTask }

func (f *fakeScrapeLookup) FindByID(string) (*model.ScrapeTask, error) {
	if f.task == nil {
		return nil, errors.New("not found")
	}
	return f.task, nil
}

type fakeScrapeActions struct{ retried string }

func (f *fakeScrapeActions) StartScrape(id, _ string) error { f.retried = id; return nil }

type fakeMigrationActions struct{ retried, rolledBack string }

func (f *fakeMigrationActions) RetryArtifactCleanup(id string) error { f.retried = id; return nil }
func (f *fakeMigrationActions) RollbackLegacyArtifactMigration(id string) error {
	f.rolledBack = id
	return nil
}

type fakeArtifactLookup struct {
	artifact *model.TranscodeArtifactRecord
}

func (f *fakeArtifactLookup) FindArtifactCleanupOperation(string) (*model.TranscodeArtifactRecord, error) {
	if f.artifact == nil {
		return nil, errors.New("not found")
	}
	return f.artifact, nil
}

func TestAvailableTaskActions(t *testing.T) {
	if got := AvailableTaskActions(TaskKindScrape, "failed"); len(got) != 1 || got[0] != TaskActionRetry {
		t.Fatalf("scrape actions=%v", got)
	}
	if got := AvailableTaskActions(TaskKindLegacyArtifactMigration, "queued"); len(got) != 1 || got[0] != TaskActionRollback {
		t.Fatalf("migration actions=%v", got)
	}
	if got := AvailableTaskActions(TaskKindScan, "running"); len(got) != 0 {
		t.Fatalf("scan actions=%v", got)
	}
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	if got := AvailableTaskActionsForTask(UnifiedTask{Kind: TaskKindLegacyArtifactMigration, Status: TaskStatusQueued, RollbackUntil: &future}, time.Now()); len(got) != 1 || got[0] != TaskActionRollback {
		t.Fatalf("future migration actions=%v", got)
	}
	if got := AvailableTaskActionsForTask(UnifiedTask{Kind: TaskKindLegacyArtifactMigration, Status: TaskStatusQueued, RollbackUntil: &past}, time.Now()); len(got) != 0 {
		t.Fatalf("expired migration actions=%v", got)
	}
}

func TestTaskActionDispatcherRetryScrape(t *testing.T) {
	actions := &fakeScrapeActions{}
	d := &TaskActionDispatcher{scrape: actions, scrapeLookup: &fakeScrapeLookup{task: &model.ScrapeTask{Status: "failed"}}}
	if _, err := d.Execute(TaskKindScrape, "s-1", TaskActionRetry, "admin"); err != nil {
		t.Fatal(err)
	}
	if actions.retried != "s-1" {
		t.Fatalf("retried=%q", actions.retried)
	}
}

func TestTaskActionDispatcherRollbackLegacyMigration(t *testing.T) {
	actions := &fakeMigrationActions{}
	rollbackUntil := time.Now().Add(time.Hour)
	d := &TaskActionDispatcher{
		artifactCleanup: actions,
		artifactLookup: &fakeArtifactLookup{artifact: &model.TranscodeArtifactRecord{
			MigrationSource:      repository.LegacyTranscodeArtifactMigrationSource,
			CleanupState:         repository.ArtifactCleanupPending,
			CleanupRollbackUntil: &rollbackUntil,
		}},
	}
	if _, err := d.Execute(TaskKindLegacyArtifactMigration, "a-1", TaskActionRollback, "admin"); err != nil {
		t.Fatal(err)
	}
	if actions.rolledBack != "a-1" {
		t.Fatalf("rolledBack=%q", actions.rolledBack)
	}
}

func TestTaskActionDispatcherRejectsExpiredLegacyRollback(t *testing.T) {
	actions := &fakeMigrationActions{}
	rollbackUntil := time.Now().Add(-time.Minute)
	d := &TaskActionDispatcher{
		artifactCleanup: actions,
		artifactLookup: &fakeArtifactLookup{artifact: &model.TranscodeArtifactRecord{
			MigrationSource:      repository.LegacyTranscodeArtifactMigrationSource,
			CleanupState:         repository.ArtifactCleanupPending,
			CleanupRollbackUntil: &rollbackUntil,
		}},
	}
	if _, err := d.Execute(TaskKindLegacyArtifactMigration, "a-expired", TaskActionRollback, "admin"); !errors.Is(err, ErrTaskActionConflict) {
		t.Fatalf("expired rollback error=%v", err)
	}
	if actions.rolledBack != "" {
		t.Fatalf("expired rollback was dispatched: %q", actions.rolledBack)
	}
}
