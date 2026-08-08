package certification

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	serviceffmpeg "github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

// sourceOriginInputGraph is both the FFprobe evidence source and the FFmpeg HLS
// input. Using the same lavfi graph avoids an intermediate container silently
// shifting or rejecting negative timestamps.
func sourceOriginInputGraph(spec SourceOriginCaseSpec) string {
	offset := sourceOriginOffsetExpression(spec.SourceOffsetMicros)
	if spec.SourceMode == transcodesourceorigin.ModeVFR {
		return fmt.Sprintf(
			"testsrc2=size=%dx%d:rate=24:duration=20,settb=AVTB[v0];"+
				"testsrc2=size=%dx%d:rate=30:duration=20,settb=AVTB[v1];"+
				"[v0][v1]concat=n=2:v=1:a=0,setpts=%s[out0];"+
				"sine=frequency=1000:sample_rate=%d:duration=%d,asettb=1/%d,asetpts=%s[out1]",
			fixtureWidth,
			fixtureHeight,
			fixtureWidth,
			fixtureHeight,
			offset,
			spec.AudioSampleRate,
			sourceOriginDurationSeconds,
			spec.AudioSampleRate,
			offset,
		)
	}
	return fmt.Sprintf(
		"testsrc2=size=%dx%d:rate=%s:duration=%d,settb=AVTB,setpts=%s[out0];"+
			"sine=frequency=1000:sample_rate=%d:duration=%d,asettb=1/%d,asetpts=%s[out1]",
		fixtureWidth,
		fixtureHeight,
		sourceOriginRateExpression(spec),
		sourceOriginDurationSeconds,
		offset,
		spec.AudioSampleRate,
		sourceOriginDurationSeconds,
		spec.AudioSampleRate,
		offset,
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
	sourceGraph string,
	timestampPlan transcodetimestamp.Plan,
	spec SourceOriginCaseSpec,
	startMicros,
	durationMicros int64,
) (string, error) {
	directory := filepath.Join(workDir, name)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create source origin %s directory: %w", name, err)
	}
	args, err := sourceOriginHLSArgs(sourceGraph, directory, timestampPlan, spec, startMicros, durationMicros)
	if err != nil {
		return "", fmt.Errorf("build source origin %s command: %w", name, err)
	}
	if err := runCommand(ctx, ffmpegPath, args...); err != nil {
		return "", fmt.Errorf("produce source origin %s fixture: %w", name, err)
	}
	return filepath.Join(directory, "stream.m3u8"), nil
}

func sourceOriginHLSArgs(
	sourceGraph,
	outputDir string,
	timestampPlan transcodetimestamp.Plan,
	spec SourceOriginCaseSpec,
	startMicros,
	durationMicros int64,
) ([]string, error) {
	args := serviceffmpeg.BuildHLSArgs(serviceffmpeg.BuildOptions{
		InputPath:  sourceGraph,
		OutputDir:  outputDir,
		ExtraInput: []string{"-f", "lavfi"},
		HWAccel:    serviceffmpeg.HWAccelNone,
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
