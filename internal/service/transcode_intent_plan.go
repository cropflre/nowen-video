package service

import (
	"github.com/nowen-video/nowen-video/internal/model"
	transcodedomain "github.com/nowen-video/nowen-video/internal/transcode/domain"
)

func (s *TranscodeService) buildJobFFmpegArgs(job *TranscodeJob, outputDir, backend string) []string {
	if job == nil || job.Media == nil {
		return nil
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
	if job.ExecutionJob != nil && job.ExecutionJob.Intent == string(transcodedomain.IntentStartupHLS) {
		return startupStreamOutputArgs(args, job.ExecutionJob.DurationMS)
	}
	return args
}

func transcodeArtifactKind(job *TranscodeJob) string {
	if job != nil && job.ExecutionJob != nil && job.ExecutionJob.Intent == string(transcodedomain.IntentStartupHLS) {
		return startupStreamArtifactKind
	}
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

func supportedTranscodeIntent(record *model.TranscodeJobRecord) bool {
	if record == nil {
		return false
	}
	switch transcodedomain.Intent(record.Intent) {
	case transcodedomain.IntentRuntimeHLS, transcodedomain.IntentStartupHLS:
		return true
	default:
		return false
	}
}
