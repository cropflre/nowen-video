package service

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	transcodeprofile "github.com/nowen-video/nowen-video/internal/transcode/profile"
)

const hlsTargetSegmentSeconds = 2

// QualityConfig and qualityPresets remain as source-compatible adapters for
// legacy stream/on-demand code. Values are derived from the shared profile
// catalog, so there is still only one hard-coded quality authority.
type QualityConfig = transcodeprofile.EncodingProfile

var qualityPresets = runtimeQualityPresetMap()

func runtimeQualityPresetMap() map[string]QualityConfig {
	presets := make(map[string]QualityConfig)
	for _, name := range transcodeprofile.Names() {
		if preset, ok := transcodeprofile.Runtime(name); ok {
			presets[name] = preset
		}
	}
	return presets
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
	return s.buildFFmpegArgsForBackendWithProbe(
		media,
		s.probeMediaForPlan(media),
		inputPath,
		outputDir,
		quality,
		startOffset,
		backend,
	)
}

func (s *TranscodeService) buildFFmpegArgsForBackendWithProbe(
	media *model.Media,
	probe *model.MediaProbeRecord,
	inputPath,
	outputDir,
	quality string,
	startOffset float64,
	backend string,
) []string {
	qc, ok := transcodeprofile.Runtime(quality)
	if !ok {
		qc, _ = transcodeprofile.Runtime("720p")
	}

	var videoFilter string
	if backend == ffmpeg.HWAccelNone || backend == "" {
		videoFilter = buildSoftwareVideoFilter(probe, qc.Width, qc.Height)
	}

	startNumber := 0
	if startOffset > 0 {
		startNumber = int(startOffset / float64(hlsTargetSegmentSeconds))
	}

	qsvGlobalQuality := 0
	if backend == ffmpeg.HWAccelQSV {
		qsvGlobalQuality = 23
	}

	gopSize := hlsTargetSegmentSeconds * 25
	if probe != nil {
		gopSize = probe.GOPSize(hlsTargetSegmentSeconds)
	}

	args := ffmpeg.BuildHLSArgs(ffmpeg.BuildOptions{
		InputPath:             inputPath,
		OutputDir:             outputDir,
		HWAccel:               backend,
		Profile:               ffmpeg.Profile{Width: qc.Width, Height: qc.Height, VideoBitrate: qc.VideoBitrate, AudioBitrate: qc.AudioBitrate, MaxBitrate: qc.MaxBitrate, BufSize: qc.BufSize},
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
		GOPSize:               gopSize,
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
	if media == nil {
		return []string{"360p", "480p", "720p", "1080p"}
	}
	origHeight := parseResolutionHeight(media.Resolution)
	if origHeight <= 0 {
		return []string{"360p", "480p", "720p", "1080p"}
	}
	available := transcodeprofile.NamesUpToHeight(origHeight)
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

func fitScaleFilter(width, height int) string {
	return fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,"+
			"pad=%d:%d:(ow-iw)/2:(oh-ih)/2:color=black,setsar=1",
		width,
		height,
		width,
		height,
	)
}

func buildSoftwareVideoFilter(probe *model.MediaProbeRecord, width, height int) string {
	fit := fitScaleFilter(width, height)
	if probe == nil || !probe.HDR {
		return fit
	}
	// HDR is determined from transfer characteristics or explicit mastering /
	// content-light / Dolby Vision side data. Codec names alone never enable
	// tone mapping, so ordinary SDR HEVC is not unnecessarily transformed.
	return "zscale=t=linear:npl=100,format=gbrpf32le," +
		"tonemap=hable:desat=0," +
		"zscale=p=bt709:t=bt709:m=bt709:r=tv," +
		"format=yuv420p," + fit
}

// Kept for source compatibility with older tests and adapters. The Media
// summary does not contain reliable HDR metadata, therefore this method now
// returns the safe SDR fit filter instead of guessing from the codec.
func (s *TranscodeService) buildFFmpegHDRTonemapFilter(_ *model.Media, width, height int) string {
	return buildSoftwareVideoFilter(nil, width, height)
}
