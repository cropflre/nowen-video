package certification

import (
	"encoding/json"
	"fmt"

	transcodeavsync "github.com/nowen-video/nowen-video/internal/transcode/avsync"
	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
)

type SourceOriginCaseReport struct {
	Case                  SourceOriginCaseSpec           `json:"case"`
	ContractVersion       string                         `json:"contract_version"`
	ContractHash          string                         `json:"contract_hash"`
	Evidence              transcodesourceorigin.Contract `json:"evidence"`
	BoundaryVersion       string                         `json:"boundary_version"`
	BoundaryHash          string                         `json:"boundary_hash"`
	Boundary              transcodeboundary.Contract     `json:"boundary"`
	AVSyncVersion         string                         `json:"av_sync_version"`
	AVSyncHash            string                         `json:"av_sync_hash"`
	AVSync                transcodeavsync.Contract        `json:"av_sync"`
}

type SourceOriginMatrixReport struct {
	SchemaVersion string                   `json:"schema_version"`
	Cases         []SourceOriginCaseReport `json:"cases"`
}

func (r SourceOriginCaseReport) Validate() error {
	expected, ok := LookupSourceOriginCase(r.Case.ID)
	if !ok || r.Case != expected {
		return fmt.Errorf("source origin case %q does not match registry", r.Case.ID)
	}
	if err := r.Evidence.Validate(); err != nil {
		return err
	}
	version, hash, _, err := transcodesourceorigin.Identity(r.Evidence)
	if err != nil {
		return err
	}
	if r.ContractVersion != version || r.ContractHash != hash {
		return fmt.Errorf("source origin evidence identity is invalid")
	}
	if err := r.Boundary.Validate(); err != nil {
		return err
	}
	boundaryVersion, boundaryHash, _, err := transcodeboundary.Identity(r.Boundary)
	if err != nil {
		return err
	}
	if r.BoundaryVersion != boundaryVersion || r.BoundaryHash != boundaryHash {
		return fmt.Errorf("source origin boundary identity is invalid")
	}
	if err := r.AVSync.ValidateAgainst(r.Boundary); err != nil {
		return err
	}
	avSyncVersion, avSyncHash, _, err := transcodeavsync.Identity(r.AVSync)
	if err != nil {
		return err
	}
	if r.AVSyncVersion != avSyncVersion || r.AVSyncHash != avSyncHash {
		return fmt.Errorf("source origin A/V sync identity is invalid")
	}
	if r.Evidence.CaseID != r.Case.ID || r.Evidence.FixtureID != r.Case.FixtureID ||
		r.Evidence.SourceMode != r.Case.SourceMode ||
		r.Evidence.DeclaredFrameRateNumerator != r.Case.DeclaredFrameRateNumerator ||
		r.Evidence.DeclaredFrameRateDenominator != r.Case.DeclaredFrameRateDenominator ||
		r.Evidence.SourceOffsetMicros != r.Case.SourceOffsetMicros ||
		r.Evidence.ExpectedBoundaryMicros != r.Case.ExpectedBoundaryMicros {
		return fmt.Errorf("source origin evidence does not match registered case")
	}
	if r.Evidence.BoundaryEvidenceVersion != r.BoundaryVersion || r.Evidence.BoundaryEvidenceHash != r.BoundaryHash ||
		r.Evidence.AVSyncEvidenceVersion != r.AVSyncVersion || r.Evidence.AVSyncEvidenceHash != r.AVSyncHash {
		return fmt.Errorf("source origin evidence does not bind produced-media evidence")
	}
	if r.Boundary.CaseID != r.Case.ID || r.Boundary.FixtureID != r.Case.FixtureID ||
		r.AVSync.CaseID != r.Case.ID || r.AVSync.FixtureID != r.Case.FixtureID {
		return fmt.Errorf("source origin produced-media case identity is inconsistent")
	}
	if r.Evidence.SeamlessAllowed || !r.Evidence.DiscontinuityRequired ||
		r.Boundary.SeamlessAllowed || !r.Boundary.DiscontinuityRequired ||
		r.AVSync.SeamlessAllowed || !r.AVSync.DiscontinuityRequired {
		return fmt.Errorf("source origin certification attempted to authorize seamless playback")
	}
	return nil
}

func BuildSourceOriginMatrixReport(cases []SourceOriginCaseReport) (SourceOriginMatrixReport, error) {
	report := SourceOriginMatrixReport{
		SchemaVersion: SourceOriginMatrixSchemaVersion,
		Cases:         append([]SourceOriginCaseReport(nil), cases...),
	}
	if err := report.Validate(); err != nil {
		return SourceOriginMatrixReport{}, err
	}
	return report, nil
}

func (r SourceOriginMatrixReport) Validate() error {
	if r.SchemaVersion != SourceOriginMatrixSchemaVersion {
		return fmt.Errorf("unsupported source origin matrix schema %q", r.SchemaVersion)
	}
	if len(r.Cases) != len(sourceOriginCaseSpecs) {
		return fmt.Errorf("source origin matrix is incomplete")
	}
	ffmpegVersion := ""
	ffprobeVersion := ""
	for index, report := range r.Cases {
		if report.Case.ID != sourceOriginCaseSpecs[index].ID {
			return fmt.Errorf("source origin case order is invalid")
		}
		if err := report.Validate(); err != nil {
			return fmt.Errorf("validate source origin case %s: %w", report.Case.ID, err)
		}
		if ffmpegVersion == "" {
			ffmpegVersion = report.Evidence.FFmpegVersion
			ffprobeVersion = report.Evidence.FFprobeVersion
		} else if report.Evidence.FFmpegVersion != ffmpegVersion || report.Evidence.FFprobeVersion != ffprobeVersion {
			return fmt.Errorf("source origin matrix toolchain identity is inconsistent")
		}
	}
	return nil
}

func MarshalSourceOriginMatrixReport(report SourceOriginMatrixReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal source origin matrix: %w", err)
	}
	return append(content, '\n'), nil
}
