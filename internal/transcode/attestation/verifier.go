package attestation

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w: %s", filepath.Base(name), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

type VerifyRequest struct {
	ManifestPath        string
	EncodingPlanVersion string
	EncodingPlanHash    string
	EncodingPlanJSON    string
	Scope               string
}

type Verifier struct {
	FFprobePath string
	Runner      CommandRunner
}

func (v Verifier) Verify(ctx context.Context, request VerifyRequest) (Attestation, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if request.ManifestPath == "" {
		return Attestation{}, fmt.Errorf("manifest path is required")
	}
	if request.Scope == "" {
		request.Scope = ScopeComplete
	}
	segments, err := readManifestSegments(request.ManifestPath)
	if err != nil {
		return Attestation{}, err
	}
	if len(segments) == 0 {
		return Attestation{}, fmt.Errorf("manifest contains no media segments")
	}
	first, err := v.probeSegment(ctx, filepath.Dir(request.ManifestPath), segments[0])
	if err != nil {
		return Attestation{}, fmt.Errorf("probe first segment: %w", err)
	}
	last := first
	if request.Scope == ScopeComplete && len(segments) > 1 {
		last, err = v.probeSegment(ctx, filepath.Dir(request.ManifestPath), segments[len(segments)-1])
		if err != nil {
			return Attestation{}, fmt.Errorf("probe last segment: %w", err)
		}
	}
	attestation := Attestation{
		SchemaVersion:       SchemaVersion,
		Scope:               request.Scope,
		EncodingPlanVersion: request.EncodingPlanVersion,
		EncodingPlanHash:    request.EncodingPlanHash,
		SegmentCount:        len(segments),
		First:               first,
		Last:                last,
	}
	if err := VerifyAgainstEncodingPlan(
		attestation,
		request.EncodingPlanVersion,
		request.EncodingPlanHash,
		request.EncodingPlanJSON,
	); err != nil {
		return Attestation{}, err
	}
	return attestation, nil
}

func (v Verifier) probeSegment(ctx context.Context, directory, name string) (SegmentEvidence, error) {
	ffprobePath := strings.TrimSpace(v.FFprobePath)
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	runner := v.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	segmentPath := filepath.Join(directory, name)
	output, err := runner.Run(
		ctx,
		ffprobePath,
		"-v", "error",
		"-print_format", "json",
		"-show_streams",
		"-show_packets",
		"-show_entries",
		"stream=index,codec_name,codec_type,profile,level,width,height,pix_fmt,color_primaries,color_transfer,color_space,channels,sample_rate,avg_frame_rate,r_frame_rate,time_base:packet=stream_index,pts,dts,duration,pts_time,dts_time,duration_time,flags",
		segmentPath,
	)
	if err != nil {
		return SegmentEvidence{}, err
	}
	var document probeDocument
	decoder := json.NewDecoder(bytes.NewReader(output))
	if err := decoder.Decode(&document); err != nil {
		return SegmentEvidence{}, fmt.Errorf("decode ffprobe output: %w", err)
	}
	return document.evidence(name)
}

func readManifestSegments(manifestPath string) ([]string, error) {
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("open hls manifest: %w", err)
	}
	defer file.Close()

	segments := make([]string, 0, 16)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parsed, err := url.Parse(line)
		if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("manifest contains unsupported segment URI %q", line)
		}
		name := filepath.ToSlash(parsed.Path)
		if name == "" || strings.Contains(name, "/") || name != filepath.Base(name) {
			return nil, fmt.Errorf("manifest segment URI is unsafe: %q", line)
		}
		if strings.ToLower(filepath.Ext(name)) != ".ts" {
			return nil, fmt.Errorf("manifest segment is not MPEG-TS: %q", line)
		}
		segments = append(segments, name)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read hls manifest: %w", err)
	}
	return segments, nil
}

type probeDocument struct {
	Streams []probeStream `json:"streams"`
	Packets []probePacket `json:"packets"`
}

type probeStream struct {
	Index          int    `json:"index"`
	CodecName      string `json:"codec_name"`
	CodecType      string `json:"codec_type"`
	Profile        string `json:"profile"`
	Level          int    `json:"level"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	PixelFormat    string `json:"pix_fmt"`
	ColorPrimaries string `json:"color_primaries"`
	ColorTransfer  string `json:"color_transfer"`
	ColorMatrix    string `json:"color_space"`
	Channels       int    `json:"channels"`
	SampleRate     string `json:"sample_rate"`
	AverageRate    string `json:"avg_frame_rate"`
	RealRate       string `json:"r_frame_rate"`
	TimeBase       string `json:"time_base"`
}

type probePacket struct {
	StreamIndex  int            `json:"stream_index"`
	PTS          flexibleNumber `json:"pts"`
	DTS          flexibleNumber `json:"dts"`
	Duration     flexibleNumber `json:"duration"`
	PTSTime      flexibleNumber `json:"pts_time"`
	DTSTime      flexibleNumber `json:"dts_time"`
	DurationTime flexibleNumber `json:"duration_time"`
}

type flexibleNumber string

func (n *flexibleNumber) UnmarshalJSON(data []byte) error {
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
	*n = flexibleNumber(value)
	return nil
}

func (n flexibleNumber) int64Value() (int64, bool) {
	if n == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(string(n), 10, 64)
	return value, err == nil
}

func (n flexibleNumber) float64Value() (float64, bool) {
	if n == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(string(n), 64)
	return value, err == nil && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (d probeDocument) evidence(name string) (SegmentEvidence, error) {
	var video probeStream
	var audio probeStream
	videoFound := false
	audioFound := false
	for _, stream := range d.Streams {
		switch stream.CodecType {
		case "video":
			if !videoFound {
				video = stream
				videoFound = true
			}
		case "audio":
			if !audioFound {
				audio = stream
				audioFound = true
			}
		}
	}
	if !videoFound || !audioFound {
		return SegmentEvidence{}, fmt.Errorf("segment must contain video and audio streams")
	}
	videoTimeline, err := packetRange(d.Packets, video.Index, video.TimeBase)
	if err != nil {
		return SegmentEvidence{}, fmt.Errorf("video packets: %w", err)
	}
	audioTimeline, err := packetRange(d.Packets, audio.Index, audio.TimeBase)
	if err != nil {
		return SegmentEvidence{}, fmt.Errorf("audio packets: %w", err)
	}
	return SegmentEvidence{
		Name:  name,
		Video: video.identity(),
		Audio: audio.identity(),
		Timeline: Timeline{
			Video: videoTimeline,
			Audio: audioTimeline,
		},
	}, nil
}

func (s probeStream) identity() StreamIdentity {
	sampleRate, _ := strconv.Atoi(s.SampleRate)
	rate := s.AverageRate
	if rate == "" || rate == "0/0" {
		rate = s.RealRate
	}
	return StreamIdentity{
		CodecName:      strings.ToLower(strings.TrimSpace(s.CodecName)),
		Profile:        s.Profile,
		Level:          s.Level,
		Width:          s.Width,
		Height:         s.Height,
		PixelFormat:    strings.ToLower(strings.TrimSpace(s.PixelFormat)),
		ColorPrimaries: strings.ToLower(strings.TrimSpace(s.ColorPrimaries)),
		ColorTransfer:  strings.ToLower(strings.TrimSpace(s.ColorTransfer)),
		ColorMatrix:    strings.ToLower(strings.TrimSpace(s.ColorMatrix)),
		Channels:       s.Channels,
		SampleRate:     sampleRate,
		FrameRateMilli: rationalMilli(rate),
		TimeBase:       strings.TrimSpace(s.TimeBase),
	}
}

func packetRange(packets []probePacket, streamIndex int, timeBase string) (PacketRange, error) {
	selected := make([]probePacket, 0, len(packets))
	for _, packet := range packets {
		if packet.StreamIndex == streamIndex {
			selected = append(selected, packet)
		}
	}
	if len(selected) == 0 {
		return PacketRange{}, fmt.Errorf("no packets found")
	}
	first := selected[0]
	last := selected[len(selected)-1]
	firstPTS, firstPTSOK := first.PTS.int64Value()
	if !firstPTSOK {
		firstPTS, firstPTSOK = first.DTS.int64Value()
	}
	firstDTS, firstDTSOK := first.DTS.int64Value()
	if !firstDTSOK {
		firstDTS = firstPTS
	}
	lastPTS, lastPTSOK := last.PTS.int64Value()
	if !lastPTSOK {
		lastPTS, lastPTSOK = last.DTS.int64Value()
	}
	lastDTS, lastDTSOK := last.DTS.int64Value()
	if !lastDTSOK {
		lastDTS = lastPTS
	}
	if !firstPTSOK || !lastPTSOK {
		return PacketRange{}, fmt.Errorf("packet timestamps are unavailable")
	}
	duration, durationOK := last.Duration.int64Value()
	if !durationOK || duration <= 0 {
		duration = 1
	}
	startMS, ok := milliseconds(first.PTSTime, firstPTS, timeBase)
	if !ok {
		return PacketRange{}, fmt.Errorf("first packet time is unavailable")
	}
	lastMS, ok := milliseconds(last.PTSTime, lastPTS, timeBase)
	if !ok {
		return PacketRange{}, fmt.Errorf("last packet time is unavailable")
	}
	durationMS, ok := durationMilliseconds(last.DurationTime, duration, timeBase)
	if !ok || durationMS <= 0 {
		durationMS = 1
	}
	return PacketRange{
		FirstPTS: firstPTS,
		FirstDTS: firstDTS,
		LastPTS:  lastPTS,
		LastDTS:  lastDTS,
		EndPTS:   lastPTS + duration,
		StartMS:  startMS,
		EndMS:    lastMS + durationMS,
	}, nil
}

func milliseconds(explicit flexibleNumber, ticks int64, timeBase string) (int64, bool) {
	if seconds, ok := explicit.float64Value(); ok {
		return int64(math.Round(seconds * 1000)), true
	}
	numerator, denominator, ok := rational(timeBase)
	if !ok {
		return 0, false
	}
	return int64(math.Round(float64(ticks) * numerator / denominator * 1000)), true
}

func durationMilliseconds(explicit flexibleNumber, ticks int64, timeBase string) (int64, bool) {
	return milliseconds(explicit, ticks, timeBase)
}

func rationalMilli(value string) int {
	numerator, denominator, ok := rational(value)
	if !ok || denominator == 0 {
		return 0
	}
	return int(math.Round(numerator / denominator * 1000))
}

func rational(value string) (float64, float64, bool) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return 0, 0, false
	}
	numerator, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, 0, false
	}
	denominator, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || denominator == 0 {
		return 0, 0, false
	}
	return numerator, denominator, true
}
