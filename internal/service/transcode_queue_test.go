package service

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type databaseQueueTestContext struct {
	db            *gorm.DB
	legacyRepo    *repository.TranscodeRepo
	executionRepo *repository.TranscodeExecutionRepo
	queue         *transcodePriorityQueue
}

func newDatabaseQueueTestContext(t *testing.T, capacity int) *databaseQueueTestContext {
	t.Helper()
	dsn := fmt.Sprintf("file:transcode-database-queue-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Media{}, &model.TranscodeTask{}); err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	repos := repository.NewRepositories(db)
	executionRepo := repository.NewTranscodeExecutionRepo(db)
	return &databaseQueueTestContext{
		db:            db,
		legacyRepo:    repos.Transcode,
		executionRepo: executionRepo,
		queue:         newTranscodePriorityQueue(executionRepo, repos.Transcode, capacity, zap.NewNop().Sugar()),
	}
}

func (c *databaseQueueTestContext) createJob(t *testing.T, id string, priority int, createdAt time.Time) *model.TranscodeJobRecord {
	t.Helper()
	mediaID := "media-" + id
	media := &model.Media{
		ID:        mediaID,
		LibraryID: "library-test",
		Title:     id,
		MediaType: "movie",
		FilePath:  filepath.Join(t.TempDir(), id+".mp4"),
		Duration:  60,
	}
	if err := c.db.Create(media).Error; err != nil {
		t.Fatal(err)
	}
	task := &model.TranscodeTask{
		ID:         "task-" + id,
		MediaID:    mediaID,
		Status:     "pending",
		Quality:    "720p",
		OutputDir:  filepath.Join(t.TempDir(), id),
		MediaTitle: id,
		Priority:   priority,
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
	}
	if err := c.legacyRepo.Create(task); err != nil {
		t.Fatal(err)
	}
	activeKey := "active-" + id
	legacyID := task.ID
	record := &model.TranscodeJobRecord{
		ID:           "job-" + id,
		LegacyTaskID: &legacyID,
		MediaID:      mediaID,
		Intent:       "runtime_hls",
		ProfileID:    "720p",
		Priority:     priority,
		Status:       "queued",
		DesiredState: "running",
		ActiveKey:    &activeKey,
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt,
	}
	if err := c.executionRepo.CreateJob(record); err != nil {
		t.Fatal(err)
	}
	return record
}

func TestDatabaseTranscodeQueueClaimsPriorityThenFIFO(t *testing.T) {
	ctx := newDatabaseQueueTestContext(t, 10)
	base := time.Date(2026, 7, 31, 13, 0, 0, 0, time.UTC)
	ctx.createJob(t, "background", TranscodePriorityBackground, base)
	ctx.createJob(t, "interactive-later", TranscodePriorityInteractive, base.Add(2*time.Second))
	ctx.createJob(t, "interactive-first", TranscodePriorityInteractive, base.Add(time.Second))
	ctx.createJob(t, "retry", TranscodePriorityRetry, base.Add(3*time.Second))

	for index, expected := range []string{"task-interactive-first", "task-interactive-later", "task-retry", "task-background"} {
		job, ok := ctx.queue.Pop(fmt.Sprintf("worker-%d", index), time.Minute)
		if !ok || job == nil || job.Task.ID != expected {
			t.Fatalf("claim[%d] expected %s, got %+v", index, expected, job)
		}
		if job.leaseToken == "" || job.ExecutionJob.Status != "claimed" {
			t.Fatalf("claimed job has no durable ownership: %+v", job.ExecutionJob)
		}
	}
}

func TestDatabaseTranscodeQueueRestoresWithoutLocalPush(t *testing.T) {
	ctx := newDatabaseQueueTestContext(t, 10)
	record := ctx.createJob(t, "restart", TranscodePriorityInteractive, time.Now())

	// A fresh queue has no process-local delivery state, but it must still find
	// the durable row immediately.
	restarted := newTranscodePriorityQueue(ctx.executionRepo, ctx.legacyRepo, 10, zap.NewNop().Sugar())
	job, ok := restarted.Pop("restart-worker", time.Minute)
	if !ok || job == nil || job.ExecutionJob.ID != record.ID {
		t.Fatalf("durable job was not restored: %+v", job)
	}
	if job.Media.ID != record.MediaID || job.Task.ID != *record.LegacyTaskID {
		t.Fatalf("durable payload was not reconstructed: job=%+v", job)
	}
}

func TestDatabaseTranscodeQueueSkipsInvalidPayload(t *testing.T) {
	ctx := newDatabaseQueueTestContext(t, 10)
	base := time.Now()
	missingTaskID := "missing-task"
	invalidKey := "invalid-key"
	invalid := &model.TranscodeJobRecord{
		ID:           "job-invalid",
		LegacyTaskID: &missingTaskID,
		MediaID:      "missing-media",
		Intent:       "runtime_hls",
		ProfileID:    "720p",
		Priority:     TranscodePriorityInteractive,
		Status:       "queued",
		DesiredState: "running",
		ActiveKey:    &invalidKey,
		CreatedAt:    base,
		UpdatedAt:    base,
	}
	if err := ctx.executionRepo.CreateJob(invalid); err != nil {
		t.Fatal(err)
	}
	valid := ctx.createJob(t, "valid-after-invalid", TranscodePriorityBackground, base.Add(time.Second))

	job, ok := ctx.queue.Pop("worker", time.Minute)
	if !ok || job == nil || job.ExecutionJob.ID != valid.ID {
		t.Fatalf("invalid payload blocked durable queue: %+v", job)
	}
	stored, err := ctx.executionRepo.FindJobByID(invalid.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "failed" || stored.ActiveKey != nil {
		t.Fatalf("invalid durable payload was not terminalized: %+v", stored)
	}
}

func TestDatabaseTranscodeQueueCapacityAndClose(t *testing.T) {
	ctx := newDatabaseQueueTestContext(t, 1)
	if !ctx.queue.CanAccept() {
		t.Fatal("empty durable queue rejected work")
	}
	ctx.createJob(t, "capacity", TranscodePriorityBackground, time.Now())
	if ctx.queue.CanAccept() {
		t.Fatal("durable queue accepted work beyond configured capacity")
	}
	ctx.queue.Close()
	if !ctx.queue.IsClosed() || ctx.queue.Notify() {
		t.Fatal("closed database queue accepted a wake-up")
	}
	if job, ok := ctx.queue.Pop("late-worker", time.Minute); ok || job != nil {
		t.Fatalf("closed database queue returned work: %+v", job)
	}
	if drained := ctx.queue.CloseAndDrain(); len(drained) != 0 {
		t.Fatalf("database queue must not expose process-local deliveries: %d", len(drained))
	}
}

func TestDatabaseTranscodeQueueReleasesClaimWhenClosed(t *testing.T) {
	ctx := newDatabaseQueueTestContext(t, 10)
	record := ctx.createJob(t, "shutdown-race", TranscodePriorityInteractive, time.Now())
	job, ok := ctx.queue.Pop("shutdown-worker", time.Minute)
	if !ok || job == nil {
		t.Fatal("worker did not claim shutdown test job")
	}

	ctx.queue.Close()
	ctx.queue.releaseClaimAfterClose(job.ExecutionJob)
	stored, err := ctx.executionRepo.FindJobByID(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "queued" || stored.LeaseToken != "" || stored.WorkerID != "" || stored.LeaseExpiresAt != nil {
		t.Fatalf("shutdown did not release just-claimed ownership: %+v", stored)
	}
}
