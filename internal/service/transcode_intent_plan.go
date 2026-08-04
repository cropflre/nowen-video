package service

import (
	"fmt"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodedomain "github.com/nowen-video/nowen-video/internal/transcode/domain"
)

// buildJobFFmpegArgs remains source-compatible for focused argument tests.
// Attempt execution uses the checked variant so a malformed timestamp contract
// is rejected before an FFmpeg process or Artifact workspace is created.
func (s *TranscodeService) buildJobFFmpegArgs(job *TranscodeJob, outputDir, backend string) []string {
	args, _ := s.buildJobFFmpegArgsChecked(job, outputDir, backend)
	return args
}

func (s *TranscodeService) buildJobFFmpegArgsChecked(job *TranscodeJob, outputDir, backend string) ([]string, error) {
	if job == nil || job.Media == nil {
		return nil, fmt.Errorf("transcode job media is missing")
	}
	args := s.buildFFmpegArgsForBackendWithProbe(
		job.Media,
		job.Probe,
		job.Media.FilePath,
		outputDir,
		job.Quality,
		job.startOffset,
		backend,
	)
	if timestampNormalizationRequired(job.ExecutionJob) {
		plan, err := validateTimestampExecution(job.ExecutionJob, backend)
		if err != nil {
			return nil, err
		}
		args = applyTimestampNormalization(args, plan)
	}
	if job.ExecutionJob != nil && job.ExecutionJob.Intent == string(transcodedomain.IntentStartupHLS) {
		args = startupStreamOutputArgs(args, job.ExecutionJob.DurationMS)
	}
	return args, nil
}

func transcodeArtifactKind(job *TranscodeJob) string {
	if job == nil || job.ExecutionJob == nil {
		return "hls_variant"
	}
	switch transcodedomain.Intent(job.ExecutionJob.Intent) {
	case transcodedomain.IntentStartupHLS:
		return startupStreamArtifactKind
	case transcodedomain.IntentStartupContinuationHLS:
		return startupContinuationArtifactKind
	default:
		return "hls_variant"
	}
}

func transcodeArtifactDurationMS(job *TranscodeJob) int64 {
	if job == nil {
		return 0
	}
	if job.ExecutionJob != nil && job.ExecutionJob.DurationMS > 0 {
		return job.ExecutionJob.DurationMS
	}
	if job.Probe != nil && job.Probe.DurationMS > 0 {
		return job.Probe.DurationMS
	}
	if job.Media != nil && job.Media.Duration > 0 {
		return int64(job.Media.Duration * 1000)
	}
	return 0
}

// supportedTranscodeIntent is the final execution fence for the retired
// persistent playback scheduler. Runtime/startup/on-demand work must execute
// through PlaybackSessionService, which owns its process and temporary files.
// Explicit administrator preprocessing uses PreprocessService and is not
// hydrated through this queue.
func supportedTranscodeIntent(record *model.TranscodeJobRecord) bool {
	if record == nil {
		return false
	}
	return !isRetiredRuntimePlaybackIntent(record.Intent) && record.Intent != retiredRuntimePlaybackIntent
}
