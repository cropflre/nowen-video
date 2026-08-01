package certification

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
)

func buildBoundaryStream(kind string, startup, continuation boundaryProbeStreamEvidence) (transcodeboundary.StreamEvidence, error) {
	if startup.TimeBase == "" || startup.TimeBase != continuation.TimeBase {
		return transcodeboundary.StreamEvidence{}, fmt.Errorf("stream time base mismatch")
	}
	if kind == transcodeboundary.StreamVideo && (startup.FrameRateMilli <= 0 || startup.FrameRateMilli != continuation.FrameRateMilli) {
		return transcodeboundary.StreamEvidence{}, fmt.Errorf("video frame rate mismatch")
	}
	if kind == transcodeboundary.StreamAudio && (startup.SampleRate <= 0 || startup.SampleRate != continuation.SampleRate) {
		return transcodeboundary.StreamEvidence{}, fmt.Errorf("audio sample rate mismatch")
	}
	startupWindow, err := buildSegmentWindow(startup.SegmentName, transcodeboundary.WindowTail, startup)
	if err != nil {
		return transcodeboundary.StreamEvidence{}, err
	}
	continuationWindow, err := buildSegmentWindow(continuation.SegmentName, transcodeboundary.WindowHead, continuation)
	if err != nil {
		return transcodeboundary.StreamEvidence{}, err
	}
	nominalTicks := medianPacketDuration(startup.Packets, continuation.Packets)
	if nominalTicks <= 0 {
		return transcodeboundary.StreamEvidence{}, fmt.Errorf("nominal packet duration is unavailable")
	}
	nominalMicros, err := transcodeboundary.TicksToMicros(nominalTicks, startup.TimeBase)
	if err != nil {
		return transcodeboundary.StreamEvidence{}, err
	}
	presentationTicks := continuationWindow.EarliestPTS - startupWindow.LatestEndPTS
	decodeTicks := continuationWindow.FirstDTS - startupWindow.LastEndDTS
	presentationMicros, err := transcodeboundary.TicksToMicros(presentationTicks, startup.TimeBase)
	if err != nil {
		return transcodeboundary.StreamEvidence{}, err
	}
	decodeMicros, err := transcodeboundary.TicksToMicros(decodeTicks, startup.TimeBase)
	if err != nil {
		return transcodeboundary.StreamEvidence{}, err
	}
	tolerance := transcodeboundary.ToleranceMicros(nominalMicros)
	evidence := transcodeboundary.StreamEvidence{
		Kind:                        kind,
		TimeBase:                    startup.TimeBase,
		SampleRate:                  startup.SampleRate,
		FrameRateMilli:              startup.FrameRateMilli,
		Startup:                     startupWindow,
		Continuation:                continuationWindow,
		PresentationDeltaTicks:      presentationTicks,
		PresentationDeltaMicros:     presentationMicros,
		DecodeDeltaTicks:            decodeTicks,
		DecodeDeltaMicros:           decodeMicros,
		NominalPacketDurationTicks:  nominalTicks,
		NominalPacketDurationMicros: nominalMicros,
		ToleranceMicros:             tolerance,
		BoundaryUnitsMilli:          int64(math.Round(float64(presentationTicks) * 1000 / float64(nominalTicks))),
		Status:                      transcodeboundary.Classify(presentationMicros, nominalMicros, tolerance),
	}
	if kind == transcodeboundary.StreamVideo {
		evidence.SampleRate = 0
	} else {
		evidence.FrameRateMilli = 0
		nominalSamples, err := transcodeboundary.TicksToSamples(nominalTicks, evidence.TimeBase, evidence.SampleRate)
		if err != nil {
			return transcodeboundary.StreamEvidence{}, err
		}
		boundarySamples, err := transcodeboundary.TicksToSamples(presentationTicks, evidence.TimeBase, evidence.SampleRate)
		if err != nil {
			return transcodeboundary.StreamEvidence{}, err
		}
		startupSkip, startupDiscard, startupObserved := boundarySideDataTotals(startupWindow.Packets)
		continuationSkip, continuationDiscard, continuationObserved := boundarySideDataTotals(continuationWindow.Packets)
		evidence.AudioDelay = &transcodeboundary.AudioDelayEvidence{
			NominalPacketSamples:       nominalSamples,
			BoundaryDeltaSamples:       boundarySamples,
			StartupSkipSamples:         startupSkip,
			ContinuationSkipSamples:    continuationSkip,
			StartupDiscardPadding:      startupDiscard,
			ContinuationDiscardPadding: continuationDiscard,
			SideDataObserved:           startupObserved || continuationObserved,
		}
	}
	return evidence, nil
}

func buildSegmentWindow(segmentName, position string, stream boundaryProbeStreamEvidence) (transcodeboundary.SegmentWindow, error) {
	if len(stream.Packets) == 0 {
		return transcodeboundary.SegmentWindow{}, fmt.Errorf("packet window is empty")
	}
	earliestPTS := stream.Packets[0].PTS
	latestEndPTS := stream.Packets[0].PTS + stream.Packets[0].Duration
	firstDTS := stream.Packets[0].DTS
	lastEndDTS := stream.Packets[0].DTS + stream.Packets[0].Duration
	for _, packet := range stream.Packets[1:] {
		if packet.PTS < earliestPTS {
			earliestPTS = packet.PTS
		}
		if end := packet.PTS + packet.Duration; end > latestEndPTS {
			latestEndPTS = end
		}
		if packet.DTS < firstDTS {
			firstDTS = packet.DTS
		}
		if end := packet.DTS + packet.Duration; end > lastEndDTS {
			lastEndDTS = end
		}
	}
	start := 0
	end := len(stream.Packets)
	if position == transcodeboundary.WindowTail && end > transcodeboundary.MaxWindowPackets {
		start = end - transcodeboundary.MaxWindowPackets
	}
	if position == transcodeboundary.WindowHead && end > transcodeboundary.MaxWindowPackets {
		end = transcodeboundary.MaxWindowPackets
	}
	packets := make([]transcodeboundary.PacketEvidence, 0, end-start)
	for ordinal := start; ordinal < end; ordinal++ {
		packet := stream.Packets[ordinal]
		ptsMicros, err := transcodeboundary.TicksToMicros(packet.PTS, stream.TimeBase)
		if err != nil {
			return transcodeboundary.SegmentWindow{}, err
		}
		dtsMicros, err := transcodeboundary.TicksToMicros(packet.DTS, stream.TimeBase)
		if err != nil {
			return transcodeboundary.SegmentWindow{}, err
		}
		durationMicros, err := transcodeboundary.TicksToMicros(packet.Duration, stream.TimeBase)
		if err != nil {
			return transcodeboundary.SegmentWindow{}, err
		}
		packets = append(packets, transcodeboundary.PacketEvidence{
			Ordinal:        ordinal,
			PTS:            packet.PTS,
			DTS:            packet.DTS,
			Duration:       packet.Duration,
			PTSMicros:      ptsMicros,
			DTSMicros:      dtsMicros,
			DurationMicros: durationMicros,
			KeyFrame:       packet.KeyFrame,
			SideData:       packet.SideData,
		})
	}
	return transcodeboundary.SegmentWindow{
		SegmentName:  segmentName,
		PacketCount:  len(stream.Packets),
		Position:     position,
		Packets:      packets,
		EarliestPTS:  earliestPTS,
		LatestEndPTS: latestEndPTS,
		FirstDTS:     firstDTS,
		LastEndDTS:   lastEndDTS,
	}, nil
}

func medianPacketDuration(groups ...[]boundaryProbePacketEvidence) int64 {
	values := make([]int64, 0)
	for _, group := range groups {
		for _, packet := range group {
			if packet.Duration > 0 {
				values = append(values, packet.Duration)
			}
		}
	}
	if len(values) == 0 {
		return 0
	}
	sort.Slice(values, func(left, right int) bool { return values[left] < values[right] })
	return values[len(values)/2]
}

func boundarySideDataTotals(packets []transcodeboundary.PacketEvidence) (skipSamples, discardPadding int64, observed bool) {
	for _, packet := range packets {
		for _, sideData := range packet.SideData {
			observed = true
			skipSamples += sideData.SkipSamples
			discardPadding += sideData.DiscardPadding
		}
	}
	return skipSamples, discardPadding, observed
}

func boundaryRationalMilli(value string) int {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) != 2 {
		return 0
	}
	numerator, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	denominator, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || denominator == 0 {
		return 0
	}
	return int(math.Round(numerator / denominator * 1000))
}
