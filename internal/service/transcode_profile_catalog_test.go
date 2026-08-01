package service

import (
	"reflect"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodeprofile "github.com/nowen-video/nowen-video/internal/transcode/profile"
)

func TestAvailableQualitiesUseSharedCatalogOrder(t *testing.T) {
	service := &TranscodeService{}
	if got := service.GetAvailableQualities(&model.Media{Resolution: "1080p"}); !reflect.DeepEqual(got, []string{"360p", "480p", "720p", "1080p"}) {
		t.Fatalf("unexpected 1080p quality ladder: %v", got)
	}
	if got := service.GetAvailableQualities(&model.Media{Resolution: "4K"}); !reflect.DeepEqual(got, transcodeprofile.Names()) {
		t.Fatalf("4K ladder drifted from shared catalog: got=%v catalog=%v", got, transcodeprofile.Names())
	}
	if got := service.GetAvailableQualities(&model.Media{Resolution: "240p"}); !reflect.DeepEqual(got, []string{"360p"}) {
		t.Fatalf("sub-360 source fallback changed: %v", got)
	}
}

func TestLegacyABRProfilesMirrorPersistentCatalog(t *testing.T) {
	persistent := transcodeprofile.PersistentProfiles()
	if !reflect.DeepEqual(abrProfiles, persistent) {
		t.Fatalf("legacy ABR compatibility slice drifted: got=%+v want=%+v", abrProfiles, persistent)
	}

	statusNames := transcodeprofile.Names()
	if got := (&ABRService{hwAccel: "none"}).GetABRStatus().Profiles; !reflect.DeepEqual(got, statusNames) {
		t.Fatalf("ABR status names drifted: got=%v want=%v", got, statusNames)
	}
}
