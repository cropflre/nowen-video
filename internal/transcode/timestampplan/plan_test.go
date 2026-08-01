package timestampplan

import "testing"

func TestDefaultPlanIdentityIsDeterministic(t *testing.T) {
	plan := Default()
	versionA, hashA, canonicalA, err := Identity(plan)
	if err != nil {
		t.Fatal(err)
	}
	versionB, hashB, canonicalB, err := Identity(plan)
	if err != nil {
		t.Fatal(err)
	}
	if versionA != SchemaVersion || versionA != versionB || hashA != hashB || canonicalA != canonicalB {
		t.Fatal("timestamp plan identity is not deterministic")
	}
	if !plan.SupportsBackend(BackendSoftware) || plan.SupportsBackend("qsv") {
		t.Fatal("timestamp plan v1 backend certification is invalid")
	}
}

func TestTimestampPolicyMutationChangesHash(t *testing.T) {
	plan := Default()
	_, original, _, err := Identity(plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.OriginUpperToleranceMS++
	_, changed, _, err := Identity(plan)
	if err != nil {
		t.Fatal(err)
	}
	if original == changed {
		t.Fatal("timestamp compatibility mutation did not change identity")
	}
}

func TestVerifyObservedStartRejectsResetContinuation(t *testing.T) {
	plan := Default()
	if err := plan.VerifyObservedStart(30_000, 31_400, 31_379); err != nil {
		t.Fatalf("normalized continuation rejected: %v", err)
	}
	if err := plan.VerifyObservedStart(30_000, 1_400, 1_379); err == nil {
		t.Fatal("reset continuation timeline was accepted")
	}
}

func TestPlanValidationRejectsHardwareCertificationInV1(t *testing.T) {
	plan := Default()
	plan.CertifiedBackends = []string{"qsv"}
	if err := plan.Validate(); err == nil {
		t.Fatal("hardware backend was unexpectedly certified")
	}
}
