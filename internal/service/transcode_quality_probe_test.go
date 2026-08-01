package service

import (
	"strings"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestBuildSoftwareVideoFilterPreservesAspectRatioForSDRHEVC(t *testing.T) {
	probe := &model.MediaProbeRecord{
		VideoCodec:    "hevc",
		ColorTransfer: "bt709",
		HDR:           false,
	}
	filter := buildSoftwareVideoFilter(probe, 1280, 720)
	if strings.Contains(filter, "tonemap=") {
		t.Fatalf("SDR HEVC was incorrectly tone mapped: %s", filter)
	}
	for _, expected := range []string{
		"force_original_aspect_ratio=decrease",
		"pad=1280:720",
		"setsar=1",
	} {
		if !strings.Contains(filter, expected) {
			t.Fatalf("filter missing %q: %s", expected, filter)
		}
	}
}

func TestBuildSoftwareVideoFilterToneMapsConfirmedHDR(t *testing.T) {
	filter := buildSoftwareVideoFilter(&model.MediaProbeRecord{HDR: true}, 1920, 1080)
	for _, expected := range []string{
		"tonemap=hable",
		"zscale=p=bt709",
		"format=yuv420p",
		"pad=1920:1080",
	} {
		if !strings.Contains(filter, expected) {
			t.Fatalf("HDR filter missing %q: %s", expected, filter)
		}
	}
}

func TestCompatibilityHDRFilterNoLongerGuessesByCodec(t *testing.T) {
	service := &TranscodeService{}
	filter := service.buildFFmpegHDRTonemapFilter(&model.Media{VideoCodec: "hevc"}, 854, 480)
	if strings.Contains(filter, "tonemap=") {
		t.Fatalf("legacy media summary must not guess HDR: %s", filter)
	}
}
