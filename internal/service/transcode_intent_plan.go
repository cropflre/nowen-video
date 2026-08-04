package service

import (
	"fmt"

	"github.com/nowen-video/nowen-video/internal/model"
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
	return args, nil
}

// The durable executor no longer distinguishes runtime/startup Artifact kinds.
// Its remaining compatibility records use the historical generic kind while
// all executable runtime media is owned by Playback Session Generations.
func transcodeArtifactKind(_ *TranscodeJob) string {
	return "hls_variant"
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
// persistent playback scheduler. Runtime/startup/on-demand work executes only
// through PlaybackSessionService, while explicit administrator preprocessing
// remains owned by PreprocessService. No Job can be hydrated by this queue.
func supportedTranscodeIntent(_ *model.TranscodeJobRecord) bool {
	return false
}
