package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
	transcodedomain "github.com/nowen-video/nowen-video/internal/transcode/domain"
	"gorm.io/gorm"
)

const (
	startupContinuationPlannerVersion = "startup-continuation-hls-v4"
	startupContinuationArtifactKind   = "startup_continuation_hls"
	startupContinuationPriority       = 95
)

// StartupStreamDescriptor is the immutable, source-matched startup artifact
// that may be exposed to playback planning. Filesystem paths and canonical plan
// or attestation JSON are intentionally kept inside the service boundary; API
// clients receive authenticated URLs plus safe identity diagnostics only.
type StartupStreamDescriptor struct {
	MediaID              string
	ProfileID            string
	SourceFingerprint    string
	DurationMS           int64
	ArtifactID           string
	ManifestPath         string
	OutputDir            string
	EncodingPlanVersion  string
	EncodingPlanHash     string
	EncodingPlanJSON     string
	TimestampPlanVersion string
	TimestampPlanHash    string
	TimestampPlanJSON    string
	TimelineOriginMS     int64
	AttestationVersion   string
	AttestationHash      string
	AttestationJSON      string
	TimelineStartMS      int64
	TimelineEndMS        int64
	Probe                *model.MediaProbeRecord
}

func (s *TranscodeService) ResolvePublishedStartupStream(media *model.Media) (*StartupStreamDescriptor, error) {
	if s == nil || s.executionRepo == nil || media == nil {
		return nil, gorm.ErrRecordNotFound
	}
	probe := s.GetCachedMediaProbe(media)
	if probe == nil || probe.SourceFingerprint == "" || !StartupStreamEligible(media, probe) {
		return nil, gorm.ErrRecordNotFound
	}
	profileID := startupStreamProfile(probe)
	encodingIdentity, err := startupEncodingIdentity(probe, profileID)
	if err != nil {
		return nil, fmt.Errorf("build startup encoding plan: %w", err)
	}
	timestampIdentity, err := startupTimestampIdentity()
	if err != nil {
		return nil, fmt.Errorf("build startup timestamp plan: %w", err)
	}
	artifact, err := s.executionRepo.FindPublishedArtifactByExecutionContract(
		media.ID,
		profileID,
		probe.SourceFingerprint,
		startupStreamPlannerVersion,
		startupStreamArtifactKind,
		encodingIdentity.Version,
		encodingIdentity.Hash,
		timestampIdentity.Version,
		timestampIdentity.Hash,
		0,
	)
	if err != nil {
		return nil, err
	}
	if !sameEncodingPlan(
		artifact.EncodingPlanVersion,
		artifact.EncodingPlanHash,
		artifact.EncodingPlanJSON,
		encodingIdentity.Version,
		encodingIdentity.Hash,
		encodingIdentity.Canonical,
	) || !sameTimestampPlan(
		artifact.TimestampPlanVersion,
		artifact.TimestampPlanHash,
		artifact.TimestampPlanJSON,
		timestampIdentity.Version,
		timestampIdentity.Hash,
		timestampIdentity.Canonical,
	) || artifact.TimelineOriginMS != 0 {
		return nil, gorm.ErrRecordNotFound
	}
	if _, err := decodeArtifactAttestation(artifact); err != nil {
		return nil, gorm.ErrRecordNotFound
	}
	manifestPath := artifact.ManifestPath
	if manifestPath == "" && artifact.Path != "" {
		manifestPath = filepath.Join(artifact.Path, "stream.m3u8")
	}
	if manifestPath == "" {
		return nil, gorm.ErrRecordNotFound
	}
	if _, err := os.Stat(manifestPath); err != nil {
		return nil, gorm.ErrRecordNotFound
	}
	durationMS := artifact.DurationMS
	if durationMS <= 0 {
		durationMS = startupStreamDurationMS
	}
	return &StartupStreamDescriptor{
		MediaID:              media.ID,
		ProfileID:            profileID,
		SourceFingerprint:    probe.SourceFingerprint,
		DurationMS:           durationMS,
		ArtifactID:           artifact.ID,
		ManifestPath:         manifestPath,
		OutputDir:            artifact.Path,
		EncodingPlanVersion:  encodingIdentity.Version,
		EncodingPlanHash:     encodingIdentity.Hash,
		EncodingPlanJSON:     encodingIdentity.Canonical,
		TimestampPlanVersion: timestampIdentity.Version,
		TimestampPlanHash:    timestampIdentity.Hash,
		TimestampPlanJSON:    timestampIdentity.Canonical,
		TimelineOriginMS:     artifact.TimelineOriginMS,
		AttestationVersion:   artifact.AttestationVersion,
		AttestationHash:      artifact.AttestationHash,
		AttestationJSON:      artifact.AttestationJSON,
		TimelineStartMS:      artifact.TimelineStartMS,
		TimelineEndMS:        artifact.TimelineEndMS,
		Probe:                probe,
	}, nil
}

func startupContinuationActiveKey(
	mediaID,
	profileID,
	fingerprint string,
	startMS int64,
	encodingPlanHash,
	timestampPlanHash string,
) string {
	return stableHash(strings.Join([]string{
		mediaID,
		string(transcodedomain.IntentStartupContinuationHLS),
		profileID,
		fmt.Sprintf("%d", startMS),
		fingerprint,
		startupContinuationPlannerVersion,
		encodingPlanHash,
		timestampPlanHash,
	}, "|"))
}

// SubmitStartupContinuation creates an interactive, durable continuation from
// the exact end of the immutable startup artifact. It never writes into the
// startup artifact directory and persists both shared plan identities plus the
// Job-owned continuation timeline origin.
func (s *TranscodeService) SubmitStartupContinuation(
	media *model.Media,
	startup *StartupStreamDescriptor,
) (*model.TranscodeTask, error) {
	if s == nil || s.executionRepo == nil || s.jobs == nil || media == nil || startup == nil {
		return nil, fmt.Errorf("startup continuation inputs are incomplete")
	}
	if startup.MediaID != media.ID || startup.ProfileID == "" || startup.SourceFingerprint == "" || startup.Probe == nil {
		return nil, fmt.Errorf("startup descriptor does not match media")
	}
	if _, err := decodeStartupDescriptorAttestation(startup); err != nil {
		return nil, fmt.Errorf("startup descriptor attestation is invalid: %w", err)
	}
	expectedEncodingIdentity, err := startupEncodingIdentity(startup.Probe, startup.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("rebuild continuation encoding plan: %w", err)
	}
	if !sameEncodingPlan(
		startup.EncodingPlanVersion,
		startup.EncodingPlanHash,
		startup.EncodingPlanJSON,
		expectedEncodingIdentity.Version,
		expectedEncodingIdentity.Hash,
		expectedEncodingIdentity.Canonical,
	) {
		return nil, fmt.Errorf("startup descriptor encoding plan is stale")
	}
	expectedTimestampIdentity, err := startupTimestampIdentity()
	if err != nil {
		return nil, fmt.Errorf("rebuild continuation timestamp plan: %w", err)
	}
	if !sameTimestampPlan(
		startup.TimestampPlanVersion,
		startup.TimestampPlanHash,
		startup.TimestampPlanJSON,
		expectedTimestampIdentity.Version,
		expectedTimestampIdentity.Hash,
		expectedTimestampIdentity.Canonical,
	) || startup.TimelineOriginMS != 0 {
		return nil, fmt.Errorf("startup descriptor timestamp plan is stale")
	}
	startMS := startup.DurationMS
	if startMS <= 0 {
		startMS = startupStreamDurationMS
	}
	if startup.Probe.DurationMS > 0 && startup.Probe.DurationMS <= startMS {
		return nil, gorm.ErrRecordNotFound
	}
	activeKey := startupContinuationActiveKey(
		media.ID,
		startup.ProfileID,
		startup.SourceFingerprint,
		startMS,
		startup.EncodingPlanHash,
		startup.TimestampPlanHash,
	)

	if artifact, findErr := s.executionRepo.FindPublishedArtifactByExecutionContract(
		media.ID,
		startup.ProfileID,
		startup.SourceFingerprint,
		startupContinuationPlannerVersion,
		startupContinuationArtifactKind,
		startup.EncodingPlanVersion,
		startup.EncodingPlanHash,
		startup.TimestampPlanVersion,
		startup.TimestampPlanHash,
		startMS,
	); findErr == nil && artifact != nil && sameEncodingPlan(
		artifact.EncodingPlanVersion,
		artifact.EncodingPlanHash,
		artifact.EncodingPlanJSON,
		startup.EncodingPlanVersion,
		startup.EncodingPlanHash,
		startup.EncodingPlanJSON,
	) && sameTimestampPlan(
		artifact.TimestampPlanVersion,
		artifact.TimestampPlanHash,
		artifact.TimestampPlanJSON,
		startup.TimestampPlanVersion,
		startup.TimestampPlanHash,
		startup.TimestampPlanJSON,
	) && artifact.TimelineOriginMS == startMS {
		if continuationEvidence, evidenceErr := decodeArtifactAttestation(artifact); evidenceErr == nil {
			if startupEvidence, startupErr := decodeStartupDescriptorAttestation(startup); startupErr == nil && transcodeattestation.BridgeCompatible(startupEvidence, continuationEvidence) == nil {
				if artifact.ManifestPath != "" {
					if _, statErr := os.Stat(artifact.ManifestPath); statErr == nil {
						return completedCompatibilityTask(media, startup.ProfileID, artifact, startupContinuationPriority), nil
					}
				}
			}
		}
	}
	if active, findErr := s.executionRepo.FindActiveByKey(activeKey); findErr == nil && active.LegacyTaskID != nil {
		if task, taskErr := s.repo.FindByID(*active.LegacyTaskID); taskErr == nil {
			return task, nil
		}
	}
	if !s.jobs.CanAccept() {
		return nil, fmt.Errorf("transcode queue is full or shutting down")
	}

	s.submitMu.Lock()
	defer s.submitMu.Unlock()
	if active, findErr := s.executionRepo.FindActiveByKey(activeKey); findErr == nil && active.LegacyTaskID != nil {
		if task, taskErr := s.repo.FindByID(*active.LegacyTaskID); taskErr == nil {
			return task, nil
		}
	}

	remainingMS := int64(0)
	if startup.Probe.DurationMS > startMS {
		remainingMS = startup.Probe.DurationMS - startMS
	}
	task := &model.TranscodeTask{
		MediaID:    media.ID,
		Status:     "pending",
		Quality:    startup.ProfileID,
		OutputDir:  s.GetLegacyOutputDir(media.ID, startup.ProfileID),
		MediaTitle: media.DescriptiveTitle(),
		Priority:   startupContinuationPriority,
		MaxRetry:   2,
	}
	if err := s.repo.Create(task); err != nil {
		return nil, err
	}
	legacyID := task.ID
	now := time.Now()
	record := &model.TranscodeJobRecord{
		LegacyTaskID:      &legacyID,
		MediaID:           media.ID,
		Intent:            string(transcodedomain.IntentStartupContinuationHLS),
		ProfileID:         startup.ProfileID,
		AudioTrack:        -1,
		StartMS:           startMS,
		DurationMS:        remainingMS,
		Priority:          startupContinuationPriority,
		Status:            "queued",
		DesiredState:      "running",
		ActiveKey:         &activeKey,
		SourceFingerprint: startup.SourceFingerprint,
		PlanHash: stableHash(strings.Join([]string{
			startupContinuationPlannerVersion,
			startup.ProfileID,
			fmt.Sprintf("%d", startMS),
			"none",
			startup.EncodingPlanHash,
			startup.TimestampPlanHash,
		}, "|")),
		PlannerVersion:       startupContinuationPlannerVersion,
		EncodingPlanVersion:  startup.EncodingPlanVersion,
		EncodingPlanHash:     startup.EncodingPlanHash,
		EncodingPlanJSON:     startup.EncodingPlanJSON,
		TimestampPlanVersion: startup.TimestampPlanVersion,
		TimestampPlanHash:    startup.TimestampPlanHash,
		TimestampPlanJSON:    startup.TimestampPlanJSON,
		TimelineOriginMS:     startMS,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := s.executionRepo.CreateJob(record); err != nil {
		_ = s.repo.DeleteByID(task.ID)
		return nil, fmt.Errorf("create startup continuation job: %w", err)
	}
	if !s.jobs.Push(&TranscodeJob{Task: task, ExecutionJob: record}) {
		completedAt := time.Now()
		_, _ = s.executionRepo.CompleteQueuedJob(record.ID, "failed", completedAt)
		task.Status = "failed"
		task.Error = "transcode queue is full or shutting down"
		task.CompletedAt = &completedAt
		_ = s.repo.Update(task)
		return nil, fmt.Errorf("transcode queue is full or shutting down")
	}
	if s.logger != nil {
		s.logger.Infof(
			"已提交 Startup Continuation job=%s media=%s profile=%s start=%dms encoding_plan=%s timestamp_plan=%s origin=%dms",
			record.ID,
			media.ID,
			startup.ProfileID,
			startMS,
			startup.EncodingPlanHash,
			startup.TimestampPlanHash,
			startMS,
		)
	}
	return task, nil
}

func completedCompatibilityTask(
	media *model.Media,
	profileID string,
	artifact *model.TranscodeArtifactRecord,
	priority int,
) *model.TranscodeTask {
	return &model.TranscodeTask{
		MediaID:    media.ID,
		Status:     "done",
		Quality:    profileID,
		Progress:   100,
		OutputDir:  artifact.Path,
		MediaTitle: media.DescriptiveTitle(),
		Priority:   priority,
	}
}

func (s *TranscodeService) ResolveReadableStartupContinuation(
	media *model.Media,
	startup *StartupStreamDescriptor,
) (*model.TranscodeArtifactRecord, error) {
	if s == nil || s.executionRepo == nil || media == nil || startup == nil {
		return nil, gorm.ErrRecordNotFound
	}
	startMS := startup.DurationMS
	if startMS <= 0 {
		startMS = startupStreamDurationMS
	}
	artifact, err := s.executionRepo.FindReadableArtifactByExecutionContract(
		media.ID,
		startup.ProfileID,
		startup.SourceFingerprint,
		startupContinuationPlannerVersion,
		startupContinuationArtifactKind,
		startup.EncodingPlanVersion,
		startup.EncodingPlanHash,
		startup.TimestampPlanVersion,
		startup.TimestampPlanHash,
		startMS,
		time.Now(),
	)
	if err != nil {
		if !errorsIsRecordNotFound(err) {
			return nil, err
		}
		artifact, err = s.executionRepo.FindActiveArtifactByExecutionContractForAttestation(
			media.ID,
			startup.ProfileID,
			startup.SourceFingerprint,
			startupContinuationPlannerVersion,
			startupContinuationArtifactKind,
			startup.EncodingPlanVersion,
			startup.EncodingPlanHash,
			startup.TimestampPlanVersion,
			startup.TimestampPlanHash,
			startMS,
			time.Now(),
		)
		if err != nil {
			return nil, err
		}
		if err := s.ensureProvisionalArtifactAttestation(artifact); err != nil {
			return nil, err
		}
	}
	if !sameEncodingPlan(
		artifact.EncodingPlanVersion,
		artifact.EncodingPlanHash,
		artifact.EncodingPlanJSON,
		startup.EncodingPlanVersion,
		startup.EncodingPlanHash,
		startup.EncodingPlanJSON,
	) || !sameTimestampPlan(
		artifact.TimestampPlanVersion,
		artifact.TimestampPlanHash,
		artifact.TimestampPlanJSON,
		startup.TimestampPlanVersion,
		startup.TimestampPlanHash,
		startup.TimestampPlanJSON,
	) || artifact.TimelineOriginMS != startMS {
		return nil, gorm.ErrRecordNotFound
	}
	startupEvidence, err := decodeStartupDescriptorAttestation(startup)
	if err != nil {
		return nil, gorm.ErrRecordNotFound
	}
	continuationEvidence, err := decodeArtifactAttestation(artifact)
	if err != nil {
		return nil, gorm.ErrRecordNotFound
	}
	if err := transcodeattestation.BridgeCompatible(startupEvidence, continuationEvidence); err != nil {
		return nil, gorm.ErrRecordNotFound
	}
	return artifact, nil
}

func decodeStartupDescriptorAttestation(startup *StartupStreamDescriptor) (transcodeattestation.Attestation, error) {
	if startup == nil {
		return transcodeattestation.Attestation{}, fmt.Errorf("startup descriptor is nil")
	}
	return decodeArtifactAttestation(&model.TranscodeArtifactRecord{
		EncodingPlanVersion:  startup.EncodingPlanVersion,
		EncodingPlanHash:     startup.EncodingPlanHash,
		EncodingPlanJSON:     startup.EncodingPlanJSON,
		TimestampPlanVersion: startup.TimestampPlanVersion,
		TimestampPlanHash:    startup.TimestampPlanHash,
		TimestampPlanJSON:    startup.TimestampPlanJSON,
		TimelineOriginMS:     startup.TimelineOriginMS,
		AttestationVersion:   startup.AttestationVersion,
		AttestationStatus:    artifactAttestationVerified,
		AttestationHash:      startup.AttestationHash,
		AttestationJSON:      startup.AttestationJSON,
		TimelineStartMS:      startup.TimelineStartMS,
		TimelineEndMS:        startup.TimelineEndMS,
	})
}

func errorsIsRecordNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
