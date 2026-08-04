package service

import (
	"github.com/nowen-video/nowen-video/internal/model"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

// Runtime playback no longer executes through the durable transcode queue, so
// no persisted runtime Job can request timestamp command rewriting.
func timestampNormalizationRequired(_ *model.TranscodeJobRecord) bool {
	return false
}

func validateTimestampExecution(_ *model.TranscodeJobRecord, _ string) (transcodetimestamp.Plan, error) {
	return transcodetimestamp.Plan{}, nil
}

func (s *TranscodeService) preferredAttemptBackend(_ *TranscodeJob) string {
	return normalizeAttemptBackend(s.hwAccel)
}

// Generic adapters remain for value-object certification and historical
// Artifact verification. They are not selected by the durable runtime queue.
func applyTimestampNormalization(args []string, plan transcodetimestamp.Plan) []string {
	normalized, err := transcodetimestamp.ApplyFFmpeg(args, plan)
	if err != nil {
		return nil
	}
	return normalized
}

func timestampPlanCommandSummary(args []string) string {
	return transcodetimestamp.CommandSummary(args)
}
