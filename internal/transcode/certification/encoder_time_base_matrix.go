package certification

import (
	"encoding/json"
	"fmt"

	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
)

const EncoderTimeBaseMatrixSchemaVersion = "ffmpeg-encoder-time-base-candidate-matrix-v1"

const (
	EncoderTimeBaseCaseCFR23976Zero    = "candidate-cfr-24000-1001-origin-zero-v1"
	EncoderTimeBaseCaseCFR24Zero       = "candidate-cfr-24-origin-zero-v1"
	EncoderTimeBaseCaseCFR25Zero       = "candidate-cfr-25-origin-zero-v1"
	EncoderTimeBaseCaseCFR2997Zero     = "candidate-cfr-30000-1001-origin-zero-v1"
	EncoderTimeBaseCaseCFR30Zero       = "candidate-cfr-30-origin-zero-v1"
	EncoderTimeBaseCaseCFR50Zero       = "candidate-cfr-50-origin-zero-v1"
	EncoderTimeBaseCaseCFR5994Zero     = "candidate-cfr-60000-1001-origin-zero-v1"
	EncoderTimeBaseCaseVFR2430Zero     = "candidate-vfr-24-30-origin-zero-v1"
	EncoderTimeBaseCaseVFR2530Zero     = "candidate-vfr-25-30-origin-zero-v1"
	EncoderTimeBaseCaseVFR29975994Zero = "candidate-vfr-30000-1001-60000-1001-origin-zero-v1"
	EncoderTimeBaseCaseCFR30Positive5S = "candidate-cfr-30-origin-positive-5s-v1"
	EncoderTimeBaseCaseCFR30Negative2S = "candidate-cfr-30-origin-negative-2s-v1"
	encoderTimeBaseDurationMicros      = int64(40_000_000)
	encoderTimeBaseBoundaryMicros      = int64(30_000_000)
	encoderTimeBaseAudioSampleRate     = 48_000
)

var encoderTimeBaseCaseSpecs = []transcodetimebase.CaseSpec{
	newEncoderTimeBaseCFRCase(EncoderTimeBaseCaseCFR23976Zero, "24000/1001 fps CFR source at zero origin", 24_000, 1_001, 0, 48),
	newEncoderTimeBaseCFRCase(EncoderTimeBaseCaseCFR24Zero, "24 fps CFR source at zero origin", 24, 1, 0, 48),
	newEncoderTimeBaseCFRCase(EncoderTimeBaseCaseCFR25Zero, "25 fps CFR source at zero origin", 25, 1, 0, 50),
	newEncoderTimeBaseCFRCase(EncoderTimeBaseCaseCFR2997Zero, "30000/1001 fps CFR source at zero origin", 30_000, 1_001, 0, 60),
	newEncoderTimeBaseCFRCase(EncoderTimeBaseCaseCFR30Zero, "30 fps CFR source at zero origin", 30, 1, 0, 60),
	newEncoderTimeBaseCFRCase(EncoderTimeBaseCaseCFR50Zero, "50 fps CFR source at zero origin", 50, 1, 0, 100),
	newEncoderTimeBaseCFRCase(EncoderTimeBaseCaseCFR5994Zero, "60000/1001 fps CFR source at zero origin", 60_000, 1_001, 0, 120),
	newEncoderTimeBaseVFRCase(EncoderTimeBaseCaseVFR2430Zero, "VFR source with 24 fps then 30 fps cadence", 24, 1, 30, 1, 0, 60),
	newEncoderTimeBaseVFRCase(EncoderTimeBaseCaseVFR2530Zero, "VFR source with 25 fps then 30 fps cadence", 25, 1, 30, 1, 0, 60),
	newEncoderTimeBaseVFRCase(EncoderTimeBaseCaseVFR29975994Zero, "VFR source with 30000/1001 then 60000/1001 cadence", 30_000, 1_001, 60_000, 1_001, 0, 120),
	newEncoderTimeBaseCFRCase(EncoderTimeBaseCaseCFR30Positive5S, "30 fps CFR source with positive five-second origin", 30, 1, 5_000_000, 60),
	newEncoderTimeBaseCFRCase(EncoderTimeBaseCaseCFR30Negative2S, "30 fps CFR source with negative two-second origin", 30, 1, -2_000_000, 60),
}

var encoderTimeBaseCandidateSpecs = []transcodetimebase.CandidateSpec{
	{ID: transcodetimebase.CandidateAVTB, Description: "Explicit AVTB encoder time base", EncoderTimeBase: "1/1000000"},
	{ID: transcodetimebase.Candidate90K, Description: "Explicit MPEG-TS 90 kHz encoder time base", EncoderTimeBase: "1/90000"},
}

type EncoderTimeBaseMatrixReport struct {
	SchemaVersion   string                     `json:"schema_version"`
	ContractVersion string                     `json:"contract_version"`
	ContractHash    string                     `json:"contract_hash"`
	Evidence        transcodetimebase.Contract `json:"evidence"`
}

func newEncoderTimeBaseCFRCase(id, description string, numerator, denominator, offsetMicros int64, gopSize int) transcodetimebase.CaseSpec {
	return transcodetimebase.CaseSpec{
		ID: id, Description: description, SourceMode: transcodesourceorigin.ModeCFR,
		PrimaryFrameRateNumerator: numerator, PrimaryFrameRateDenominator: denominator,
		SourceOffsetMicros: offsetMicros, AudioSampleRate: encoderTimeBaseAudioSampleRate,
		GOPSize: gopSize, ExpectedBoundaryMicros: encoderTimeBaseBoundaryMicros, DurationMicros: encoderTimeBaseDurationMicros,
	}
}

func newEncoderTimeBaseVFRCase(id, description string, primaryNumerator, primaryDenominator, secondaryNumerator, secondaryDenominator, offsetMicros int64, gopSize int) transcodetimebase.CaseSpec {
	return transcodetimebase.CaseSpec{
		ID: id, Description: description, SourceMode: transcodesourceorigin.ModeVFR,
		PrimaryFrameRateNumerator: primaryNumerator, PrimaryFrameRateDenominator: primaryDenominator,
		SecondaryFrameRateNumerator: secondaryNumerator, SecondaryFrameRateDenominator: secondaryDenominator,
		SourceOffsetMicros: offsetMicros, AudioSampleRate: encoderTimeBaseAudioSampleRate,
		GOPSize: gopSize, ExpectedBoundaryMicros: encoderTimeBaseBoundaryMicros, DurationMicros: encoderTimeBaseDurationMicros,
	}
}

func AvailableEncoderTimeBaseCases() []transcodetimebase.CaseSpec {
	return append([]transcodetimebase.CaseSpec(nil), encoderTimeBaseCaseSpecs...)
}

func AvailableEncoderTimeBaseCandidates() []transcodetimebase.CandidateSpec {
	return append([]transcodetimebase.CandidateSpec(nil), encoderTimeBaseCandidateSpecs...)
}

func (r EncoderTimeBaseMatrixReport) Validate() error {
	if r.SchemaVersion != EncoderTimeBaseMatrixSchemaVersion {
		return fmt.Errorf("unsupported encoder time-base matrix schema %q", r.SchemaVersion)
	}
	if err := r.Evidence.Validate(); err != nil {
		return err
	}
	version, hash, _, err := transcodetimebase.Identity(r.Evidence)
	if err != nil {
		return err
	}
	if r.ContractVersion != version || r.ContractHash != hash {
		return fmt.Errorf("encoder time-base contract identity is invalid")
	}
	if len(r.Evidence.Cases) != len(encoderTimeBaseCaseSpecs) {
		return fmt.Errorf("encoder time-base case matrix is incomplete")
	}
	for caseIndex, evidence := range r.Evidence.Cases {
		if evidence.Case != encoderTimeBaseCaseSpecs[caseIndex] {
			return fmt.Errorf("encoder time-base case order or policy drifted at index %d", caseIndex)
		}
		if len(evidence.Candidates) != len(encoderTimeBaseCandidateSpecs) {
			return fmt.Errorf("encoder time-base candidate matrix is incomplete for %s", evidence.Case.ID)
		}
		for candidateIndex, candidate := range evidence.Candidates {
			if candidate.Spec != encoderTimeBaseCandidateSpecs[candidateIndex] {
				return fmt.Errorf("encoder time-base candidate order drifted for %s", evidence.Case.ID)
			}
		}
	}
	return nil
}

func MarshalEncoderTimeBaseMatrixReport(report EncoderTimeBaseMatrixReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal encoder time-base matrix: %w", err)
	}
	return append(content, '\n'), nil
}
