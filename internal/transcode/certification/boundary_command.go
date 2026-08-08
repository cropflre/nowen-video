package certification

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	serviceffmpeg "github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func produceBoundaryHLS(
	ctx context.Context,
	ffmpegPath,
	workDir,
	name,
	sourcePath string,
	timestampPlan transcodetimestamp.Plan,
	fixture FixtureSpec,
	startMicros,
	durationMicros int64,
) (string, error) {
	directory := filepath.Join(workDir, name)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create boundary %s directory: %w", name, err)
	}
	args, err := boundaryHLSArgs(sourcePath, directory, timestampPlan, fixture, startMicros, durationMicros)
	if err != nil {
		return "", fmt.Errorf("build boundary %s command: %w", name, err)
	}
	if err := runCommand(ctx, ffmpegPath, args...); err != nil {
		return "", fmt.Errorf("produce boundary %s fixture: %w", name, err)
	}
	return filepath.Join(directory, "stream.m3u8"), nil
}

func boundaryHLSArgs(
	sourcePath,
	outputDir string,
	timestampPlan transcodetimestamp.Plan,
	fixture FixtureSpec,
	startMicros,
	durationMicros int64,
) ([]string, error) {
	tune := ""
	if fixture.VideoTune == VideoTuneZeroLatency {
		tune = VideoTuneZeroLatency
	}
	startSeconds := float64(startMicros) / 1_000_000
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
		SoftwareTune:    tune,
		Threads:         1,
		UseCRF:          true,
		CRF:             23,
		VideoFilter:     fmt.Sprintf("scale=%d:%d", fixtureWidth, fixtureHeight),
		HLSTime:         fixtureSegmentSeconds,
		HLSFlags:        "independent_segments+append_list+program_date_time",
		HLSPlaylistType: "event",
		StartNumber:     int(startMicros / int64(fixtureSegmentSeconds*1_000_000)),
		ForceKeyFrames:  true,
		StartOffsetSec:  startSeconds,
		GOPSize:         fixture.FrameRate * fixtureSegmentSeconds,
	})
	args = serviceffmpeg.WithInputSeekMicros(args, startMicros)
	if fixture.VideoTune == VideoTuneDefault {
		var err error
		args, err = insertBeforeOutput(args, "-bf", "3")
		if err != nil {
			return nil, err
		}
	}
	if durationMicros > 0 {
		var err error
		args, err = asBoundedStartupVODMicros(args, durationMicros)
		if err != nil {
			return nil, err
		}
	}
	return transcodetimestamp.ApplyFFmpeg(args, timestampPlan)
}

func asBoundedStartupVODMicros(args []string, durationMicros int64) ([]string, error) {
	if durationMicros <= 0 {
		return nil, fmt.Errorf("boundary startup duration must be positive")
	}
	if len(args) == 0 || strings.TrimSpace(args[len(args)-1]) == "" {
		return nil, fmt.Errorf("ffmpeg arguments do not contain an output path")
	}
	body := make([]string, 0, len(args)+2)
	for index := 0; index < len(args)-1; index++ {
		arg := args[index]
		if arg == "-hls_playlist_type" && index+1 < len(args)-1 {
			index++
			continue
		}
		if arg == "-hls_flags" && index+1 < len(args)-1 {
			flags := strings.Split(args[index+1], "+")
			kept := make([]string, 0, len(flags))
			for _, flag := range flags {
				if flag != "append_list" {
					kept = append(kept, flag)
				}
			}
			body = append(body, arg, strings.Join(kept, "+"))
			index++
			continue
		}
		body = append(body, arg)
	}
	body = append(body, "-t", formatMicrosSeconds(durationMicros), "-hls_playlist_type", "vod")
	return append(body, args[len(args)-1]), nil
}

func formatMicrosSeconds(value int64) string {
	formatted := strconv.FormatFloat(float64(value)/1_000_000, 'f', 6, 64)
	parts := strings.SplitN(formatted, ".", 2)
	if len(parts) != 2 {
		return formatted
	}
	fraction := strings.TrimRight(parts[1], "0")
	for len(fraction) < 2 {
		fraction += "0"
	}
	return parts[0] + "." + fraction
}
