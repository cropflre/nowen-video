package certification

import (
	"encoding/json"
	"fmt"

	transcoderecovery "github.com/nowen-video/nowen-video/internal/transcode/recoverystress"
)

func (r RecoveryStressScenarioReport) Validate() error {
	if r.SchemaVersion != RecoveryStressScenarioReportSchemaVersion {
		return fmt.Errorf("unsupported recovery stress scenario report schema %q", r.SchemaVersion)
	}
	if err := r.Evidence.ValidateFor(r.Spec, r.Manifest); err != nil {
		return err
	}
	version, hash, _, err := transcoderecovery.ScenarioIdentity(r.Evidence, r.Spec, r.Manifest)
	if err != nil {
		return err
	}
	if r.ContractVersion != version || r.ContractHash != hash {
		return fmt.Errorf("recovery stress scenario report contract identity is invalid")
	}
	return nil
}

func (r RecoveryStressAggregateReport) Validate() error {
	if r.SchemaVersion != RecoveryStressAggregateReportSchemaVersion {
		return fmt.Errorf("unsupported recovery stress aggregate report schema %q", r.SchemaVersion)
	}
	if err := r.Evidence.ValidateFor(r.Spec, r.Manifest); err != nil {
		return err
	}
	version, hash, _, err := transcoderecovery.AggregateIdentity(r.Evidence, r.Spec, r.Manifest)
	if err != nil {
		return err
	}
	if r.ContractVersion != version || r.ContractHash != hash {
		return fmt.Errorf("recovery stress aggregate report contract identity is invalid")
	}
	return nil
}

func AggregateRecoveryStressScenarioReports(reports []RecoveryStressScenarioReport) (RecoveryStressAggregateReport, error) {
	if len(reports) == 0 {
		return RecoveryStressAggregateReport{}, fmt.Errorf("no recovery stress scenario reports were supplied")
	}
	expected := transcoderecovery.AvailableScenarios()
	byID := make(map[string]RecoveryStressScenarioReport, len(reports))
	for _, report := range reports {
		if err := report.Validate(); err != nil {
			return RecoveryStressAggregateReport{}, err
		}
		id := report.Evidence.Scenario.ID
		if _, duplicate := byID[id]; duplicate {
			return RecoveryStressAggregateReport{}, fmt.Errorf("duplicate recovery stress scenario %s", id)
		}
		byID[id] = report
	}
	if len(byID) != len(expected) {
		return RecoveryStressAggregateReport{}, fmt.Errorf("recovery stress aggregate has %d scenarios, want %d", len(byID), len(expected))
	}
	first := reports[0]
	bindings := make([]transcoderecovery.ScenarioBinding, 0, len(expected))
	totalProcesses := 0
	totalSegments := 0
	maximumRSS := int64(0)
	for _, scenario := range expected {
		report, ok := byID[scenario.ID]
		if !ok {
			return RecoveryStressAggregateReport{}, fmt.Errorf("missing recovery stress scenario %s", scenario.ID)
		}
		if report.Evidence.SpecHash != first.Evidence.SpecHash || report.Evidence.ManifestHash != first.Evidence.ManifestHash || report.Evidence.SourceGeneratorVersion != first.Evidence.SourceGeneratorVersion || report.Evidence.SourceFFmpegVersion != first.Evidence.SourceFFmpegVersion || report.Evidence.SourceFFprobeVersion != first.Evidence.SourceFFprobeVersion || report.Evidence.CertificationFFmpegVersion != first.Evidence.CertificationFFmpegVersion || report.Evidence.CertificationFFprobeVersion != first.Evidence.CertificationFFprobeVersion || report.Evidence.TimestampPlanHash != first.Evidence.TimestampPlanHash || report.Evidence.Source != first.Evidence.Source {
			return RecoveryStressAggregateReport{}, fmt.Errorf("recovery stress scenario identity differs across reports")
		}
		bindings = append(bindings, transcoderecovery.ScenarioBinding{
			ScenarioID:      scenario.ID,
			ContractVersion: report.ContractVersion,
			ContractHash:    report.ContractHash,
			Evidence:        report.Evidence,
		})
		totalProcesses += len(report.Evidence.Processes)
		for _, process := range report.Evidence.Processes {
			totalSegments += process.SegmentCount
			if process.MaxRSSBytes > maximumRSS {
				maximumRSS = process.MaxRSSBytes
			}
		}
	}
	contract := transcoderecovery.AggregateContract{
		SchemaVersion:               transcoderecovery.AggregateSchemaVersion,
		SpecVersion:                 first.Evidence.SpecVersion,
		SpecHash:                    first.Evidence.SpecHash,
		ManifestVersion:             first.Evidence.ManifestVersion,
		ManifestHash:                first.Evidence.ManifestHash,
		SourceGeneratorVersion:      first.Evidence.SourceGeneratorVersion,
		SourceFFmpegVersion:         first.Evidence.SourceFFmpegVersion,
		SourceFFprobeVersion:        first.Evidence.SourceFFprobeVersion,
		CertificationFFmpegVersion:  first.Evidence.CertificationFFmpegVersion,
		CertificationFFprobeVersion: first.Evidence.CertificationFFprobeVersion,
		TimestampPlanVersion:        first.Evidence.TimestampPlanVersion,
		TimestampPlanHash:           first.Evidence.TimestampPlanHash,
		Source:                      first.Evidence.Source,
		Scenarios:                   bindings,
		TotalProcesses:              totalProcesses,
		TotalSegmentsObserved:       totalSegments,
		MaximumRSSBytes:             maximumRSS,
		AllPassed:                   true,
		DiscontinuityRequired:       true,
	}
	version, hash, _, err := transcoderecovery.AggregateIdentity(contract, first.Spec, first.Manifest)
	if err != nil {
		return RecoveryStressAggregateReport{}, err
	}
	report := RecoveryStressAggregateReport{
		SchemaVersion:   RecoveryStressAggregateReportSchemaVersion,
		Spec:            first.Spec,
		Manifest:        first.Manifest,
		ContractVersion: version,
		ContractHash:    hash,
		Evidence:        contract,
	}
	if err := report.Validate(); err != nil {
		return RecoveryStressAggregateReport{}, err
	}
	return report, nil
}

func MarshalRecoveryStressScenarioReport(report RecoveryStressScenarioReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func MarshalRecoveryStressAggregateReport(report RecoveryStressAggregateReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}
