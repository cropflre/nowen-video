package timestampplan

import (
	"reflect"
	"strings"
	"testing"
)

func TestApplyFFmpegPlacesTimestampPolicyAtStableBoundaries(t *testing.T) {
	input := []string{"-y", "-ss", "30.00", "-i", "/media/movie.mkv", "-f", "hls", "/cache/stream.m3u8"}
	actual, err := ApplyFFmpeg(input, Default())
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{
		"-y", "-copyts", "-start_at_zero",
		"-ss", "30.00", "-i", "/media/movie.mkv", "-f", "hls",
		"-avoid_negative_ts", "disabled", "-fps_mode", "passthrough",
		"/cache/stream.m3u8",
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("unexpected normalized command:\nactual:   %v\nexpected: %v", actual, expected)
	}
	if summary := CommandSummary(actual); summary != "-copyts -start_at_zero -ss 30.00 -avoid_negative_ts disabled -fps_mode passthrough" {
		t.Fatalf("unexpected command summary %q", summary)
	}
}

func TestApplyFFmpegRejectsInvalidPlanAndMissingOutput(t *testing.T) {
	plan := Default()
	plan.AvoidNegativeTS = "make_zero"
	if _, err := ApplyFFmpeg([]string{"-y", "output.m3u8"}, plan); err == nil {
		t.Fatal("invalid timestamp plan was accepted")
	}
	if _, err := ApplyFFmpeg([]string{"-y"}, Default()); err == nil || !strings.Contains(err.Error(), "output path") {
		t.Fatalf("missing output path was not rejected: %v", err)
	}
}
