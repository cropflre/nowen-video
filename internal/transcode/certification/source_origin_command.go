package certification

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serviceffmpeg "github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func sourceOriginSourceArgs(output string, spec SourceOriginCaseSpec) []string {
	offset := sourceOriginOffsetExpression(spec.SourceOffsetMicros)
	base := []string{"-hide_banner", "-loglevel", "error", "-y", "-copyts"}
	if spec.SourceMode == transcodesourceorigin.ModeVFR {
		base = append(base,
			"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=size=%dx%d:rate=24:duration=20", fixtureWidth, fixtureHeight),
			"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=size=%dx%d:rate=30:duration=20", fixtureWidth, fixtureHeight),
			"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=1000:sample_rate=%d:duration=%d", spec.AudioSampleRate, sourceOriginDurationSeconds),
			"-filter_complex", fmt.Sprintf("[0:v]settb=AVTB[v0];[1:v]settb=AVTB[v1];[v0][v1]concat=n=2:v=1:a=0,setpts=%s[v];[2:a]asettb=1/%d,asetpts=%s[a]", offset, spec.AudioSampleRate, offset),
		)
	} else {
		base = append(base,
			"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=size=%dx%d:rate=%s:duration=%d", fixtureWidth, fixtureHeight, sourceOriginRateExpression(spec), sourceOriginDurationSeconds),
			"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=1000:sample_rate=%d:duration=%d", spec.AudioSampleRate, sourceOriginDurationSeconds),
			"-filter_complex", fmt.Sprintf("[0:v]settb=AVTB,setpts=%s[v];[1:a]asettb=1/%d,asetpts=%s[a]", offset, spec.AudioSampleRate, offset),
		)
	}
	return append(base,
		"-map", "[v]", "-map", "[a]",
		"-c:v", "libx264", "-preset", "veryfast", "-tune", "zerolatency", "-pix_fmt", "yuv420p",
		"-g", fmt.Sprint(spec.GOPSize), "-keyint_min", fmt.Sprint(spec.GOPSize), "-sc_threshold", "0",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", fmt.Sprint(spec.AudioSampleRate),
		"-fps_mode:v", "passthrough", "-avoid_negative_ts", "disabled", "-f", "nut", output,
	)
}

func sourceOriginRateExpression(spec SourceOriginCaseSpec) string {
	if spec.DeclaredFrameRateDenominator == 1 {
		return fmt.Sprint(spec.DeclaredFrameRateNumerator)
	}
	return fmt.Sprintf("%d/%d", spec.DeclaredFrameRateNumerator, spec.DeclaredFrameRateDenominator)
}

func sourceOriginOffsetExpression(offsetMicros int64) string {
	if offsetMicros == 0 {
		return "PTS"
	}
	seconds := formatMicrosSeconds(abs64Certification(offsetMicros))
	if offsetMicros > 0 {
		return "PTS+" + seconds + "/TB"
	}
	return "PTS-" + seconds + "/TB"
}

func produceSourceOriginHLS(
	ctx context.Context,
	ffmpegPath,
	workDir,
	name,
	sourcePath string,
	timestampPlan transcodetimestamp.Plan,
	spec SourceOriginCaseSpec,
	startMicros,
	durationMicros int64,
) (string, error) {
	directory := filepath.Join(workDir, name)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create source origin %s directory: %w", name, err)
	}
	args, err := sourceOriginHLSArgs(sourcePath, directory, timestampPlan, spec, startMicros, durationMicros)
	if err != nil {
		return "", fmt.Errorf("build source origin %s command: %w", name, err)
	}
	if err := runCommand(ctx, ffmpegPath, args...); err != nil {
		return "", fmt.Errorf("produce source origin %s fixture: %w", name, err)
	}
	return filepath.Join(directory, "stream.m3u8"), nil
}

func sourceOriginHLSArgs(
	sourcePath,
	outputDir string,
	timestampPlan transcodetimestamp.Plan,
	spec SourceOriginCaseSpec,
	startMicros,
	durationMicros int64,
) ([]string, error) {
	args := serviceffmpeg.BuildHLSArgs(serviceffmpeg.BuildOptions{
		InputPath: sourcePath,
		OutputDir: outputDir,
		HWAccel:   serviceffmpeg.HWAccelNone,
		Profile: serviceffmpeg.Profile{
			Width:        fixtureWidth,
			Height:       fixtureHeight,
			VideoBitrate: "800k",
			AudioBitrate: "128k",
			MaxBitrate:   "900k",
			BufSize:      "1600k",
		},
		X264Preset:      "veryfast",
		SoftwareTune:    VideoTuneZeroLatency,
		Threads:         1,
		UseCRF:          true,
		CRF:             23,
		VideoFilter:     fmt.Sprintf("scale=%d:%d", fixtureWidth, fixtureHeight),
		HLSTime:         fixtureSegmentSeconds,
		HLSFlags:        "independent_segments+append_list+program_date_time",
		HLSPlaylistType: "event",
		StartNumber:     int(startMicros / int64(fixtureSegmentSeconds*1_000_000)),
		ForceKeyFrames:  true,
		StartOffsetSec:  float64(startMicros) / 1_000_000,
		GOPSize:         spec.GOPSize,
	})
	args = serviceffmpeg.WithInputSeekMicros(args, startMicros)
	if durationMicros > 0 {
		var err error
		args, err = asBoundedStartupVODMicros(args, durationMicros)
		if err != nil {
			return nil, err
		}
	}
	return transcodetimestamp.ApplyFFmpeg(args, timestampPlan)
}

func abs64Certification(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func validateSourceOriginOutputPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("source origin output path is required")
	}
	return nil
}
