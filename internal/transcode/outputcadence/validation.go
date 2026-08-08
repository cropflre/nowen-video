package outputcadence

// ValidateFor exposes the complete timeline arithmetic validator to bound
// evidence contracts without duplicating the output-cadence rules.
func (t TimelineEvidence) ValidateFor(expectedKind string) error {
	return t.validate(expectedKind)
}

// Validate exposes the complete frame-mapping arithmetic validator to bound
// evidence contracts.
func (m FrameMapping) Validate() error {
	return m.validate()
}
