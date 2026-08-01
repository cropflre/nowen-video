package certification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
)

type sourceOriginProbeDocument struct {
	Streams []sourceOriginProbeStream `json:"streams"`
	Packets []sourceOriginProbePacket `json:"packets"`
}

type sourceOriginProbeStream struct {
	Index     int    `json:"index"`
	CodecType string `json:"codec_type"`
	TimeBase  string `json:"time_base"`
}

type sourceOriginProbePacket struct {
	StreamIndex int                        `json:"stream_index"`
	PTS         sourceOriginFlexibleNumber `json:"pts"`
	DTS         sourceOriginFlexibleNumber `json:"dts"`
	Duration    sourceOriginFlexibleNumber `json:"duration"`
}

type sourceOriginFlexibleNumber string

func (n *sourceOriginFlexibleNumber) UnmarshalJSON(data []byte) error {
	value := strings.TrimSpace(string(data))
	if value == "null" {
		*n = ""
		return nil
	}
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		var decoded string
		if err := json.Unmarshal(data, &decoded); err != nil {
			return err
		}
		value = decoded
	}
	if value == "N/A" {
		value = ""
	}
	*n = sourceOriginFlexibleNumber(value)
	return nil
}

func (n sourceOriginFlexibleNumber) int64Value() (int64, bool) {
	if n == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(string(n), 10, 64)
	return value, err == nil
}

func probeSourceOrigin(ctx context.Context, ffprobePath, sourcePath string) (transcodesourceorigin.StreamEvidence, transcodesourceorigin.StreamEvidence, error) {
	command := exec.CommandContext(
		ctx,
		ffprobePath,
		"-v", "error",
		"-print_format", "json",
		"-show_streams",
		"-show_packets",
		"-show_entries", "stream=index,codec_type,time_base:packet=stream_index,pts,dts,duration",
		sourcePath,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return transcodesourceorigin.StreamEvidence{}, transcodesourceorigin.StreamEvidence{}, fmt.Errorf("ffprobe source origin failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var document sourceOriginProbeDocument
	if err := json.NewDecoder(bytes.NewReader(output)).Decode(&document); err != nil {
		return transcodesourceorigin.StreamEvidence{}, transcodesourceorigin.StreamEvidence{}, fmt.Errorf("decode source origin probe: %w", err)
	}
	videoStream, ok := findSourceOriginStream(document.Streams, transcodesourceorigin.StreamVideo)
	if !ok {
		return transcodesourceorigin.StreamEvidence{}, transcodesourceorigin.StreamEvidence{}, fmt.Errorf("source origin probe has no video stream")
	}
	audioStream, ok := findSourceOriginStream(document.Streams, transcodesourceorigin.StreamAudio)
	if !ok {
		return transcodesourceorigin.StreamEvidence{}, transcodesourceorigin.StreamEvidence{}, fmt.Errorf("source origin probe has no audio stream")
	}
	video, err := buildSourceOriginStreamEvidence(transcodesourceorigin.StreamVideo, videoStream, document.Packets)
	if err != nil {
		return transcodesourceorigin.StreamEvidence{}, transcodesourceorigin.StreamEvidence{}, fmt.Errorf("source video evidence: %w", err)
	}
	audio, err := buildSourceOriginStreamEvidence(transcodesourceorigin.StreamAudio, audioStream, document.Packets)
	if err != nil {
		return transcodesourceorigin.StreamEvidence{}, transcodesourceorigin.StreamEvidence{}, fmt.Errorf("source audio evidence: %w", err)
	}
	return video, audio, nil
}

func findSourceOriginStream(streams []sourceOriginProbeStream, kind string) (sourceOriginProbeStream, bool) {
	for _, stream := range streams {
		if stream.CodecType == kind {
			return stream, true
		}
	}
	return sourceOriginProbeStream{}, false
}

func buildSourceOriginStreamEvidence(kind string, stream sourceOriginProbeStream, packets []sourceOriginProbePacket) (transcodesourceorigin.StreamEvidence, error) {
	selected := make([]sourceOriginProbePacket, 0, len(packets))
	for _, packet := range packets {
		if packet.StreamIndex == stream.Index {
			selected = append(selected, packet)
		}
	}
	if len(selected) < 3 {
		return transcodesourceorigin.StreamEvidence{}, fmt.Errorf("stream has %d packets, want at least 3", len(selected))
	}
	firstPTS, ok := selected[0].PTS.int64Value()
	if !ok {
		return transcodesourceorigin.StreamEvidence{}, fmt.Errorf("first PTS is unavailable")
	}
	firstDTS, ok := selected[0].DTS.int64Value()
	if !ok {
		firstDTS = firstPTS
	}
	durations := make([]int64, 0, len(selected)-1)
	for index := 0; index < len(selected)-1; index++ {
		currentPTS, currentOK := selected[index].PTS.int64Value()
		nextPTS, nextOK := selected[index+1].PTS.int64Value()
		duration, durationOK := selected[index].Duration.int64Value()
		if kind == transcodesourceorigin.StreamVideo && currentOK && nextOK && nextPTS > currentPTS {
			duration = nextPTS - currentPTS
			durationOK = true
		}
		if durationOK && duration > 0 {
			durations = append(durations, duration)
		}
	}
	if len(durations) == 0 {
		return transcodesourceorigin.StreamEvidence{}, fmt.Errorf("packet durations are unavailable")
	}
	minimum := durations[0]
	maximum := durations[0]
	distinct := make(map[int64]struct{}, 4)
	for _, duration := range durations {
		if duration < minimum {
			minimum = duration
		}
		if duration > maximum {
			maximum = duration
		}
		distinct[duration] = struct{}{}
	}
	firstPTSMicros, err := transcodeboundary.TicksToMicros(firstPTS, stream.TimeBase)
	if err != nil {
		return transcodesourceorigin.StreamEvidence{}, err
	}
	firstDTSMicros, err := transcodeboundary.TicksToMicros(firstDTS, stream.TimeBase)
	if err != nil {
		return transcodesourceorigin.StreamEvidence{}, err
	}
	minimumMicros, err := transcodeboundary.TicksToMicros(minimum, stream.TimeBase)
	if err != nil {
		return transcodesourceorigin.StreamEvidence{}, err
	}
	maximumMicros, err := transcodeboundary.TicksToMicros(maximum, stream.TimeBase)
	if err != nil {
		return transcodesourceorigin.StreamEvidence{}, err
	}
	spread := maximumMicros - minimumMicros
	return transcodesourceorigin.StreamEvidence{
		Kind:                    kind,
		TimeBase:                stream.TimeBase,
		PacketCount:             len(selected),
		FirstPTS:                firstPTS,
		FirstDTS:                firstDTS,
		FirstPTSMicros:          firstPTSMicros,
		FirstDTSMicros:          firstDTSMicros,
		MinPacketDurationTicks:  minimum,
		MaxPacketDurationTicks:  maximum,
		MinPacketDurationMicros: minimumMicros,
		MaxPacketDurationMicros: maximumMicros,
		DurationSpreadMicros:    spread,
		DistinctDurations:       len(distinct),
		VariableDuration:        spread >= transcodesourceorigin.VFRSpreadThresholdMicros,
	}, nil
}
