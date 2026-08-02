package certification

import (
	"testing"

	transcodelongdrift "github.com/nowen-video/nowen-video/internal/transcode/longdrift"
	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
)

func TestLongDurationProfileSourcesResolveInDeclaredOrder(t *testing.T) {
	spec := transcodecorpus.DefaultSpec()
	manifest := transcodecorpus.Manifest{Assets: make([]transcodecorpus.AssetEvidence, 0, len(spec.Cases))}
	for _, caseSpec := range spec.Cases {
		manifest.Assets = append(manifest.Assets, transcodecorpus.AssetEvidence{CaseID: caseSpec.ID})
	}
	for _, profile := range transcodelongdrift.AvailableProfiles() {
		caseSpec, asset, ok := longDurationProfileSource(spec, manifest, profile.SourceCaseID)
		if !ok {
			t.Fatalf("profile source %s is missing", profile.SourceCaseID)
		}
		if caseSpec.ID != profile.SourceCaseID || asset.CaseID != profile.SourceCaseID {
			t.Fatalf("profile %s resolved the wrong source: case=%s asset=%s", profile.ID, caseSpec.ID, asset.CaseID)
		}
	}
}
