package certification

import (
	"encoding/json"
	"fmt"

	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
)

func (r BoundaryCaseReport) Validate() error {
	if err := r.Case.Validate(); err != nil {
		return err
	}
	expected, ok := LookupBoundaryCase(r.Case.ID)
	if !ok || r.Case != expected {
		return fmt.Errorf("boundary case %q does not match registry", r.Case.ID)
	}
	if err := r.Evidence.Validate(); err != nil {
		return err
	}
	if r.Evidence.CaseID != r.Case.ID || r.Evidence.FixtureID != r.Case.FixtureID || r.Evidence.ExpectedBoundaryMicros != r.Case.ExpectedBoundaryMicros {
		return fmt.Errorf("boundary evidence identity does not match case")
	}
	version, hash, _, err := transcodeboundary.Identity(r.Evidence)
	if err != nil {
		return err
	}
	if r.ContractVersion != version || r.ContractHash != hash {
		return fmt.Errorf("boundary evidence hash is invalid")
	}
	return nil
}

func BuildBoundaryMatrixReport(reports []BoundaryCaseReport) (BoundaryMatrixReport, error) {
	byID := make(map[string]BoundaryCaseReport, len(reports))
	for _, report := range reports {
		if err := report.Validate(); err != nil {
			return BoundaryMatrixReport{}, fmt.Errorf("validate boundary case %s: %w", report.Case.ID, err)
		}
		if _, exists := byID[report.Case.ID]; exists {
			return BoundaryMatrixReport{}, fmt.Errorf("duplicate boundary case %s", report.Case.ID)
		}
		byID[report.Case.ID] = report
	}
	ordered := make([]BoundaryCaseReport, 0, len(boundaryCaseSpecs))
	for _, spec := range boundaryCaseSpecs {
		report, ok := byID[spec.ID]
		if !ok {
			return BoundaryMatrixReport{}, fmt.Errorf("boundary matrix is missing case %s", spec.ID)
		}
		ordered = append(ordered, report)
	}
	matrix := BoundaryMatrixReport{SchemaVersion: BoundaryMatrixSchemaVersion, Cases: ordered}
	if err := matrix.Validate(); err != nil {
		return BoundaryMatrixReport{}, err
	}
	return matrix, nil
}

func (m BoundaryMatrixReport) Validate() error {
	if m.SchemaVersion != BoundaryMatrixSchemaVersion {
		return fmt.Errorf("unsupported boundary matrix schema %q", m.SchemaVersion)
	}
	if len(m.Cases) != len(boundaryCaseSpecs) {
		return fmt.Errorf("boundary matrix is incomplete")
	}
	seen := make(map[string]struct{}, len(m.Cases))
	ffmpegVersion := ""
	ffprobeVersion := ""
	for index, report := range m.Cases {
		if err := report.Validate(); err != nil {
			return err
		}
		if report.Case.ID != boundaryCaseSpecs[index].ID {
			return fmt.Errorf("boundary matrix case order is invalid")
		}
		if _, exists := seen[report.Case.ID]; exists {
			return fmt.Errorf("duplicate boundary case %s", report.Case.ID)
		}
		seen[report.Case.ID] = struct{}{}
		if ffmpegVersion == "" {
			ffmpegVersion = report.Evidence.FFmpegVersion
			ffprobeVersion = report.Evidence.FFprobeVersion
		} else if report.Evidence.FFmpegVersion != ffmpegVersion || report.Evidence.FFprobeVersion != ffprobeVersion {
			return fmt.Errorf("boundary matrix toolchain identity is inconsistent")
		}
	}
	return nil
}

func MarshalBoundaryMatrixReport(report BoundaryMatrixReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal boundary matrix: %w", err)
	}
	return append(content, '\n'), nil
}
