package certification

import "testing"

func TestOutputCadenceMatrixRejectsIncompleteCases(t *testing.T) {
	matrix := OutputCadenceMatrixReport{SchemaVersion: OutputCadenceMatrixSchemaVersion}
	if err := matrix.Validate(); err == nil {
		t.Fatal("incomplete output cadence matrix was accepted")
	}
}

func TestOutputCadenceUsesSourceOriginRegistry(t *testing.T) {
	cases := AvailableSourceOriginCases()
	if len(cases) != len(sourceOriginCaseSpecs) {
		t.Fatalf("output cadence registry count = %d, want %d", len(cases), len(sourceOriginCaseSpecs))
	}
	for index, spec := range cases {
		if spec != sourceOriginCaseSpecs[index] {
			t.Fatalf("output cadence case %d drifted from source-origin registry", index)
		}
	}
}
