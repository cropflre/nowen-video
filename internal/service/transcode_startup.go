package service

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodedomain "github.com/nowen-video/nowen-video/internal/transcode/domain"
	transcodeprofile "github.com/nowen-video/nowen-video/internal/transcode/profile"
	"gorm.io/gorm"
)

const (
	startupStreamPlannerVersion = "startup-hls-v1"
	startupStreamArtifactKind   = "startup_hls"
	startupStreamDurationMS     = int64(30_000)
	startupStreamPriority       = 10
)

// StartupStreamEligible selects sources that require video conversion for at
// least one first-class client. H.264 8-bit SDR remains Direct/Remux/Smart
// Remux and does not consume startup storage. HEVC, HDR, high-bit-depth H.264
// and legacy codecs receive a universal H.264/AAC startup artifact.
func StartupStreamEligible(media *model.Media, probe *model.MediaProbeRecord) bool {
	if media == nil || probe == nil || strings.TrimSpace(media.ID) == "" {
		return false
	}
	path := strings.ToLower(strings.TrimSpace(media.FilePath))
	if path == "" || strings.HasSuffix(path, ".strm") || IsWebDAVPath(media.FilePath) {
		return false
	}
	if probe.DurationMS > 0 && probe.DurationMS <= int64(hlsTargetSegmentSeconds*1000) {
		return false
	}
	codec := strings.ToLower(strings.TrimSpace(probe.VideoCodec))
	universalH264 := codec == "h264" || codec == "avc" || codec == "avc1"
	return !universalH264 || probe.BitDepth > 8 || probe.HDR
}

func startupStreamProfile(probe *model.MediaProbeRecord) string {
	height := 720
	if probe != nil && probe.Height > 0 && probe.Height < height {
		height = probe.Height
	}
	profiles := transcodeprofile.NamesUpToHeight(height)
	if len(profiles) == 0 {
		return "360p"
	}
	return profiles[len(profiles)-1]
}

func startupStreamActiveKey(media *model.Media, probe *model.MediaProbeRecord, quality string) string {
	fingerprint := transcodeSourceFingerprint(media)
	if probe != nil && probe.SourceFingerprint != "" {
		fingerprint = probe.SourceFingerprint
	}
	return stableHash(strings.Join([]string{
		media.ID,
		string(transcodedomain.IntentStartupHLS),
		quality,
		fmt.Sprintf("%d", startupStreamDurationMS),
		fingerprint,
		startupStreamPlannerVersion,
	}, "|"))
}

// SubmitStartupStream creates a normal durable Job. It is claimed, leased,
// cancelled, retried and published through the same orchestrator as runtime
// HLS. The scan warmup path only calls this after a successful authoritative
// Probe, so playback never guesses startup eligibility from legacy summaries.
func (s *TranscodeService) SubmitStartupStream(media *model.Media, probe *model.MediaProbeRecord) (*model.TranscodeTask, error) {
	if s == nil || s.executionRepo == nil || s.jobs == nil {
		return nil, fmt.Errorf("transcode service is unavailable")
	}
	if !StartupStreamEligible(media, probe) {
		return nil, gorm.ErrRecordNotFound
	}
	quality := startupStreamProfile(probe)
	fingerprint := transcodeSourceFingerprint(media)
	if probe.SourceFingerprint != "" {
		fingerprint = probe.SourceFingerprint
	}
	if artifact, err := s.executionRepo.FindPublishedArtifact(
		media.ID,
		quality,
		fingerprint,
		startupStreamPlannerVersion,
		startupStreamArtifactKind,
	); err == nil && artifact != nil && artifact.ManifestPath != "" {
		if _, statErr := os.Stat(artifact.ManifestPath); statErr == nil {
			if task, taskErr := s.repo.FindByID(dereferenceString(artifactLegacyTaskID(s, artifact.JobID))); taskErr == nil {
				return task, nil
			}
			return &model.TranscodeTask{
				MediaID:    media.ID,
				Status:     "done",
				Quality:    quality,
				Progress:   100,
				OutputDir:  artifact.Path,
				MediaTitle: media.DescriptiveTitle(),
				Priority:   startupStreamPriority,
			}, nil
		}
	}

	activeKey := startupStreamActiveKey(media, probe, quality)
	if active, err := s.executionRepo.FindActiveByKey(activeKey); err == nil && active.LegacyTaskID != nil {
		if task, taskErr := s.repo.FindByID(*active.LegacyTaskID); taskErr == nil {
			return task, nil
		}
	}
	if !s.jobs.CanAccept() {
		return nil, fmt.Errorf("transcode queue is full or shutting down")
	}

	s.submitMu.Lock()
	defer s.submitMu.Unlock()
	// Recheck under the submission mutex to close concurrent scan/playback
	// submissions for the same source identity.
	if active, err := s.executionRepo.FindActiveByKey(activeKey); err == nil && active.LegacyTaskID != nil {
		if task, taskErr := s.repo.FindByID(*active.LegacyTaskID); taskErr == nil {
			return task, nil
		}
	}

	task := &model.TranscodeTask{
		MediaID:    media.ID,
		Status:     "pending",
		Quality:    quality,
		OutputDir:  s.GetLegacyOutputDir(media.ID, quality),
		MediaTitle: media.DescriptiveTitle(),
		Priority:   startupStreamPriority,
		MaxRetry:   2,
	}
	if err := s.repo.Create(task); err != nil {
		return nil, err
	}
	legacyID := task.ID
	planHash := stableHash(strings.Join([]string{
		startupStreamPlannerVersion,
		quality,
		fmt.Sprintf("%d", startupStreamDurationMS),
		normalizeAttemptBackend(s.hwAccel),
	}, "|"))
	now := time.Now()
	record := &model.TranscodeJobRecord{
		LegacyTaskID:      &legacyID,
		MediaID:           media.ID,
		Intent:            string(transcodedomain.IntentStartupHLS),
		ProfileID:         quality,
		AudioTrack:        -1,
		StartMS:           0,
		DurationMS:        startupStreamDurationMS,
		Priority:          startupStreamPriority,
		Status:            "queued",
		DesiredState:      "running",
		ActiveKey:         &activeKey,
		SourceFingerprint: fingerprint,
		PlanHash:          planHash,
		PlannerVersion:    startupStreamPlannerVersion,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.executionRepo.CreateJob(record); err != nil {
		_ = s.repo.DeleteByID(task.ID)
		return nil, fmt.Errorf("create startup stream job: %w", err)
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
		s.logger.Infof("已提交 Startup Stream job=%s media=%s profile=%s duration=%dms", record.ID, media.ID, quality, startupStreamDurationMS)
	}
	return task, nil
}

func artifactLegacyTaskID(s *TranscodeService, jobID string) *string {
	if s == nil || s.executionRepo == nil || jobID == "" {
		return nil
	}
	job, err := s.executionRepo.FindJobByID(jobID)
	if err != nil {
		return nil
	}
	return job.LegacyTaskID
}

func dereferenceString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func startupStreamOutputArgs(args []string, durationMS int64) []string {
	if durationMS <= 0 || len(args) == 0 {
		return args
	}
	// Runtime HLS is an appendable EVENT playlist. Startup HLS is a bounded,
	// immutable VOD artifact, so remove those options rather than relying on a
	// duplicate later option to override them.
	body := make([]string, 0, len(args)+2)
	for index := 0; index < len(args)-1; index++ {
		arg := args[index]
		if arg == "-hls_playlist_type" && index+1 < len(args)-1 {
			index++
			continue
		}
		if arg == "-hls_flags" && index+1 < len(args)-1 {
			flags := strings.Split(args[index+1], "+")
			kept := make([]string, 0, len(flags))
			for _, flag := range flags {
				if flag != "append_list" {
					kept = append(kept, flag)
				}
			}
			body = append(body, arg, strings.Join(kept, "+"))
			index++
			continue
		}
		body = append(body, arg)
	}
	seconds := float64(durationMS) / 1000
	body = append(body,
		"-t", fmt.Sprintf("%.3f", seconds),
		"-hls_playlist_type", "vod",
	)
	return append(body, args[len(args)-1])
}
