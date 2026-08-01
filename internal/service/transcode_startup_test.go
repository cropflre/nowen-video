package service

import (
	"strings"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestStartupStreamEligibility(t *testing.T) {
	media := &model.Media{ID: "media-1", FilePath: "/media/movie.mkv"}
	cases := []struct {
		name  string
		probe model.MediaProbeRecord
		want  bool
	}{
		{name: "h264-8bit-sdr", probe: model.MediaProbeRecord{VideoCodec: "h264", BitDepth: 8, DurationMS: 60_000}, want: false},
		{name: "h264-10bit", probe: model.MediaProbeRecord{VideoCodec: "h264", BitDepth: 10, DurationMS: 60_000}, want: true},
		{name: "h264-hdr", probe: model.MediaProbeRecord{VideoCodec: "h264", BitDepth: 8, HDR: true, DurationMS: 60_000}, want: true},
		{name: "hevc", probe: model.MediaProbeRecord{VideoCodec: "hevc", BitDepth: 10, DurationMS: 60_000}, want: true},
		{name: "vc1", probe: model.MediaProbeRecord{VideoCodec: "vc1", BitDepth: 8, DurationMS: 60_000}, want: true},
		{name: "two-second-clip", probe: model.MediaProbeRecord{VideoCodec: "hevc", DurationMS: 2_000}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := StartupStreamEligible(media, &tc.probe); got != tc.want {
				t.Fatalf("eligibility=%v want=%v probe=%+v", got, tc.want, tc.probe)
			}
		})
	}
}

func TestStartupStreamRejectsResolverSpecificSources(t *testing.T) {
	probe := &model.MediaProbeRecord{VideoCodec: "hevc", DurationMS: 60_000}
	for _, path := range []string{"/media/remote.strm", "webdav://movies/movie.mkv"} {
		media := &model.Media{ID: "media-1", FilePath: path}
		if StartupStreamEligible(media, probe) {
			t.Fatalf("resolver-specific source was accepted: %s", path)
		}
	}
}

func TestStartupStreamProfileCapsAt720p(t *testing.T) {
	cases := []struct {
		height int
		want   string
	}{
		{height: 2160, want: "720p"},
		{height: 1080, want: "720p"},
		{height: 720, want: "720p"},
		{height: 480, want: "480p"},
		{height: 240, want: "360p"},
	}
	for _, tc := range cases {
		if got := startupStreamProfile(&model.MediaProbeRecord{Height: tc.height}); got != tc.want {
			t.Fatalf("height=%d profile=%s want=%s", tc.height, got, tc.want)
		}
	}
}

func TestStartupStreamOutputArgsReplaceRuntimePlaylistSemantics(t *testing.T) {
	input := []string{
		"-i", "/media/movie.mkv",
		"-hls_flags", "independent_segments+append_list+program_date_time",
		"-hls_playlist_type", "event",
		"-progress", "pipe:2",
		"/cache/work/stream.m3u8",
	}
	output := startupStreamOutputArgs(input, startupStreamDurationMS)
	joined := strings.Join(output, " ")
	if strings.Contains(joined, "append_list") || strings.Contains(joined, "-hls_playlist_type event") {
		t.Fatalf("runtime playlist semantics leaked into startup VOD: %s", joined)
	}
	if strings.Count(joined, "-hls_playlist_type") != 1 {
		t.Fatalf("startup VOD must contain exactly one playlist type: %s", joined)
	}
	for _, expected := range []string{"-t 30.000", "-hls_playlist_type vod", "independent_segments+program_date_time"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q: %s", expected, joined)
		}
	}
	if output[len(output)-1] != "/cache/work/stream.m3u8" {
		t.Fatalf("output path moved from final argument: %+v", output)
	}
}
