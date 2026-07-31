package service

import (
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestManagedRemuxModeCopiesCompatibleAudio(t *testing.T) {
	for _, codec := range []string{"", "aac", "mp3"} {
		mode, ok := managedRemuxMode(&model.Media{VideoCodec: "h264", AudioCodec: codec})
		if !ok || mode != ManagedRemuxCopyAudio {
			t.Fatalf("audio=%q mode=%q ok=%v", codec, mode, ok)
		}
	}
}

func TestManagedRemuxModeTranscodesOnlyIncompatibleAudio(t *testing.T) {
	for _, codec := range []string{"dts", "truehd", "flac", "opus", "ac3", "eac3"} {
		mode, ok := managedRemuxMode(&model.Media{VideoCodec: "h264", AudioCodec: codec})
		if !ok || mode != ManagedRemuxTranscodeAudio {
			t.Fatalf("audio=%q mode=%q ok=%v", codec, mode, ok)
		}
	}
}

func TestManagedRemuxRejectsVideoReencodeSources(t *testing.T) {
	for _, codec := range []string{"mpeg2video", "vc1", "wmv3", "vp9", "av1", ""} {
		if mode, ok := managedRemuxMode(&model.Media{VideoCodec: codec, AudioCodec: "aac"}); ok {
			t.Fatalf("video=%q unexpectedly accepted as %q", codec, mode)
		}
	}
	if _, ok := managedRemuxMode(&model.Media{VideoCodec: "h264", StreamURL: "https://example.invalid/video"}); ok {
		t.Fatal("STRM media must not enter local managed remux")
	}
}
