package certification

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	serviceffmpeg "github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	transcodelongdrift "github.com/nowen-video/nowen-video/internal/transcode/longdrift"
	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func produceLongDurationDriftCandidate(
	ctx context.Context,
	ffmpegPath,
	outputDir,
	sourcePath string,
	timestampPlan transcodetimestamp.Plan,
	caseSpec transcodereorder.CaseSpec,
	candidateSpec transcodetimebase.CandidateSpec,
) (encoderTimeBaseProduced, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return encoderTimeBaseProduced{}, err
	}
	args, err := longDurationDriftHLSArgs(sourcePath, outputDir, timestampPlan, caseSpec, candidateSpec)
	if err != nil {
		return encoderTimeBaseProduced{}, err
	}
	if err := runCommand(ctx, ffmpegPath, args...); err != nil {
		return encoderTimeBaseProduced{}, err
	}
	return encoderTimeBaseProduced{Manifest: filepath.Join(outputDir, "stream.m3u8"), Args: args}, nil
}

func longDurationDriftHLSArgs(
	sourcePath,
	outputDir string,
	timestampPlan transcodetimestamp.Plan,
	caseSpec transcodereorder.CaseSpec,
	candidateSpec transcodetimebase.CandidateSpec,
) ([]string, error) {
	args := serviceffmpeg.BuildHLSArgs(serviceffmpeg.BuildOptions{
		InputPath: sourcePath,
		OutputDir: outputDir,
		ExtraInput: []string{"-stream_loop", "-1"},
		HWAccel: serviceffmpeg.HWAccelNone,
		Profile: serviceffmpeg.Profile{
			Width: fixtureWidth,
			Height: fixtureHeight,
			VideoBitrate: "800k",
			AudioBitrate: "128k",
			MaxBitrate: "900k",
			BufSize: "1600k",
		},
		X264Preset: "veryfast",
		SoftwareTune: VideoTuneZeroLatency,
		Threads: 1,
		UseCRF: true,
		CRF: 23,
		VideoFilter: fmt.Sprintf("scale=%d:%d", fixtureWidth, fixtureHeight),
		HLSTime: fixtureSegmentSeconds,
		HLSFlags: "independent_segments+append_list+program_date_time",
		HLSPlaylistType: "event",
		StartNumber: 0,
		ForceKeyFrames: true,
		GOPSize: caseSpec.Base.GOPSize,
	})
	var err error
	args, err = asBoundedStartupVODMicros(args, transcodelongdrift.DurationMicros)
	if err != nil {
		return nil, err
	}
	args, err = transcodetimestamp.ApplyFFmpeg(args, timestampPlan)
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
