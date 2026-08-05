package service

import (
	"fmt"
	"path/filepath"
	"strings"
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
	queue := newTranscodePriorityQueue(executionRepo, repos.Transcode, capacity, zap.NewNop().Sugar())
	queue.pollInterval = 10 * time.Millisecond
	return &databaseQueueTestContext{
		db:            db,
		legacyRepo:    repos.Transcode,
		executionRepo: executionRepo,
		queue:         queue,
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

func TestDatabaseTranscodeQueueRejectsRetiredRuntimePayload(t *testing.T) {
	ctx := newDatabaseQueueTestContext(t, 10)
	record := ctx.createJob(t, "retired", TranscodePriorityInteractive, time.Now())
	claimed, ok, err := ctx.executionRepo.ClaimJob(record.ID, "worker", time.Now(), time.Minute)
	if err != nil || !ok {
		t.Fatalf("claim retired fixture: ok=%v err=%v", ok, err)
	}
	job, hydrateErr := ctx.queue.hydrateClaimedJob(claimed)
	if job != nil || hydrateErr == nil || !strings.Contains(hydrateErr.Error(), "unsupported transcode intent") {
		t.Fatalf("retired runtime payload was hydrated job=%+v err=%v", job, hydrateErr)
	}
}

func TestDatabaseTranscodeQueueNeverClaimsRetiredRows(t *testing.T) {
	ctx := newDatabaseQueueTestContext(t, 10)
	record := ctx.createJob(t, "no-claim", TranscodePriorityInteractive, time.Now())

	job, ok := ctx.queue.Pop("retirement-worker", time.Minute)
	if ok || job != nil {
		t.Fatalf("retired queue returned executable work: ok=%v job=%+v", ok, job)
	}
	stored, err := ctx.executionRepo.FindJobByID(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "queued" || stored.LeaseToken != "" || stored.WorkerID != "" {
		t.Fatalf("retired queue mutated or claimed historical row: %+v", stored)
	}
}

func TestDatabaseTranscodeQueueFreshInstanceRemainsRetired(t *testing.T) {
	ctx := newDatabaseQueueTestContext(t, 10)
	record := ctx.createJob(t, "restart", TranscodePriorityInteractive, time.Now())
	restarted := newTranscodePriorityQueue(ctx.executionRepo, ctx.legacyRepo, 10, zap.NewNop().Sugar())

	job, ok := restarted.Pop("restart-worker", time.Minute)
	if ok || job != nil {
		t.Fatalf("fresh queue restored retired runtime work: ok=%v job=%+v", ok, job)
	}
	stored, err := ctx.executionRepo.FindJobByID(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "queued" || stored.LeaseToken != "" {
		t.Fatalf("fresh retired queue touched historical row: %+v", stored)
	}
}

func TestDatabaseTranscodeQueueRejectsCapacityAndCloses(t *testing.T) {
	ctx := newDatabaseQueueTestContext(t, 1)
	if ctx.queue.CanAccept() {
		t.Fatal("retired durable queue accepted work")
	}
	ctx.createJob(t, "capacity", TranscodePriorityBackground, time.Now())
	if ctx.queue.CanAccept() || ctx.queue.Len() != 0 {
		t.Fatal("retired durable queue exposed historical capacity")
	}
	if !ctx.queue.Notify() {
		t.Fatal("retirement lifecycle wake-up was unavailable")
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

func TestDatabaseTranscodeQueueReleasesHistoricalClaimWhenClosed(t *testing.T) {
	ctx := newDatabaseQueueTestContext(t, 10)
	record := ctx.createJob(t, "shutdown-race", TranscodePriorityInteractive, time.Now())
	claimed, ok, err := ctx.executionRepo.ClaimJob(record.ID, "shutdown-worker", time.Now(), time.Minute)
	if err != nil || !ok {
		t.Fatalf("worker did not claim shutdown test job: ok=%v err=%v", ok, err)
	}

	ctx.queue.Close()
	ctx.queue.releaseClaimAfterClose(claimed)
	stored, err := ctx.executionRepo.FindJobByID(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "queued" || stored.LeaseToken != "" || stored.WorkerID != "" || stored.LeaseExpiresAt != nil {
		t.Fatalf("shutdown did not release historical ownership: %+v", stored)
	}
}
