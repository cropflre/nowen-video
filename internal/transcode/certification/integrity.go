package certification

import (
	"encoding/json"
	"fmt"
)

// ValidateCertifiedReport applies the registry-backed identity gate used when a
// fixture report is issued or embedded in a matrix. Report.Validate remains the
// structural/schema validator so already-produced v2 reports stay readable,
// while certification entrypoints reject metadata that no longer exactly
// matches the immutable fixture registry.
func ValidateCertifiedReport(report Report) error {
	if err := report.Validate(); err != nil {
		return err
	}
	expected, ok := LookupFixture(report.FixtureID)
	if !ok {
		return fmt.Errorf("fixture %q is not registered", report.FixtureID)
	}
	if report.Fixture != expected {
		return fmt.Errorf("fixture %q metadata does not match registry", report.FixtureID)
	}
	return nil
}

func MarshalCertifiedReport(report Report) ([]byte, error) {
	if err := ValidateCertifiedReport(report); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal certified fixture report: %w", err)
	}
	return append(content, '\n'), nil
}
