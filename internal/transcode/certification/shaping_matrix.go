package certification

import (
	"encoding/json"
	"fmt"

	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	timestampexecution "github.com/nowen-video/nowen-video/internal/transcode/timestampexecution"
)

func (r ShapingCaseReport) Validate() error {
	if err := r.Case.Validate(); err != nil {
		return err
	}
	expected, ok := LookupShapingCase(r.Case.ID)
	if !ok || r.Case != expected {
		return fmt.Errorf("shaping case %q does not match registry", r.Case.ID)
	}
	var plan timestampexecution.Plan
	if err := json.Unmarshal([]byte(r.PlanJSON), &plan); err != nil {
		return fmt.Errorf("decode timestamp execution plan: %w", err)
	}
	planVersion, planHash, planJSON, err := timestampexecution.Identity(plan)
	if err != nil {
		return err
	}
	if r.PlanVersion != planVersion || r.PlanHash != planHash || r.PlanJSON != planJSON {
		return fmt.Errorf("timestamp execution plan identity is invalid")
	}
	if plan.VideoPTSShiftMicros != r.Case.VideoPTSShiftMicros || plan.AudioPTSShiftMicros != r.Case.AudioPTSShiftMicros {
		return fmt.Errorf("timestamp execution plan does not match shaping case")
	}
	if err := r.Evidence.Validate(); err != nil {
		return err
	}
	if r.Evidence.CaseID != r.Case.ID || r.Evidence.FixtureID != r.Case.FixtureID || r.Evidence.ExpectedBoundaryMicros != r.Case.ExpectedBoundaryMicros {
		return fmt.Errorf("shaping evidence identity does not match case")
	}
	evidenceVersion, evidenceHash, _, err := transcodeboundary.Identity(r.Evidence)
	if err != nil {
		return err
	}
	if r.EvidenceVersion != evidenceVersion || r.EvidenceHash != evidenceHash {
		return fmt.Errorf("shaping evidence hash is invalid")
	}
	if r.Evidence.SeamlessAllowed || !r.Evidence.DiscontinuityRequired || plan.SeamlessAllowed || !plan.DiscontinuityRequired {
		return fmt.Errorf("shaping candidate attempted to authorize seamless handoff")
	}
	return nil
}

func BuildShapingMatrixReport(reports []ShapingCaseReport) (ShapingMatrixReport, error) {
	byID := make(map[string]ShapingCaseReport, len(reports))
	for _, report := range reports {
		if err := report.Validate(); err != nil {
			return ShapingMatrixReport{}, fmt.Errorf("validate shaping case %s: %w", report.Case.ID, err)
		}
		if _, exists := byID[report.Case.ID]; exists {
			return ShapingMatrixReport{}, fmt.Errorf("duplicate shaping case %s", report.Case.ID)
		}
		byID[report.Case.ID] = report
	}
	ordered := make([]ShapingCaseReport, 0, len(shapingCaseSpecs))
	for _, spec := range shapingCaseSpecs {
		report, ok := byID[spec.ID]
		if !ok {
			return ShapingMatrixReport{}, fmt.Errorf("shaping matrix is missing case %s", spec.ID)
		}
		ordered = append(ordered, report)
	}
	matrix := ShapingMatrixReport{SchemaVersion: ShapingMatrixSchemaVersion, Cases: ordered}
	if err := matrix.Validate(); err != nil {
		return ShapingMatrixReport{}, err
	}
	return matrix, nil
}

func (m ShapingMatrixReport) Validate() error {
	if m.SchemaVersion != ShapingMatrixSchemaVersion {
		return fmt.Errorf("unsupported shaping matrix schema %q", m.SchemaVersion)
	}
	if len(m.Cases) != len(shapingCaseSpecs) {
		return fmt.Errorf("shaping matrix is incomplete")
	}
	seen := make(map[string]struct{}, len(m.Cases))
	ffmpegVersion := ""
	ffprobeVersion := ""
	for index, report := range m.Cases {
		if err := report.Validate(); err != nil {
			return err
		}
		if report.Case.ID != shapingCaseSpecs[index].ID {
			return fmt.Errorf("shaping matrix case order is invalid")
		}
		if _, exists := seen[report.Case.ID]; exists {
			return fmt.Errorf("duplicate shaping case %s", report.Case.ID)
		}
		seen[report.Case.ID] = struct{}{}
		if ffmpegVersion == "" {
			ffmpegVersion = report.Evidence.FFmpegVersion
			ffprobeVersion = report.Evidence.FFprobeVersion
		} else if report.Evidence.FFmpegVersion != ffmpegVersion || report.Evidence.FFprobeVersion != ffprobeVersion {
			return fmt.Errorf("shaping matrix toolchain identity is inconsistent")
		}
	}
	return nil
}

func MarshalShapingMatrixReport(report ShapingMatrixReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal shaping matrix: %w", err)
	}
	return append(content, '\n'), nil
}
