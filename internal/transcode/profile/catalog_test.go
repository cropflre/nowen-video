package profile

import (
	"reflect"
	"testing"
)

func TestCatalogOrderAndPolicies(t *testing.T) {
	wantNames := []string{"360p", "480p", "720p", "1080p", "2K", "4K"}
	if got := Names(); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("unexpected quality order: got=%v want=%v", got, wantNames)
	}

	runtime1080, ok := Runtime("1080p")
	if !ok {
		t.Fatal("runtime 1080p profile missing")
	}
	persistent1080, ok := Persistent("1080p")
	if !ok {
		t.Fatal("persistent 1080p profile missing")
	}
	if runtime1080.VideoBitrate != "6000k" {
		t.Fatalf("runtime bitrate changed: %s", runtime1080.VideoBitrate)
	}
	if persistent1080.VideoBitrate != "5000k" || persistent1080.MaxBitrate != "7500k" || persistent1080.BufSize != "10000k" {
		t.Fatalf("persistent policy changed: %+v", persistent1080)
	}
	if runtime1080.Width != persistent1080.Width || runtime1080.Height != persistent1080.Height || runtime1080.AudioBitrate != persistent1080.AudioBitrate {
		t.Fatalf("shared profile dimensions/audio drifted: runtime=%+v persistent=%+v", runtime1080, persistent1080)
	}
}

func TestCatalogCopiesAndHeightSelection(t *testing.T) {
	profiles := PersistentProfiles()
	profiles[0].Name = "mutated"
	if got := PersistentProfiles()[0].Name; got != "360p" {
		t.Fatalf("catalog was mutated through returned slice: %s", got)
	}

	if got := NamesUpToHeight(1080); !reflect.DeepEqual(got, []string{"360p", "480p", "720p", "1080p"}) {
		t.Fatalf("unexpected height selection: %v", got)
	}
	selected, ok := HighestPersistentAtOrBelow(900)
	if !ok || selected.Name != "720p" {
		t.Fatalf("unexpected highest profile: ok=%v profile=%+v", ok, selected)
	}
	if _, ok := HighestPersistentAtOrBelow(200); ok {
		t.Fatal("height below minimum unexpectedly returned a profile")
	}
}
