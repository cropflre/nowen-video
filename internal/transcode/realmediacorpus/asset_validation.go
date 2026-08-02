package realmediacorpus

// ValidateFor validates one resolved asset against a corpus case using the
// immutable v1 two-pass generation policy. Manifest.ValidateFor remains the
// authoritative whole-corpus order and identity gate; this method exists for
// downstream evidence adapters that already received a validated manifest.
func (a AssetEvidence) ValidateFor(caseSpec CaseSpec) error {
	return a.validateFor(caseSpec, 2)
}
