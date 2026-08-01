package service

import (
	"encoding/json"
	"fmt"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	transcodedomain "github.com/nowen-video/nowen-video/internal/transcode/domain"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

type timestampPlanIdentity struct {
	Version   string
	Hash      string
	Canonical string
}

func startupTimestampIdentity() (timestampPlanIdentity, error) {
	version, hash, canonical, err := transcodetimestamp.Identity(transcodetimestamp.Default())
	if err != nil {
		return timestampPlanIdentity{}, err
	}
	return timestampPlanIdentity{Version: version, Hash: hash, Canonical: canonical}, nil
}

func sameTimestampPlan(
	actualVersion,
	actualHash,
	actualJSON,
	expectedVersion,
	expectedHash,
	expectedJSON string,
) bool {
	if actualVersion == "" || actualHash == "" || actualJSON == "" ||
		expectedVersion == "" || expectedHash == "" || expectedJSON == "" ||
		actualVersion != expectedVersion || actualHash != expectedHash || actualJSON != expectedJSON {
		return false
	}
	var plan transcodetimestamp.Plan
	if err := json.Unmarshal([]byte(actualJSON), &plan); err != nil {
		return false
	}
	version, hash, canonical, err := transcodetimestamp.Identity(plan)
	return err == nil && version == actualVersion && hash == actualHash && canonical == actualJSON
}

func timestampNormalizationRequired(record *model.TranscodeJobRecord) bool {
	if record == nil {
		return false
	}
	switch transcodedomain.Intent(record.Intent) {
	case transcodedomain.IntentStartupHLS, transcodedomain.IntentStartupContinuationHLS:
		return record.PlannerVersion == startupStreamPlannerVersion || record.PlannerVersion == startupContinuationPlannerVersion
	default:
		return false
	}
}

func validateTimestampExecution(record *model.TranscodeJobRecord, backend string) (transcodetimestamp.Plan, error) {
	if !timestampNormalizationRequired(record) {
		return transcodetimestamp.Plan{}, nil
	}
	if record.TimestampPlanVersion == "" || record.TimestampPlanHash == "" || record.TimestampPlanJSON == "" {
		return transcodetimestamp.Plan{}, fmt.Errorf("timestamp normalization identity is missing")
	}
	if record.TimelineOriginMS < 0 || record.TimelineOriginMS != record.StartMS {
		return transcodetimestamp.Plan{}, fmt.Errorf("timeline origin does not match job start")
	}
	var plan transcodetimestamp.Plan
	if err := json.Unmarshal([]byte(record.TimestampPlanJSON), &plan); err != nil {
		return transcodetimestamp.Plan{}, fmt.Errorf("decode timestamp plan: %w", err)
	}
	version, hash, canonical, err := transcodetimestamp.Identity(plan)
	if err != nil {
		return transcodetimestamp.Plan{}, err
	}
	if version != record.TimestampPlanVersion || hash != record.TimestampPlanHash || canonical != record.TimestampPlanJSON {
		return transcodetimestamp.Plan{}, fmt.Errorf("timestamp plan identity is invalid")
	}
	if !plan.SupportsBackend(normalizeAttemptBackend(backend)) {
		return transcodetimestamp.Plan{}, fmt.Errorf("timestamp plan %s does not certify backend %s", plan.SchemaVersion, normalizeAttemptBackend(backend))
	}
	return plan, nil
}

func (s *TranscodeService) preferredAttemptBackend(job *TranscodeJob) string {
	backend := normalizeAttemptBackend(s.hwAccel)
	if job != nil && timestampNormalizationRequired(job.ExecutionJob) {
		// v1 has only been certified for software encoding. Hardware remains an
		// execution candidate for ordinary Runtime HLS, but cannot produce a
		// Startup/Continuation pair under this timestamp contract.
		return ffmpeg.HWAccelNone
	}
	return backend
}

func applyTimestampNormalization(args []string, plan transcodetimestamp.Plan) []string {
	normalized, err := transcodetimestamp.ApplyFFmpeg(args, plan)
	if err != nil {
		// The checked execution path validates the persisted plan and receives a
		// complete BuildHLSArgs vector before this adapter is called. Returning nil
		// keeps malformed direct test callers fail closed without duplicating the
		// canonical command policy in the service package.
		return nil
	}
	return normalized
}

func timestampPlanCommandSummary(args []string) string {
	return transcodetimestamp.CommandSummary(args)
}
