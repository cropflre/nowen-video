package recoverystress

import (
	"encoding/json"
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

func TestBoundedScenarioRestoresInternalMemoryLimitAfterJSON(t *testing.T) {
	expected, ok := LookupScenario(ScenarioBoundedResources)
	if !ok {
		t.Fatal("bounded scenario missing")
	}
	content, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ScenarioSpec
	if err := json.Unmarshal(content, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != expected {
		t.Fatalf("decoded bounded scenario differs:\n got: %#v\nwant: %#v", decoded, expected)
	}
	if decoded.Limits.MemoryMaxBytes != decoded.Limits.AddressSpaceBytes || decoded.Limits.MemoryMaxBytes <= 0 {
		t.Fatalf("internal memory limit = %d, address-space field = %d", decoded.Limits.MemoryMaxBytes, decoded.Limits.AddressSpaceBytes)
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
