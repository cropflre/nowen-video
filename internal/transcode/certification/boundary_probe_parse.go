package certification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
)

type boundaryProbeSegment struct {
	Name  string
	Video boundaryProbeStreamEvidence
	Audio boundaryProbeStreamEvidence
}

type boundaryProbeStreamEvidence struct {
	SegmentName    string
	TimeBase       string
	SampleRate     int
	FrameRateMilli int
	Packets        []boundaryProbePacketEvidence
}

type boundaryProbePacketEvidence struct {
	PTS      int64
	DTS      int64
	Duration int64
	KeyFrame bool
	SideData []transcodeboundary.PacketSideData
}

type boundaryProbeDocument struct {
	Streams []boundaryProbeStream `json:"streams"`
	Packets []boundaryProbePacket `json:"packets"`
}

type boundaryProbeStream struct {
	Index       int    `json:"index"`
	CodecType   string `json:"codec_type"`
	SampleRate  string `json:"sample_rate"`
	AverageRate string `json:"avg_frame_rate"`
	RealRate    string `json:"r_frame_rate"`
	TimeBase    string `json:"time_base"`
}

type boundaryProbePacket struct {
	StreamIndex int                       `json:"stream_index"`
	PTS         boundaryFlexibleNumber    `json:"pts"`
	DTS         boundaryFlexibleNumber    `json:"dts"`
	Duration    boundaryFlexibleNumber    `json:"duration"`
	Flags       string                    `json:"flags"`
	SideData    []boundaryProbePacketSide `json:"side_data_list"`
}

type boundaryProbePacketSide struct {
	Type           string                 `json:"side_data_type"`
	SkipSamples    boundaryFlexibleNumber `json:"skip_samples"`
	DiscardPadding boundaryFlexibleNumber `json:"discard_padding"`
}

type boundaryFlexibleNumber string

func (n *boundaryFlexibleNumber) UnmarshalJSON(data []byte) error {
	value := strings.TrimSpace(string(data))
	if value == "null" || value == `"N/A"` {
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
	*n = boundaryFlexibleNumber(value)
	return nil
}

func (n boundaryFlexibleNumber) int64Value() (int64, bool) {
	if n == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(string(n), 10, 64)
	return value, err == nil
}

func probeBoundarySegment(ctx context.Context, ffprobePath, segmentPath, segmentName string) (boundaryProbeSegment, error) {
	output, err := (transcodeattestation.ExecRunner{}).Run(
		ctx,
		ffprobePath,
		"-v", "error",
		"-print_format", "json",
		"-show_streams",
		"-show_packets",
		"-show_entries",
		"stream=index,codec_type,sample_rate,avg_frame_rate,r_frame_rate,time_base:packet=stream_index,pts,dts,duration,flags:packet_side_data=side_data_type,skip_samples,discard_padding",
		segmentPath,
	)
	if err != nil {
		return boundaryProbeSegment{}, err
	}
	var document boundaryProbeDocument
	if err := json.NewDecoder(bytes.NewReader(output)).Decode(&document); err != nil {
		return boundaryProbeSegment{}, fmt.Errorf("decode boundary ffprobe output: %w", err)
	}
	return document.evidence(segmentName)
}

func (d boundaryProbeDocument) evidence(name string) (boundaryProbeSegment, error) {
	var videoStream boundaryProbeStream
	var audioStream boundaryProbeStream
	videoFound := false
	audioFound := false
	for _, stream := range d.Streams {
		switch stream.CodecType {
		case transcodeboundary.StreamVideo:
			if !videoFound {
				videoStream = stream
				videoFound = true
			}
		case transcodeboundary.StreamAudio:
			if !audioFound {
				audioStream = stream
				audioFound = true
			}
		}
	}
	if !videoFound || !audioFound {
		return boundaryProbeSegment{}, fmt.Errorf("boundary segment must contain video and audio")
	}
	video, err := d.streamEvidence(videoStream)
	if err != nil {
		return boundaryProbeSegment{}, fmt.Errorf("video packets: %w", err)
	}
	audio, err := d.streamEvidence(audioStream)
	if err != nil {
		return boundaryProbeSegment{}, fmt.Errorf("audio packets: %w", err)
	}
	video.SegmentName = name
	audio.SegmentName = name
	return boundaryProbeSegment{Name: name, Video: video, Audio: audio}, nil
}

func (d boundaryProbeDocument) streamEvidence(stream boundaryProbeStream) (boundaryProbeStreamEvidence, error) {
	selected := make([]boundaryProbePacket, 0, len(d.Packets))
	for _, packet := range d.Packets {
		if packet.StreamIndex == stream.Index {
			selected = append(selected, packet)
		}
	}
	if len(selected) == 0 {
		return boundaryProbeStreamEvidence{}, fmt.Errorf("no packets found")
	}

	packets := make([]boundaryProbePacketEvidence, 0, len(selected))
	for index, packet := range selected {
		pts, ptsOK := packet.PTS.int64Value()
		dts, dtsOK := packet.DTS.int64Value()
		if !ptsOK || !dtsOK {
			return boundaryProbeStreamEvidence{}, fmt.Errorf("packet timestamps are unavailable")
		}
		duration, durationOK := packet.Duration.int64Value()
		if !durationOK || duration <= 0 {
			var err error
			duration, err = inferTerminalBoundaryPacketDuration(selected, index)
			if err != nil {
				return boundaryProbeStreamEvidence{}, err
			}
		}
		sideData := make([]transcodeboundary.PacketSideData, 0, len(packet.SideData))
		for _, raw := range packet.SideData {
			typeName := strings.TrimSpace(raw.Type)
			skip, _ := raw.SkipSamples.int64Value()
			discard, _ := raw.DiscardPadding.int64Value()
			if typeName == "" && skip == 0 && discard == 0 {
				continue
			}
			if typeName == "" {
				typeName = "unknown"
			}
			sideData = append(sideData, transcodeboundary.PacketSideData{Type: typeName, SkipSamples: skip, DiscardPadding: discard})
		}
		packets = append(packets, boundaryProbePacketEvidence{
			PTS:      pts,
			DTS:      dts,
			Duration: duration,
			KeyFrame: strings.Contains(packet.Flags, "K"),
			SideData: sideData,
		})
	}

	sampleRate, _ := strconv.Atoi(stream.SampleRate)
	rate := stream.AverageRate
	if rate == "" || rate == "0/0" {
		rate = stream.RealRate
	}
	return boundaryProbeStreamEvidence{
		TimeBase:       strings.TrimSpace(stream.TimeBase),
		SampleRate:     sampleRate,
		FrameRateMilli: boundaryRationalMilli(rate),
		Packets:        packets,
	}, nil
}

// inferTerminalBoundaryPacketDuration handles a narrow MPEG-TS demuxer behavior:
// FFprobe can omit duration for the final packet while still reporting complete,
// strictly ordered DTS values. Only that terminal packet is eligible for
// inference, and its duration is derived from the immediately preceding DTS
// delta. Missing duration anywhere else remains a hard certification failure.
func inferTerminalBoundaryPacketDuration(packets []boundaryProbePacket, index int) (int64, error) {
	if index != len(packets)-1 {
		return 0, fmt.Errorf("packet duration is unavailable before stream tail")
	}
	if index == 0 {
		return 0, fmt.Errorf("terminal packet duration cannot be inferred without a preceding packet")
	}
	previousDTS, previousOK := packets[index-1].DTS.int64Value()
	currentDTS, currentOK := packets[index].DTS.int64Value()
	if !previousOK || !currentOK || currentDTS <= previousDTS {
		return 0, fmt.Errorf("terminal packet duration cannot be inferred from DTS")
	}
	return currentDTS - previousDTS, nil
}
