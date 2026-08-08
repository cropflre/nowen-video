package certification

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serviceffmpeg "github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func produceRealMediaCandidate(
	ctx context.Context,
	ffmpegPath,
	outputDir,
	sourcePath string,
	timestampPlan transcodetimestamp.Plan,
	caseSpec transcodereorder.CaseSpec,
	candidateSpec transcodetimebase.CandidateSpec,
	startMicros,
	durationMicros int64,
) (encoderTimeBaseProduced, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return encoderTimeBaseProduced{}, err
	}
	args, err := realMediaCandidateHLSArgs(sourcePath, outputDir, timestampPlan, caseSpec, candidateSpec, startMicros, durationMicros)
	if err != nil {
		return encoderTimeBaseProduced{}, err
	}
	if err := runCommand(ctx, ffmpegPath, args...); err != nil {
		return encoderTimeBaseProduced{}, err
	}
	return encoderTimeBaseProduced{Manifest: filepath.Join(outputDir, "stream.m3u8"), Args: args}, nil
}

func realMediaCandidateHLSArgs(
	sourcePath,
	outputDir string,
	timestampPlan transcodetimestamp.Plan,
	caseSpec transcodereorder.CaseSpec,
	candidateSpec transcodetimebase.CandidateSpec,
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
		GOPSize:         caseSpec.Base.GOPSize,
	})
	args = serviceffmpeg.WithInputSeekMicros(args, startMicros)
	if durationMicros > 0 {
		var err error
		args, err = asBoundedStartupVODMicros(args, durationMicros)
		if err != nil {
			return nil, err
		}
	}
	args, err := transcodetimestamp.ApplyFFmpeg(args, timestampPlan)
	if err != nil {
		return nil, err
	}
	args = removeReorderOptionPair(args, "-tune", VideoTuneZeroLatency)
	x264Params := fmt.Sprintf("b-adapt=%d:b-pyramid=none:open-gop=0:scenecut=0", caseSpec.BAdapt)
	return insertIsolationBeforeOutput(args,
		"-enc_time_base:v:0", candidateSpec.EncoderTimeBase,
		"-bf", fmt.Sprint(caseSpec.BFrames),
		"-b_strategy", fmt.Sprint(caseSpec.BAdapt),
		"-refs", fmt.Sprint(caseSpec.ReferenceFrames),
		"-x264-params", x264Params,
	), nil
}

func probeRealMediaSourceTimelines(
	ctx context.Context,
	ffprobePath,
	sourcePath string,
	caseSpec transcodetimebase.CaseSpec,
) (transcodeoutputcadence.TimelineEvidence, transcodeoutputcadence.TimelineEvidence, error) {
	full, points, err := probeVideoCadenceTimeline(
		ctx,
		ffprobePath,
		outputCadenceProbeInput{Path: sourcePath},
		"real_media_source_full_"+caseSpec.ID,
		caseSpec.SourceOffsetMicros,
		caseSpec.SourceOffsetMicros+caseSpec.DurationMicros,
	)
	if err != nil {
		return transcodeoutputcadence.TimelineEvidence{}, transcodeoutputcadence.TimelineEvidence{}, err
	}
	startupTicks := make([]int64, 0, len(points))
	continuationTicks := make([]int64, 0, len(points))
	for _, point := range points {
		relative := point.Micros - caseSpec.SourceOffsetMicros
		switch {
		case relative >= 0 && relative < caseSpec.ExpectedBoundaryMicros:
			startupTicks = append(startupTicks, point.Ticks)
		case relative >= caseSpec.ExpectedBoundaryMicros && relative < caseSpec.DurationMicros:
			continuationTicks = append(continuationTicks, point.Ticks)
		}
	}
	startup, err := transcodeoutputcadence.NewTimelineEvidence(
		transcodeoutputcadence.TimelineSourceStartup,
		full.TimeBase,
		caseSpec.SourceOffsetMicros,
		caseSpec.SourceOffsetMicros+caseSpec.ExpectedBoundaryMicros,
		startupTicks,
	)
	if err != nil {
		return transcodeoutputcadence.TimelineEvidence{}, transcodeoutputcadence.TimelineEvidence{}, err
	}
	continuation, err := transcodeoutputcadence.NewTimelineEvidence(
		transcodeoutputcadence.TimelineSourceContinuation,
		full.TimeBase,
		caseSpec.SourceOffsetMicros+caseSpec.ExpectedBoundaryMicros,
		caseSpec.SourceOffsetMicros+caseSpec.DurationMicros,
		continuationTicks,
	)
	if err != nil {
		return transcodeoutputcadence.TimelineEvidence{}, transcodeoutputcadence.TimelineEvidence{}, err
	}
	return startup, continuation, nil
}

func realMediaSourceOriginSpec(caseSpec transcodereorder.CaseSpec) SourceOriginCaseSpec {
	numerator := caseSpec.Base.PrimaryFrameRateNumerator
	denominator := caseSpec.Base.PrimaryFrameRateDenominator
	if caseSpec.Base.SourceMode == transcodesourceorigin.ModeVFR {
		numerator = int64(caseSpec.Base.DeclaredFrameRateMilli())
		denominator = 1_000
	}
	return SourceOriginCaseSpec{
		ID:                           caseSpec.Base.ID,
		Description:                  caseSpec.Base.Description,
		FixtureID:                    "real-media-" + caseSpec.Base.ID,
		SourceMode:                   caseSpec.Base.SourceMode,
		DeclaredFrameRateNumerator:   numerator,
		DeclaredFrameRateDenominator: denominator,
		SourceOffsetMicros:           caseSpec.Base.SourceOffsetMicros,
		AudioSampleRate:              caseSpec.Base.AudioSampleRate,
		GOPSize:                      caseSpec.Base.GOPSize,
		ExpectedBoundaryMicros:       caseSpec.Base.ExpectedBoundaryMicros,
	}
}

func hashRealMediaArgs(args []string, workDir, sourcePath string) string {
	normalized := append([]string(nil), args...)
	for index, value := range normalized {
		normalized[index] = strings.ReplaceAll(value, sourcePath, "$SOURCE")
	}
	return hashNormalizedArgs(normalized, workDir)
}
