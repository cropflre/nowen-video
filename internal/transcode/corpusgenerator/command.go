package corpusgenerator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
)

const GeneratorVersion = "deterministic-real-media-corpus-generator-v1"
const RepeatCount = 2

type CommandPlan struct {
	Args          []string
	RelativePath  string
	CommandSHA256 string
}

func BuildCommand(caseSpec transcodecorpus.CaseSpec, outputPath string) (CommandPlan, error) {
	if err := caseSpec.Validate(); err != nil {
		return CommandPlan{}, err
	}
	if caseSpec.Tier != transcodecorpus.TierDeterministicContainer {
		return CommandPlan{}, fmt.Errorf("case %s is not generator-owned", caseSpec.ID)
	}
	plan := caseSpec.Source
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-fflags", "+bitexact"}
	if plan.Video.FrameRateMode == transcodecorpus.FrameRateCFR {
		rate := formatRational(plan.Video.FrameRates[0])
		args = append(args,
			"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=size=%dx%d:rate=%s:duration=%s", plan.Video.Width, plan.Video.Height, rate, formatMicros(plan.Timeline.DurationMicros)),
			"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=1000:sample_rate=%d:duration=%s", plan.Audio.SampleRate, formatMicros(plan.Timeline.DurationMicros)),
			"-map", "0:v:0", "-map", "1:a:0",
		)
	} else {
		segmentMicros := plan.Timeline.DurationMicros / int64(len(plan.Video.FrameRates))
		if segmentMicros*int64(len(plan.Video.FrameRates)) != plan.Timeline.DurationMicros {
			return CommandPlan{}, fmt.Errorf("case %s VFR duration cannot be divided evenly", caseSpec.ID)
		}
		for _, rate := range plan.Video.FrameRates {
			args = append(args, "-f", "lavfi", "-i", fmt.Sprintf(
				"testsrc2=size=%dx%d:rate=%s:duration=%s",
				plan.Video.Width, plan.Video.Height, formatRational(rate), formatMicros(segmentMicros),
			))
		}
		audioIndex := len(plan.Video.FrameRates)
		args = append(args,
			"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=1000:sample_rate=%d:duration=%s", plan.Audio.SampleRate, formatMicros(plan.Timeline.DurationMicros)),
			"-filter_complex", vfrFilter(len(plan.Video.FrameRates)),
			"-map", "[v]", "-map", fmt.Sprintf("%d:a:0", audioIndex),
		)
	}

	x264Params := fmt.Sprintf(
		"bframes=%d:b-adapt=0:b-pyramid=none:open-gop=0:scenecut=0:keyint=%d:min-keyint=%d:ref=%d:threads=1:lookahead-threads=1:sliced-threads=0:colorprim=%s:transfer=%s:colormatrix=%s",
		plan.Video.BFrames,
		plan.Video.GOPSize,
		plan.Video.GOPSize,
		plan.Video.ReferenceFrames,
		plan.Video.ColorPrimaries,
		plan.Video.ColorTransfer,
		plan.Video.ColorMatrix,
	)
	args = append(args,
		"-threads:v", "1",
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "23",
		"-profile:v", plan.Video.Profile,
		"-pix_fmt", plan.Video.PixelFormat,
		"-g", strconv.Itoa(plan.Video.GOPSize),
		"-keyint_min", strconv.Itoa(plan.Video.GOPSize),
		"-sc_threshold", "0",
		"-bf", strconv.Itoa(plan.Video.BFrames),
		"-refs", strconv.Itoa(plan.Video.ReferenceFrames),
		"-b_strategy", "0",
		"-x264-params", x264Params,
		"-color_primaries", plan.Video.ColorPrimaries,
		"-color_trc", plan.Video.ColorTransfer,
		"-colorspace", plan.Video.ColorMatrix,
		"-fps_mode", "passthrough",
	)
	switch plan.Audio.Codec {
	case transcodecorpus.CodecAAC:
		args = append(args, "-c:a", "aac")
	case transcodecorpus.CodecOpus:
		args = append(args, "-c:a", "libopus")
	default:
		return CommandPlan{}, fmt.Errorf("unsupported audio codec %q", plan.Audio.Codec)
	}
	args = append(args,
		"-b:a", "128k",
		"-ar", strconv.Itoa(plan.Audio.SampleRate),
		"-ac", strconv.Itoa(plan.Audio.Channels),
		"-map_metadata", "-1",
		"-flags:v", "+bitexact",
		"-flags:a", "+bitexact",
		"-bitexact",
	)
	if plan.Timeline.OriginMicros != 0 {
		args = append(args, "-output_ts_offset", formatMicros(plan.Timeline.OriginMicros), "-avoid_negative_ts", "disabled")
	}
	switch plan.Container {
	case transcodecorpus.ContainerMP4:
		args = append(args,
			"-metadata", "creation_time=1970-01-01T00:00:00Z",
			"-movflags", "+faststart",
			"-use_editlist", boolInt(plan.Timeline.HasEditList),
			"-f", "mp4",
		)
	case transcodecorpus.ContainerMatroska:
		args = append(args, "-write_crc32", "0", "-f", "matroska")
	case transcodecorpus.ContainerMPEGTS:
		args = append(args,
			"-muxdelay", "0",
			"-muxpreload", "0",
			"-mpegts_flags", "+resend_headers",
			"-f", "mpegts",
		)
	default:
		return CommandPlan{}, fmt.Errorf("unsupported container %q", plan.Container)
	}
	args = append(args, outputPath)
	relativePath := filepath.ToSlash(filepath.Join("assets", caseSpec.ID+extensionFor(plan.Container)))
	return CommandPlan{
		Args:          args,
		RelativePath:  relativePath,
		CommandSHA256: normalizedCommandHash(args),
	}, nil
}

func extensionFor(container string) string {
	switch container {
	case transcodecorpus.ContainerMP4:
		return ".mp4"
	case transcodecorpus.ContainerMatroska:
		return ".mkv"
	case transcodecorpus.ContainerMPEGTS:
		return ".ts"
	default:
		return ".media"
	}
}

func vfrFilter(count int) string {
	parts := make([]string, 0, count+1)
	inputs := strings.Builder{}
	for index := 0; index < count; index++ {
		parts = append(parts, fmt.Sprintf("[%d:v]settb=AVTB,setpts=PTS-STARTPTS[v%d]", index, index))
		inputs.WriteString(fmt.Sprintf("[v%d]", index))
	}
	parts = append(parts, fmt.Sprintf("%sconcat=n=%d:v=1:a=0[v]", inputs.String(), count))
	return strings.Join(parts, ";")
}

func normalizedCommandHash(args []string) string {
	normalized := append([]string(nil), args...)
	if len(normalized) > 0 {
		normalized[len(normalized)-1] = "<OUTPUT>"
	}
	digest := sha256.Sum256([]byte("ffmpeg\x00" + strings.Join(normalized, "\x00")))
	return hex.EncodeToString(digest[:])
}

func formatRational(value transcodecorpus.Rational) string {
	if value.Denominator == 1 {
		return strconv.FormatInt(value.Numerator, 10)
	}
	return fmt.Sprintf("%d/%d", value.Numerator, value.Denominator)
}

func formatMicros(value int64) string {
	return strconv.FormatFloat(float64(value)/1_000_000, 'f', 6, 64)
}

func boolInt(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
