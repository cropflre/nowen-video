package certification

import "testing"

func TestValidateCertifiedReportRejectsDescriptionDrift(t *testing.T) {
	report := validReport(FixtureCFR48KZeroLatency)
	if err := ValidateCertifiedReport(report); err != nil {
		t.Fatalf("valid certified report rejected: %v", err)
	}

	report.Fixture.Description = "tampered fixture description"
	if err := report.Validate(); err != nil {
		t.Fatalf("structural validation should keep v2 reports readable: %v", err)
	}
	if err := ValidateCertifiedReport(report); err == nil {
		t.Fatal("registry identity gate accepted fixture description drift")
	}
}

func TestMarshalCertifiedReportUsesRegistryGate(t *testing.T) {
	report := validReport(FixtureCFR44K1ZeroLatency)
	report.Fixture.Description = "drifted"
	if _, err := MarshalCertifiedReport(report); err == nil {
		t.Fatal("certified marshal accepted registry metadata drift")
	}
}
