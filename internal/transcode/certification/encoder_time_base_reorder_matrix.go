package certification

import (
	"encoding/json"
	"fmt"

	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
)

const EncoderTimeBaseReorderMatrixSchemaVersion = "ffmpeg-encoder-time-base-reorder-matrix-v1"

const (
	EncoderTimeBaseReorderCFR24B2Zero       = "reorder-cfr-24-b2-origin-zero-v1"
	EncoderTimeBaseReorderCFR2997B3Zero     = "reorder-cfr-30000-1001-b3-origin-zero-v1"
	EncoderTimeBaseReorderVFR2430B3Zero     = "reorder-vfr-24-30-b3-origin-zero-v1"
	EncoderTimeBaseReorderCFR30B3Positive5S = "reorder-cfr-30-b3-origin-positive-5s-v1"
	EncoderTimeBaseReorderCFR30B3Negative2S = "reorder-cfr-30-b3-origin-negative-2s-v1"
	EncoderTimeBaseReorderCFR30B3LongGOP    = "reorder-cfr-30-b3-long-gop-origin-zero-v1"
)

var encoderTimeBaseReorderCaseSpecs = []transcodereorder.CaseSpec{
	newEncoderTimeBaseReorderCase(
		newEncoderTimeBaseCFRCase(EncoderTimeBaseReorderCFR24B2Zero, "24 fps CFR with two B-frames", 24, 1, 0, 48),
		2, 3,
	),
	newEncoderTimeBaseReorderCase(
		newEncoderTimeBaseCFRCase(EncoderTimeBaseReorderCFR2997B3Zero, "30000/1001 fps CFR with three B-frames", 30_000, 1_001, 0, 60),
		3, 4,
	),
	newEncoderTimeBaseReorderCase(
		newEncoderTimeBaseVFRCase(EncoderTimeBaseReorderVFR2430B3Zero, "24 to 30 fps VFR with three B-frames", 24, 1, 30, 1, 0, 60),
		3, 4,
	),
	newEncoderTimeBaseReorderCase(
		newEncoderTimeBaseCFRCase(EncoderTimeBaseReorderCFR30B3Positive5S, "30 fps CFR with three B-frames and positive five-second origin", 30, 1, 5_000_000, 60),
		3, 4,
	),
	newEncoderTimeBaseReorderCase(
		newEncoderTimeBaseCFRCase(EncoderTimeBaseReorderCFR30B3Negative2S, "30 fps CFR with three B-frames and negative two-second origin", 30, 1, -2_000_000, 60),
		3, 4,
	),
	newEncoderTimeBaseReorderCase(
		newEncoderTimeBaseCFRCase(EncoderTimeBaseReorderCFR30B3LongGOP, "30 fps CFR with three B-frames and configured ten-second GOP", 30, 1, 0, 300),
		3, 4,
	),
}

func newEncoderTimeBaseReorderCase(base transcodetimebase.CaseSpec, bFrames, references int) transcodereorder.CaseSpec {
	return transcodereorder.CaseSpec{
		Base:            base,
		BFrames:         bFrames,
		BAdapt:          0,
		ReferenceFrames: references,
		OpenGOP:         false,
	}
}

type EncoderTimeBaseReorderMatrixReport struct {
	SchemaVersion   string                    `json:"schema_version"`
	ContractVersion string                    `json:"contract_version"`
	ContractHash    string                    `json:"contract_hash"`
	Evidence        transcodereorder.Contract `json:"evidence"`
}

func AvailableEncoderTimeBaseReorderCases() []transcodereorder.CaseSpec {
	return append([]transcodereorder.CaseSpec(nil), encoderTimeBaseReorderCaseSpecs...)
}

func (r EncoderTimeBaseReorderMatrixReport) Validate() error {
	if r.SchemaVersion != EncoderTimeBaseReorderMatrixSchemaVersion {
		return fmt.Errorf("unsupported encoder time-base reorder matrix schema %q", r.SchemaVersion)
	}
	if err := r.Evidence.Validate(); err != nil {
		return err
	}
	version, hash, _, err := transcodereorder.Identity(r.Evidence)
	if err != nil {
		return err
	}
	if r.ContractVersion != version || r.ContractHash != hash {
		return fmt.Errorf("encoder time-base reorder contract identity is invalid")
	}
	if len(r.Evidence.Cases) != len(encoderTimeBaseReorderCaseSpecs) {
		return fmt.Errorf("encoder time-base reorder case matrix is incomplete")
	}
	for caseIndex, evidence := range r.Evidence.Cases {
		if evidence.Case != encoderTimeBaseReorderCaseSpecs[caseIndex] {
			return fmt.Errorf("encoder time-base reorder case order drifted at index %d", caseIndex)
		}
		if len(evidence.Candidates) != len(encoderTimeBaseCandidateSpecs) {
			return fmt.Errorf("encoder time-base reorder candidate matrix is incomplete for %s", evidence.Case.Base.ID)
		}
		for candidateIndex, candidate := range evidence.Candidates {
			if candidate.Spec != encoderTimeBaseCandidateSpecs[candidateIndex] {
				return fmt.Errorf("encoder time-base reorder candidate order drifted for %s", evidence.Case.Base.ID)
			}
		}
	}
	return nil
}

func MarshalEncoderTimeBaseReorderMatrixReport(report EncoderTimeBaseReorderMatrixReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal encoder time-base reorder matrix: %w", err)
	}
	return append(content, '\n'), nil
}
