package certification

import (
	"encoding/json"
	"fmt"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
)

const OutputCadenceMatrixSchemaVersion = "ffmpeg-output-cadence-matrix-v1"

type OutputCadenceCaseReport struct {
	Case            SourceOriginCaseSpec            `json:"case"`
	SourceOrigin    SourceOriginCaseReport          `json:"source_origin"`
	ContractVersion string                          `json:"contract_version"`
	ContractHash    string                          `json:"contract_hash"`
	Evidence        transcodeoutputcadence.Contract `json:"evidence"`
}

type OutputCadenceMatrixReport struct {
	SchemaVersion string                    `json:"schema_version"`
	Cases         []OutputCadenceCaseReport `json:"cases"`
}

func (r OutputCadenceCaseReport) Validate() error {
	expected, ok := LookupSourceOriginCase(r.Case.ID)
	if !ok || r.Case != expected {
		return fmt.Errorf("output cadence case %q does not match registry", r.Case.ID)
	}
	if err := r.SourceOrigin.Validate(); err != nil {
		return fmt.Errorf("source origin evidence: %w", err)
	}
	if r.SourceOrigin.Case != r.Case {
		return fmt.Errorf("output cadence source-origin case identity is inconsistent")
	}
	if err := r.Evidence.Validate(); err != nil {
		return err
	}
	version, hash, _, err := transcodeoutputcadence.Identity(r.Evidence)
	if err != nil {
		return err
	}
	if r.ContractVersion != version || r.ContractHash != hash {
		return fmt.Errorf("output cadence evidence identity is invalid")
	}
	if r.Evidence.CaseID != r.Case.ID || r.Evidence.FixtureID != r.Case.FixtureID ||
		r.Evidence.SourceMode != r.Case.SourceMode ||
		r.Evidence.DeclaredFrameRateNumerator != r.Case.DeclaredFrameRateNumerator ||
		r.Evidence.DeclaredFrameRateDenominator != r.Case.DeclaredFrameRateDenominator ||
		r.Evidence.ExpectedBoundaryMicros != r.Case.ExpectedBoundaryMicros {
		return fmt.Errorf("output cadence evidence does not match registered case")
	}
	if r.Evidence.SourceOriginVersion != r.SourceOrigin.ContractVersion || r.Evidence.SourceOriginHash != r.SourceOrigin.ContractHash ||
		r.Evidence.TimestampPlanVersion != r.SourceOrigin.Evidence.TimestampPlanVersion || r.Evidence.TimestampPlanHash != r.SourceOrigin.Evidence.TimestampPlanHash ||
		r.Evidence.BoundaryEvidenceVersion != r.SourceOrigin.BoundaryVersion || r.Evidence.BoundaryEvidenceHash != r.SourceOrigin.BoundaryHash ||
		r.Evidence.AVSyncEvidenceVersion != r.SourceOrigin.AVSyncVersion || r.Evidence.AVSyncEvidenceHash != r.SourceOrigin.AVSyncHash {
		return fmt.Errorf("output cadence evidence does not bind its source and produced-media evidence")
	}
	if r.Evidence.FFmpegVersion != r.SourceOrigin.Evidence.FFmpegVersion || r.Evidence.FFprobeVersion != r.SourceOrigin.Evidence.FFprobeVersion {
		return fmt.Errorf("output cadence toolchain identity is inconsistent")
	}
	if r.Evidence.SeamlessAllowed || !r.Evidence.DiscontinuityRequired {
		return fmt.Errorf("output cadence certification attempted to authorize seamless playback")
	}
	return nil
}

func BuildOutputCadenceMatrixReport(cases []OutputCadenceCaseReport) (OutputCadenceMatrixReport, error) {
	report := OutputCadenceMatrixReport{
		SchemaVersion: OutputCadenceMatrixSchemaVersion,
		Cases:         append([]OutputCadenceCaseReport(nil), cases...),
	}
	if err := report.Validate(); err != nil {
		return OutputCadenceMatrixReport{}, err
	}
	return report, nil
}

func (r OutputCadenceMatrixReport) Validate() error {
	if r.SchemaVersion != OutputCadenceMatrixSchemaVersion {
		return fmt.Errorf("unsupported output cadence matrix schema %q", r.SchemaVersion)
	}
	if len(r.Cases) != len(sourceOriginCaseSpecs) {
		return fmt.Errorf("output cadence matrix is incomplete")
	}
	ffmpegVersion := ""
	ffprobeVersion := ""
	hashes := make(map[string]struct{}, len(r.Cases))
	for index, report := range r.Cases {
		if report.Case.ID != sourceOriginCaseSpecs[index].ID {
			return fmt.Errorf("output cadence case order is invalid")
		}
		if err := report.Validate(); err != nil {
			return fmt.Errorf("validate output cadence case %s: %w", report.Case.ID, err)
		}
		if _, exists := hashes[report.ContractHash]; exists {
			return fmt.Errorf("output cadence evidence hash is duplicated")
		}
		hashes[report.ContractHash] = struct{}{}
		if ffmpegVersion == "" {
			ffmpegVersion = report.Evidence.FFmpegVersion
			ffprobeVersion = report.Evidence.FFprobeVersion
		} else if report.Evidence.FFmpegVersion != ffmpegVersion || report.Evidence.FFprobeVersion != ffprobeVersion {
			return fmt.Errorf("output cadence matrix toolchain identity is inconsistent")
		}
	}
	return nil
}

func MarshalOutputCadenceMatrixReport(report OutputCadenceMatrixReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal output cadence matrix: %w", err)
	}
	return append(content, '\n'), nil
}
