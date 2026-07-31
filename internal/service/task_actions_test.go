package service

import (
	"errors"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

type fakeTranscodeLookup struct{ task *model.TranscodeTask }

func (f *fakeTranscodeLookup) FindByID(string) (*model.TranscodeTask, error) {
	if f.task == nil {
		return nil, errors.New("not found")
	}
	return f.task, nil
}

type fakeScrapeLookup struct{ task *model.ScrapeTask }

func (f *fakeScrapeLookup) FindByID(string) (*model.ScrapeTask, error) {
	if f.task == nil {
		return nil, errors.New("not found")
	}
	return f.task, nil
}

type fakeTranscodeActions struct {
	cancelled string
	retried   string
}

func (f *fakeTranscodeActions) CancelTranscode(id string) error {
	f.cancelled = id
	return nil
}

func (f *fakeTranscodeActions) RetryTask(id string, resolver func(string) (*model.Media, error)) error {
	f.retried = id
	_, err := resolver("media-1")
	return err
}

type fakeScrapeActions struct{ retried string }

func (f *fakeScrapeActions) StartScrape(id, _ string) error {
	f.retried = id
	return nil
}

func TestAvailableTaskActions(t *testing.T) {
	tests := []struct {
		kind   string
		status string
		want   string
	}{
		{TaskKindTranscode, "pending", TaskActionCancel},
		{TaskKindTranscode, "queued", TaskActionCancel},
		{TaskKindTranscode, "running", TaskActionCancel},
		{TaskKindTranscode, "failed", TaskActionRetry},
		{TaskKindTranscode, "cancelled", TaskActionRetry},
		{TaskKindScrape, "failed", TaskActionRetry},
	}
	for _, tt := range tests {
		actions := AvailableTaskActions(tt.kind, tt.status)
		if len(actions) != 1 || actions[0] != tt.want {
			t.Fatalf("kind=%s status=%s actions=%v want=%s", tt.kind, tt.status, actions, tt.want)
		}
	}
	if actions := AvailableTaskActions(TaskKindScan, "running"); len(actions) != 0 {
		t.Fatalf("scan actions must stay empty: %v", actions)
	}
}

func TestTaskActionDispatcherCancelTranscode(t *testing.T) {
	for _, status := range []string{"pending", "running"} {
		actions := &fakeTranscodeActions{}
		dispatcher := &TaskActionDispatcher{
			transcode:       actions,
			transcodeLookup: &fakeTranscodeLookup{task: &model.TranscodeTask{Status: status}},
		}
		result, err := dispatcher.Execute(TaskKindTranscode, "t-1", TaskActionCancel, "admin")
		if err != nil {
			t.Fatalf("status=%s: %v", status, err)
		}
		if actions.cancelled != "t-1" || !result.Accepted || result.Action != TaskActionCancel {
			t.Fatalf("status=%s result=%+v cancelled=%s", status, result, actions.cancelled)
		}
	}
}

func TestTaskActionDispatcherRetryTranscode(t *testing.T) {
	actions := &fakeTranscodeActions{}
	dispatcher := &TaskActionDispatcher{
		transcode:       actions,
		transcodeLookup: &fakeTranscodeLookup{task: &model.TranscodeTask{Status: "failed"}},
		mediaResolver: func(string) (*model.Media, error) {
			return &model.Media{ID: "media-1"}, nil
		},
	}
	if _, err := dispatcher.Execute(TaskKindTranscode, "t-2", TaskActionRetry, "admin"); err != nil {
		t.Fatal(err)
	}
	if actions.retried != "t-2" {
		t.Fatalf("expected retry call, got %q", actions.retried)
	}
}

func TestTaskActionDispatcherRetryScrape(t *testing.T) {
	actions := &fakeScrapeActions{}
	dispatcher := &TaskActionDispatcher{
		scrape:       actions,
		scrapeLookup: &fakeScrapeLookup{task: &model.ScrapeTask{Status: "failed"}},
	}
	if _, err := dispatcher.Execute(TaskKindScrape, "s-1", TaskActionRetry, "admin"); err != nil {
		t.Fatal(err)
	}
	if actions.retried != "s-1" {
		t.Fatalf("expected scrape retry call, got %q", actions.retried)
	}
}

func TestTaskActionDispatcherRejectsUnsafeActions(t *testing.T) {
	dispatcher := &TaskActionDispatcher{
		transcode:       &fakeTranscodeActions{},
		transcodeLookup: &fakeTranscodeLookup{task: &model.TranscodeTask{Status: "done"}},
	}
	_, err := dispatcher.Execute(TaskKindTranscode, "t-3", TaskActionCancel, "admin")
	if !errors.Is(err, ErrTaskActionConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}

	_, err = dispatcher.Execute(TaskKindScan, "lib-1", TaskActionCancel, "admin")
	if !errors.Is(err, ErrTaskActionUnsupported) {
		t.Fatalf("expected unsupported, got %v", err)
	}
}
