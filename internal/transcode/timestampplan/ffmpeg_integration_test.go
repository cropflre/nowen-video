package timestampplan

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const requireFFmpegFixtureEnv = "NOWEN_REQUIRE_FFMPEG_TIMESTAMP_FIXTURE"

func TestFFmpegCopyTSStartAtZeroPreservesContinuationOrigin(t *testing.T) {
	ffmpeg, ffmpegErr := exec.LookPath("ffmpeg")
	ffprobe, ffprobeErr := exec.LookPath("ffprobe")
	if ffmpegErr != nil || ffprobeErr != nil {
		if os.Getenv(requireFFmpegFixtureEnv) == "1" {
			t.Fatalf("required ffmpeg fixture tools are unavailable: ffmpeg=%v ffprobe=%v", ffmpegErr, ffprobeErr)
		}
		t.Skip("ffmpeg/ffprobe not installed")
	}

	root := t.TempDir()
	source := filepath.Join(root, "source.mp4")
	runFixtureCommand(t, ffmpeg,
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=320x180:rate=30:duration=8",
		"-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=48000:duration=8",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "ultrafast",
		"-g", "60", "-keyint_min", "60", "-sc_threshold", "0",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
		"-shortest", source,
	)

	startupDir := filepath.Join(root, "startup")
	continuationDir := filepath.Join(root, "continuation")
	if err := os.MkdirAll(startupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(continuationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Production Startup is bounded with -t. Production Continuation is not:
	// it seeks to the Job origin and runs to source EOF. Keeping the fixture
	// identical matters because -t is evaluated against copied timestamps.
	runNormalizedFixture(t, ffmpeg, source, startupDir, 0, 4)
	runNormalizedFixture(t, ffmpeg, source, continuationDir, 4, 0)

	startupManifest := filepath.Join(startupDir, "stream.m3u8")
	continuationManifest := filepath.Join(continuationDir, "stream.m3u8")
	startupSegment := firstPlaylistSegment(t, startupManifest)
	continuationSegment := firstPlaylistSegment(t, continuationManifest)
	startupVideo := firstPacketTime(t, ffprobe, startupSegment, startupManifest, "v:0")
	continuationVideo := firstPacketTime(t, ffprobe, continuationSegment, continuationManifest, "v:0")
	startupAudio := firstPacketTime(t, ffprobe, startupSegment, startupManifest, "a:0")
	continuationAudio := firstPacketTime(t, ffprobe, continuationSegment, continuationManifest, "a:0")

	videoDelta := continuationVideo - startupVideo
	audioDelta := continuationAudio - startupAudio
	t.Logf(
		"measured packet origins: startup_video=%.6fs continuation_video=%.6fs video_delta=%.6fs startup_audio=%.6fs continuation_audio=%.6fs audio_delta=%.6fs",
		startupVideo,
		continuationVideo,
		videoDelta,
		startupAudio,
		continuationAudio,
		audioDelta,
	)
	assertOriginDelta(t, "video", startupVideo, continuationVideo, 4)
	assertOriginDelta(t, "audio", startupAudio, continuationAudio, 4)
	if continuationVideo < 4 || continuationAudio < 4 {
		t.Fatalf(
			"continuation timestamps reset near muxer origin: startup_video=%.6f continuation_video=%.6f startup_audio=%.6f continuation_audio=%.6f",
			startupVideo,
			continuationVideo,
			startupAudio,
			continuationAudio,
		)
	}
}

func runNormalizedFixture(t *testing.T, ffmpeg, source, outputDir string, startSeconds, durationSeconds int) {
	t.Helper()
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-copyts", "-start_at_zero"}
	if startSeconds > 0 {
		args = append(args, "-ss", strconv.Itoa(startSeconds))
	}
	args = append(args,
		"-i", source,
		"-map", "0:v:0", "-map", "0:a:0",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "ultrafast",
		"-g", "60", "-keyint_min", "60", "-sc_threshold", "0",
		"-c:a", "aac", "-ar", "48000", "-ac", "2",
	)
	if durationSeconds > 0 {
		args = append(args, "-t", strconv.Itoa(durationSeconds))
	}
	args = append(args,
		"-f", "hls", "-hls_time", "2", "-hls_list_size", "0",
		"-hls_playlist_type", "vod", "-hls_flags", "independent_segments",
		"-hls_segment_filename", filepath.Join(outputDir, "seg%04d.ts"),
		"-avoid_negative_ts", "disabled", "-fps_mode", "passthrough",
		filepath.Join(outputDir, "stream.m3u8"),
	)
	runFixtureCommand(t, ffmpeg, args...)
}

func runFixtureCommand(t *testing.T, path string, args ...string) {
	t.Helper()
	command := exec.Command(path, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %s %s\n%s\nerror=%v", path, strings.Join(args, " "), string(output), err)
	}
}

func firstPlaylistSegment(t *testing.T, manifest string) string {
	t.Helper()
	content, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if filepath.IsAbs(line) {
			return line
		}
		return filepath.Join(filepath.Dir(manifest), line)
	}
	t.Fatalf("playlist contains no segment: %s\n%s", manifest, string(content))
	return ""
}

func firstPacketTime(t *testing.T, ffprobe, segment, manifest, streamSelector string) float64 {
	t.Helper()
	command := exec.Command(
		ffprobe,
		"-v", "error",
		"-select_streams", streamSelector,
		"-show_packets",
		"-show_entries", "packet=pts_time,dts_time",
		"-of", "json",
		segment,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		manifestContent, _ := os.ReadFile(manifest)
		info, statErr := os.Stat(segment)
		size := int64(-1)
		if statErr == nil {
			size = info.Size()
		}
		t.Fatalf(
			"ffprobe %s %s failed: %v\nsegment_size=%d stat_error=%v\nmanifest=%s\nffprobe_output=%s",
			streamSelector,
			segment,
			err,
			size,
			statErr,
			string(manifestContent),
			string(output),
		)
	}
	var payload struct {
		Packets []struct {
			PTS string `json:"pts_time"`
			DTS string `json:"dts_time"`
		} `json:"packets"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("decode ffprobe packet output: %v\n%s", err, string(output))
	}
	if len(payload.Packets) == 0 {
		t.Fatalf("ffprobe returned no %s packets for %s", streamSelector, segment)
	}
	value := payload.Packets[0].PTS
	if value == "" || value == "N/A" {
		value = payload.Packets[0].DTS
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		t.Fatalf("parse first %s packet time %q: %v", streamSelector, value, err)
	}
	return parsed
}

func assertOriginDelta(t *testing.T, kind string, startup, continuation, expected float64) {
	t.Helper()
	delta := continuation - startup
	if delta < expected-0.15 || delta > expected+0.15 {
		t.Fatalf(
			"%s timestamp origin delta is not preserved: startup=%.6f continuation=%.6f delta=%.6f expected=%.3f",
			kind,
			startup,
			continuation,
			delta,
			expected,
		)
	}
	if startup < 0 || continuation <= startup {
		t.Fatalf("%s packet order is invalid: startup=%.6f continuation=%.6f", kind, startup, continuation)
	}
}

func ExamplePlan_ffmpegPolicy() {
	plan := Default()
	fmt.Printf("%s %s %t %t %s %s", plan.SchemaVersion, plan.Strategy, plan.CopyTimestamps, plan.StartAtZero, plan.AvoidNegativeTS, plan.FPSMode)
	// Output: hls-timestamp-normalization-v1 copyts_start_at_zero true true disabled passthrough
}
