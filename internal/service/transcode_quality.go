package service

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/service/ffmpeg"
)

const hlsTargetSegmentSeconds = 2

var qualityPresets = map[string]QualityConfig{
	"360p":  {Width: 640, Height: 360, VideoBitrate: "800k", AudioBitrate: "96k"},
	"480p":  {Width: 854, Height: 480, VideoBitrate: "1500k", AudioBitrate: "128k"},
	"720p":  {Width: 1280, Height: 720, VideoBitrate: "3000k", AudioBitrate: "128k"},
	"1080p": {Width: 1920, Height: 1080, VideoBitrate: "6000k", AudioBitrate: "192k"},
	"2K":    {Width: 2560, Height: 1440, VideoBitrate: "12000k", AudioBitrate: "192k"},
	"4K":    {Width: 3840, Height: 2160, VideoBitrate: "25000k", AudioBitrate: "256k"},
}

type QualityConfig struct {
	Width        int
	Height       int
	VideoBitrate string
	AudioBitrate string
}

func (s *TranscodeService) GetOutputDir(mediaID, quality string) string {
	return filepath.Join(s.cfg.Cache.CacheDir, "transcode", mediaID, quality)
}

// buildFFmpegArgs remains as a compatibility wrapper for existing tests and
// callers. Attempts use buildFFmpegArgsForBackend so fallback never mutates the
// service-wide hardware backend.
func (s *TranscodeService) buildFFmpegArgs(media *model.Media, inputPath, outputDir, quality string, startOffset float64) []string {
	return s.buildFFmpegArgsForBackend(media, inputPath, outputDir, quality, startOffset, s.hwAccel)
}

func (s *TranscodeService) buildFFmpegArgsForBackend(media *model.Media, inputPath, outputDir, quality string, startOffset float64, backend string) []string {
	qc, ok := qualityPresets[quality]
	if !ok {
		qc = qualityPresets["720p"]
	}

	var videoFilter string
	if backend == ffmpeg.HWAccelNone || backend == "" {
		videoFilter = s.buildFFmpegHDRTonemapFilter(media, qc.Width, qc.Height)
	}

	startNumber := 0
	if startOffset > 0 {
		startNumber = int(startOffset / float64(hlsTargetSegmentSeconds))
	}

	qsvGlobalQuality := 0
	if backend == ffmpeg.HWAccelQSV {
		qsvGlobalQuality = 23
	}

	args := ffmpeg.BuildHLSArgs(ffmpeg.BuildOptions{
		InputPath:             inputPath,
		OutputDir:             outputDir,
		HWAccel:               backend,
		Profile:               ffmpeg.Profile{Width: qc.Width, Height: qc.Height, VideoBitrate: qc.VideoBitrate, AudioBitrate: qc.AudioBitrate},
		VAAPIDevice:           s.cfg.App.VAAPIDevice,
		X264Preset:            "ultrafast",
		QSVPreset:             "ultrafast",
		Threads:               ffmpeg.CalcThreads(s.cfg),
		UseCRF:                true,
		CRF:                   23,
		SoftwareTune:          "zerolatency",
		NvencTune:             "ll",
		QSVAttachOutputFormat: false,
		QSVGlobalQuality:      qsvGlobalQuality,
		VideoFilter:           videoFilter,
		HLSTime:               hlsTargetSegmentSeconds,
		HLSFlags:              "independent_segments+append_list+program_date_time",
		HLSPlaylistType:       "event",
		StartNumber:           startNumber,
		ForceKeyFrames:        true,
		StartOffsetSec:        startOffset,
		GOPSize:               hlsTargetSegmentSeconds * 25,
		SkipVAAPIRateLimits:   true,
	})
	return withMachineProgress(args)
}

func withMachineProgress(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	result := make([]string, 0, len(args)+4)
	result = append(result, args[:len(args)-1]...)
	result = append(result, "-progress", "pipe:2", "-nostats")
	result = append(result, args[len(args)-1])
	return result
}

func (s *TranscodeService) GetAvailableQualities(media *model.Media) []string {
	origHeight := parseResolutionHeight(media.Resolution)
	if origHeight <= 0 {
		return []string{"360p", "480p", "720p", "1080p"}
	}
	ordered := []struct {
		name   string
		height int
	}{
		{"360p", 360},
		{"480p", 480},
		{"720p", 720},
		{"1080p", 1080},
		{"2K", 1440},
		{"4K", 2160},
	}
	available := make([]string, 0, len(ordered))
	for _, candidate := range ordered {
		if candidate.height <= origHeight {
			available = append(available, candidate.name)
		}
	}
	if len(available) == 0 {
		return []string{"360p"}
	}
	return available
}

func parseResolutionHeight(resolution string) int {
	switch resolution {
	case "4K":
		return 2160
	case "2K":
		return 1440
	case "1080p":
		return 1080
	case "720p":
		return 720
	case "480p":
		return 480
	case "360p":
		return 360
	default:
		if strings.HasSuffix(resolution, "p") {
			if height, err := strconv.Atoi(strings.TrimSuffix(resolution, "p")); err == nil {
				return height
			}
		}
		return 0
	}
}

func (s *TranscodeService) buildFFmpegHDRTonemapFilter(media *model.Media, width, height int) string {
	codec := strings.ToLower(media.VideoCodec)
	isHDR := codec == "hevc" || codec == "h265" || codec == "vp9" || codec == "av1"
	if !isHDR {
		return fmt.Sprintf("scale=%d:%d", width, height)
	}
	return fmt.Sprintf(
		"zscale=t=linear:npl=100,format=gbrpf32le,"+
			"tonemap=hable:desat=0,"+
			"zscale=p=bt709:t=bt709:m=bt709:r=tv,"+
			"format=yuv420p,scale=%d:%d",
		width, height,
	)
}
