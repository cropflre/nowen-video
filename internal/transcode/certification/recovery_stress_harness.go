package certification

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/transcode/artifactstore"
	"github.com/nowen-video/nowen-video/internal/transcode/executor"
	"github.com/nowen-video/nowen-video/internal/transcode/governor"
	transcodelongdrift "github.com/nowen-video/nowen-video/internal/transcode/longdrift"
	transcoderecovery "github.com/nowen-video/nowen-video/internal/transcode/recoverystress"
	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
	transcoderuntime "github.com/nowen-video/nowen-video/internal/transcode/runtime"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type recoveryAttempt struct {
	Ordinal   int
	Record    model.TranscodeAttemptRecord
	Artifact  model.TranscodeArtifactRecord
	Workspace string
	Args      []string
}

type recoveryScenarioResult struct {
	Transitions []transcoderecovery.StateTransitionEvidence
	Processes   []transcoderecovery.ProcessEvidence
	Fence       transcoderecovery.LeaseFenceEvidence
	Artifact    transcoderecovery.ArtifactOutcomeEvidence
	ErrorCode   string
}

type recoveryHarness struct {
	db            *gorm.DB
	repo          *repository.TranscodeExecutionRepo
	store         *artifactstore.Store
	runtime       *transcoderuntime.Runtime
	ffmpegPath    string
	workDir       string
	sourcePath    string
	profile       transcodelongdrift.ProfileSpec
	caseSpec      transcodereorder.CaseSpec
	candidate     transcodetimebase.CandidateSpec
	timestampPlan transcodetimestamp.Plan
	scenario      transcoderecovery.ScenarioSpec
	transitions   []transcoderecovery.StateTransitionEvidence
}

func newRecoveryHarness(
	root,
	ffmpegPath,
	sourcePath string,
	profile transcodelongdrift.ProfileSpec,
	caseSpec transcodereorder.CaseSpec,
	candidate transcodetimebase.CandidateSpec,
	timestampPlan transcodetimestamp.Plan,
	scenario transcoderecovery.ScenarioSpec,
) (*recoveryHarness, func(), error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, func() {}, err
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(root, "state.sqlite")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, func() {}, fmt.Errorf("open recovery stress database: %w", err)
	}
	if err := model.AutoMigrateTranscodeExecution(db); err != nil {
		return nil, func() {}, fmt.Errorf("migrate recovery stress database: %w", err)
	}
	store, err := artifactstore.New(filepath.Join(root, "artifact-store"))
	if err != nil {
		return nil, func() {}, err
	}
	runtime := transcoderuntime.New(executor.NewProcessRunner(), governor.New(governor.Config{
		SoftwareTranscodes: 1,
		HardwareTranscodes: 1,
		RemuxStreams:       1,
		OnDemandSegments:   1,
	}))
	closeFn := func() {
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	}
	return &recoveryHarness{
		db:            db,
		repo:          repository.NewTranscodeExecutionRepo(db),
		store:         store,
		runtime:       runtime,
		ffmpegPath:    ffmpegPath,
		workDir:       root,
		sourcePath:    sourcePath,
		profile:       profile,
		caseSpec:      caseSpec,
		candidate:     candidate,
		timestampPlan: timestampPlan,
		scenario:      scenario,
	}, closeFn, nil
}

func (h *recoveryHarness) run(ctx context.Context) (recoveryScenarioResult, error) {
	switch h.scenario.ID {
	case transcoderecovery.ScenarioCancelActiveWrite:
		return h.runCancel(ctx)
	case transcoderecovery.ScenarioSIGKILLRecovery:
		return h.runSIGKILLRecovery(ctx)
	case transcoderecovery.ScenarioENOSPCWrite:
		return h.runENOSPC(ctx)
	case transcoderecovery.ScenarioBoundedResources:
		return h.runBoundedResources(ctx)
	case transcoderecovery.ScenarioStaleLeaseFence:
		return h.runStaleLeaseFence(ctx)
	default:
		return recoveryScenarioResult{}, fmt.Errorf("unsupported recovery stress scenario %q", h.scenario.ID)
	}
}

func (h *recoveryHarness) createClaimedJob(workerID string, now time.Time) (*model.TranscodeJobRecord, error) {
	activeKey := "recovery-stress:" + h.scenario.ID
	job := &model.TranscodeJobRecord{
		ID:                uuid.NewString(),
		MediaID:           "recovery-stress-media",
		Intent:            "hls",
		ProfileID:         h.profile.ID,
		Priority:          100,
		Status:            "queued",
		DesiredState:      "running",
		ActiveKey:         &activeKey,
		SourceFingerprint: filepath.Base(h.sourcePath),
		PlannerVersion:    recoveryStressPlannerVersion,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := h.repo.CreateJob(job); err != nil {
		return nil, err
	}
	h.transition("queued", "running", 0, 0, "", "durable recovery stress job created")
	claimed, ok, err := h.repo.ClaimJob(job.ID, workerID, now.Add(time.Millisecond), recoveryStressLeaseDuration)
	if err != nil || !ok {
		return nil, fmt.Errorf("claim recovery stress job: claimed=%t err=%v", ok, err)
	}
	h.transition("claimed", "running", 1, 0, "", "worker acquired authoritative Lease")
	return claimed, nil
}

func (h *recoveryHarness) prepareAttempt(job *model.TranscodeJobRecord, ordinal int) (*recoveryAttempt, error) {
	number, err := h.repo.NextAttemptNumber(job.ID)
	if err != nil {
		return nil, err
	}
	if number != ordinal {
		return nil, fmt.Errorf("attempt number %d, want %d", number, ordinal)
	}
	attempt := model.TranscodeAttemptRecord{
		ID:        uuid.NewString(),
		JobID:     job.ID,
		Number:    number,
		Backend:   "software",
		Status:    "preparing",
		ExitCode:  -1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := h.repo.CreateAttempt(&attempt); err != nil {
		return nil, err
	}
	workspace, err := h.store.PrepareWorkspace(job.ID, attempt.ID)
	if err != nil {
		return nil, err
	}
	policy := transcodelongdrift.DefaultPolicy()
	policy.DurationMicros = h.scenario.LogicalDurationMicros
	policy.CheckpointIntervalMicros = h.scenario.LogicalDurationMicros
	policy.RepeatCount = 1
	args, err := longDurationHLSArgsForPolicy(h.sourcePath, workspace, h.timestampPlan, h.caseSpec, h.candidate, policy)
	if err != nil {
		return nil, err
	}
	args = insertIsolationBeforeOutput(args, "-progress", "pipe:2", "-nostats")
	commandJSON, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	if err := h.repo.UpdateAttemptWorkspaceAndCommand(attempt.ID, workspace, string(commandJSON), time.Now()); err != nil {
		return nil, err
	}
	running, err := h.repo.SetJobRunning(job.ID, attempt.ID, job.LeaseToken, time.Now())
	if err != nil || !running {
		return nil, fmt.Errorf("set recovery stress job running: updated=%t err=%v", running, err)
	}
	artifact := model.TranscodeArtifactRecord{
		ID:                uuid.NewString(),
		JobID:             job.ID,
		AttemptID:         attempt.ID,
		MediaID:           job.MediaID,
		Kind:              "hls_variant",
		ProfileID:         h.profile.ID,
		SourceFingerprint: job.SourceFingerprint,
		PlannerVersion:    job.PlannerVersion,
		TempPath:          workspace,
		Status:            "staging",
		SegmentDuration:   2,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	if err := h.repo.CreateArtifact(&artifact); err != nil {
		return nil, err
	}
	h.transition("running", "running", ordinal, ordinal, "staging", fmt.Sprintf("Attempt %d owns isolated workspace", ordinal))
	return &recoveryAttempt{Ordinal: ordinal, Record: attempt, Artifact: artifact, Workspace: workspace, Args: args}, nil
}
