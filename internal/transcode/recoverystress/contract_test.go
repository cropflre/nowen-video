package recoverystress

import (
	"testing"
)

func TestAvailableScenariosAreCanonical(t *testing.T) {
	expected := []string{
		ScenarioCancelActiveWrite,
		ScenarioSIGKILLRecovery,
		ScenarioENOSPCWrite,
		ScenarioBoundedResources,
		ScenarioStaleLeaseFence,
	}
	scenarios := AvailableScenarios()
	if len(scenarios) != len(expected) {
		t.Fatalf("scenario count = %d, want %d", len(scenarios), len(expected))
	}
	for index, id := range expected {
		if scenarios[index].ID != id {
			t.Fatalf("scenario[%d] = %q, want %q", index, scenarios[index].ID, id)
		}
		if scenarios[index].LogicalDurationMicros <= 0 || scenarios[index].ExpectedProcessCount <= 0 {
			t.Fatalf("scenario %s has invalid execution geometry", id)
		}
		if found, ok := LookupScenario(id); !ok || found != scenarios[index] {
			t.Fatalf("lookup scenario %s did not return canonical value", id)
		}
	}
}

func TestTokenHashDoesNotExposeLease(t *testing.T) {
	const token = "lease-token-secret"
	hash := TokenHash(token)
	if !isSHA256(hash) {
		t.Fatalf("token hash is not SHA-256: %q", hash)
	}
	if hash == token || TokenHash(token+"-2") == hash {
		t.Fatal("token hash does not fence distinct Lease identities")
	}
}
