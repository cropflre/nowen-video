package corpusgenerator

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
)

type probeDocument struct {
	Streams []probeStream `json:"streams"`
	Format  probeFormat   `json:"format"`
}

type probeFormat struct {
	FormatName string `json:"format_name"`
}

type probeStream struct {
	Index          int    `json:"index"`
	CodecType      string `json:"codec_type"`
	CodecName      string `json:"codec_name"`
	Profile        string `json:"profile"`
	PixelFormat    string `json:"pix_fmt"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	ColorSpace     string `json:"color_space"`
	ColorTransfer  string `json:"color_transfer"`
	ColorPrimaries string `json:"color_primaries"`
	TimeBase       string `json:"time_base"`
	SampleRate     string `json:"sample_rate"`
	Channels       int    `json:"channels"`
	HasBFrames     int    `json:"has_b_frames"`
}

type packetDocument struct {
	Packets []probePacket `json:"packets"`
}

type probePacket struct {
	PTS      flexibleInt64 `json:"pts"`
	DTS      flexibleInt64 `json:"dts"`
	Duration flexibleInt64 `json:"duration"`
	Flags    string        `json:"flags"`
}

type flexibleInt64 struct {
	Value int64
	Valid bool
}

func (f *flexibleInt64) UnmarshalJSON(data []byte) error {
	value := strings.TrimSpace(string(data))
	if value == "" || value == "null" || value == `"N/A"` {
		*f = flexibleInt64{}
		return nil
	}
	if strings.HasPrefix(value, `"`) {
		var decoded string
		if err := json.Unmarshal(data, &decoded); err != nil {
			return err
		}
		value = decoded
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return err
	}
	*f = flexibleInt64{Value: parsed, Valid: true}
	return nil
}

type packetPoint struct {
	PTS      int64
	DTS      flexibleInt64
	Duration flexibleInt64
	KeyFrame bool
	Decode   int
}

func ProbeAsset(ctx context.Context, runner Runner, ffprobePath, mediaPath string, plan transcodecorpus.SourcePlan) (transcodecorpus.ProbeEvidence, error) {
	metadataRaw, err := runner.Run(ctx, ffprobePath,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-show_entries", "format=format_name:stream=index,codec_type,codec_name,profile,pix_fmt,width,height,color_space,color_transfer,color_primaries,time_base,sample_rate,channels,has_b_frames",
		mediaPath,
	)
	if err != nil {
		return transcodecorpus.ProbeEvidence{}, err
	}
	var metadata probeDocument
	if err := json.Unmarshal(metadataRaw, &metadata); err != nil {
		return transcodecorpus.ProbeEvidence{}, fmt.Errorf("decode ffprobe stream metadata: %w", err)
	}
	var video *probeStream
	audio := make([]probeStream, 0, 2)
	for index := range metadata.Streams {
		stream := &metadata.Streams[index]
		switch stream.CodecType {
		case "video":
			if video == nil {
				video = stream
			}
		case "audio":
			audio = append(audio, *stream)
		}
	}
	if video == nil || len(audio) == 0 {
		return transcodecorpus.ProbeEvidence{}, fmt.Errorf("corpus asset must contain video and audio")
	}

	packetsRaw, err := runner.Run(ctx, ffprobePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-print_format", "json",
		"-show_packets",
		"-show_entries", "packet=pts,dts,duration,flags",
		mediaPath,
	)
	if err != nil {
		return transcodecorpus.ProbeEvidence{}, err
	}
	var packetDoc packetDocument
	if err := json.Unmarshal(packetsRaw, &packetDoc); err != nil {
		return transcodecorpus.ProbeEvidence{}, fmt.Errorf("decode ffprobe packet metadata: %w", err)
	}
	videoTimeBase, err := parseRational(video.TimeBase)
	if err != nil {
		return transcodecorpus.ProbeEvidence{}, fmt.Errorf("video time base: %w", err)
	}
	audioTimeBase, err := parseRational(audio[0].TimeBase)
	if err != nil {
		return transcodecorpus.ProbeEvidence{}, fmt.Errorf("audio time base: %w", err)
	}

	points := make([]packetPoint, 0, len(packetDoc.Packets))
	for index, packet := range packetDoc.Packets {
		if !packet.PTS.Valid {
			return transcodecorpus.ProbeEvidence{}, fmt.Errorf("video packet %d has no PTS", index)
		}
		points = append(points, packetPoint{
			PTS:      packet.PTS.Value,
			DTS:      packet.DTS,
			Duration: packet.Duration,
			KeyFrame: strings.Contains(packet.Flags, "K"),
			Decode:   index,
		})
	}
	if len(points) < 2 {
		return transcodecorpus.ProbeEvidence{}, fmt.Errorf("video packet set is incomplete")
	}
	presentation := append([]packetPoint(nil), points...)
	sort.SliceStable(presentation, func(i, j int) bool {
		if presentation[i].PTS == presentation[j].PTS {
			return presentation[i].Decode < presentation[j].Decode
		}
		return presentation[i].PTS < presentation[j].PTS
	})
	for index := 1; index < len(presentation); index++ {
		if presentation[index].PTS <= presentation[index-1].PTS {
			return transcodecorpus.ProbeEvidence{}, fmt.Errorf("presentation PTS is not strictly increasing")
		}
	}

	startMicros := ticksToMicros(presentation[0].PTS, videoTimeBase)
	deltas := make([]int64, 0, len(presentation)-1)
	for index := 1; index < len(presentation); index++ {
		deltas = append(deltas, ticksToMicros(presentation[index].PTS-presentation[index-1].PTS, videoTimeBase))
	}
	lastDuration := int64(0)
	if presentation[len(presentation)-1].Duration.Valid && presentation[len(presentation)-1].Duration.Value > 0 {
		lastDuration = ticksToMicros(presentation[len(presentation)-1].Duration.Value, videoTimeBase)
	}
	if lastDuration <= 0 {
		lastDuration = deltas[len(deltas)-1]
	}
	durationMicros := ticksToMicros(presentation[len(presentation)-1].PTS-presentation[0].PTS, videoTimeBase) + lastDuration
	observedRates, err := classifyRates(deltas, videoTimeBase, plan.Video.FrameRates)
	if err != nil {
		return transcodecorpus.ProbeEvidence{}, err
	}

	keyFrames := 0
	keyPositions := make([]int, 0)
	for index, point := range presentation {
		if point.KeyFrame {
			keyFrames++
			keyPositions = append(keyPositions, index)
		}
	}
	maxKeyInterval := 0
	for index := 1; index < len(keyPositions); index++ {
		if delta := keyPositions[index] - keyPositions[index-1]; delta > maxKeyInterval {
			maxKeyInterval = delta
		}
	}

	rank := make([]int, len(points))
	for presentationIndex, point := range presentation {
		rank[point.Decode] = presentationIndex
	}
	maxReorderDepth := 0
	ptsInversions := 0
	for index := range points {
		if delta := rank[index] - index; delta > maxReorderDepth {
			maxReorderDepth = delta
		}
		if index > 0 && points[index].PTS < points[index-1].PTS {
			ptsInversions++
		}
	}
	maxComposition := int64(0)
	for _, point := range points {
		if !point.DTS.Valid {
			continue
		}
		composition := ticksToMicros(point.PTS-point.DTS.Value, videoTimeBase)
		if composition > maxComposition {
			maxComposition = composition
		}
	}

	audioSampleRate, err := strconv.Atoi(audio[0].SampleRate)
	if err != nil {
		return transcodecorpus.ProbeEvidence{}, fmt.Errorf("audio sample rate is invalid: %w", err)
	}
	container, err := normalizeContainer(metadata.Format.FormatName)
	if err != nil {
		return transcodecorpus.ProbeEvidence{}, err
	}
	hasEditList := false
	if container == transcodecorpus.ContainerMP4 {
		hasEditList, err = containsMP4Box(mediaPath, "edts")
		if err != nil {
			return transcodecorpus.ProbeEvidence{}, err
		}
	}

	return transcodecorpus.ProbeEvidence{
		Container:                   container,
		DurationMicros:              durationMicros,
		StartMicros:                 startMicros,
		VideoCodec:                  video.CodecName,
		VideoProfile:                strings.ToLower(video.Profile),
		PixelFormat:                 video.PixelFormat,
		Width:                       video.Width,
		Height:                      video.Height,
		ColorPrimaries:              video.ColorPrimaries,
		ColorTransfer:               video.ColorTransfer,
		ColorMatrix:                 video.ColorSpace,
		FrameRateMode:               plan.Video.FrameRateMode,
		ObservedRates:               observedRates,
		VideoTimeBase:               videoTimeBase,
		FrameCount:                  len(points),
		KeyFrameCount:               keyFrames,
		MaxKeyFrameInterval:         maxKeyInterval,
		MaxPresentationReorderDepth: maxReorderDepth,
		MaxCompositionOffsetMicros:  maxComposition,
		AudioCodec:                  audio[0].CodecName,
		AudioSampleRate:             audioSampleRate,
		AudioChannels:               audio[0].Channels,
		AudioTrackCount:             len(audio),
		AudioTimeBase:               audioTimeBase,
		HasBFrameReorder:            video.HasBFrames > 0 && ptsInversions > 0 && maxReorderDepth > 0,
		HasEditList:                 hasEditList,
	}, nil
}

func classifyRates(deltas []int64, timeBase transcodecorpus.Rational, expected []transcodecorpus.Rational) ([]transcodecorpus.Rational, error) {
	counts := make([]int, len(expected))
	quantization := int64(math.Ceil(1_000_000 * float64(timeBase.Numerator) / float64(timeBase.Denominator)))
	if quantization < 2 {
		quantization = 2
	}
	tolerance := quantization + 2
	for _, delta := range deltas {
		bestIndex := -1
		bestDistance := int64(math.MaxInt64)
		for index, rate := range expected {
			want := int64(math.Round(1_000_000 * float64(rate.Denominator) / float64(rate.Numerator)))
			distance := abs64(delta - want)
			if distance < bestDistance {
				bestDistance = distance
				bestIndex = index
			}
		}
		if bestIndex < 0 || bestDistance > tolerance {
			return nil, fmt.Errorf("observed frame delta %d microseconds does not match declared rates", delta)
		}
		counts[bestIndex]++
	}
	for index, count := range counts {
		if count < 2 {
			return nil, fmt.Errorf("declared frame rate %s is not materially observed", formatRational(expected[index]))
		}
	}
	return append([]transcodecorpus.Rational(nil), expected...), nil
}

func normalizeContainer(formatName string) (string, error) {
	switch {
	case strings.Contains(formatName, "mp4") || strings.Contains(formatName, "mov"):
		return transcodecorpus.ContainerMP4, nil
	case strings.Contains(formatName, "matroska"):
		return transcodecorpus.ContainerMatroska, nil
	case strings.Contains(formatName, "mpegts"):
		return transcodecorpus.ContainerMPEGTS, nil
	default:
		return "", fmt.Errorf("unsupported observed container %q", formatName)
	}
}

func parseRational(value string) (transcodecorpus.Rational, error) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return transcodecorpus.Rational{}, fmt.Errorf("invalid rational %q", value)
	}
	numerator, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return transcodecorpus.Rational{}, err
	}
	denominator, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return transcodecorpus.Rational{}, err
	}
	result := transcodecorpus.Rational{Numerator: numerator, Denominator: denominator}
	return result, result.Validate()
}

func ticksToMicros(ticks int64, timeBase transcodecorpus.Rational) int64 {
	numerator := ticks * timeBase.Numerator * 1_000_000
	if numerator >= 0 {
		return (numerator + timeBase.Denominator/2) / timeBase.Denominator
	}
	return -((-numerator + timeBase.Denominator/2) / timeBase.Denominator)
}

func abs64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
