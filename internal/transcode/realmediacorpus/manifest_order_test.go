package realmediacorpus

import (
	"strings"
	"testing"
)

func TestManifestRejectsNoncanonicalAssetOrder(t *testing.T) {
	spec := DefaultSpec()
	manifest := validManifest(t, spec)
	manifest.Assets[0], manifest.Assets[1] = manifest.Assets[1], manifest.Assets[0]
	if err := manifest.ValidateFor(spec); err == nil || !strings.Contains(err.Error(), "canonical case order") {
		t.Fatalf("expected canonical asset order failure, got %v", err)
	}
}
