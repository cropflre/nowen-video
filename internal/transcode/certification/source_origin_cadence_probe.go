package certification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
)

type outputCadenceProbeInput struct {
	Path  string
	Lavfi bool
}

func probeOutputCadenceContract(
	ctx context.Context,
	ffprobePath,
	sourceGraph,
	startupManifest,
	continuationManifest string,
	spec SourceOriginCaseSpec,
	ffmpegVersion,
	ffprobeVersion,
	sourceOriginVersion,
	sourceOriginHash,
	timestampVersion,
	timestampHash,
	boundaryVersion,
	boundaryHash,
	avSyncVersion,
	avSyncHash string,
) (transcodeoutputcadence.Contract, error) {
	sourceTimeline, sourcePTS, err := probeVideoCadenceTimeline(
		ctx,
		ffprobePath,
		outputCadenceProbeInput{Path: sourceGraph, Lavfi: true},
		transcodeoutputcadence.TimelineSource,
		spec.SourceOffsetMicros,
		spec.SourceOffsetMicros+int64(sourceOriginDurationSeconds)*1_000_000,
	)
	if err != nil {
		return transcodeoutputcadence.Contract{}, fmt.Errorf("probe source output-cadence timeline: %w", err)
	}
	startupTimeline, _, err := probeVideoCadenceTimeline(
		ctx,
		ffprobePath,
		outputCadenceProbeInput{Path: startupManifest},
		transcodeoutputcadence.TimelineStartup,
		0,
		spec.ExpectedBoundaryMicros,
	)
	if err != nil {
		return transcodeoutputcadence.Contract{}, fmt.Errorf("probe startup output-cadence timeline: %w", err)
	}
	continuationTimeline, _, err := probeVideoCadenceTimeline(
		ctx,
		ffprobePath,
		outputCadenceProbeInput{Path: continuationManifest},
		transcodeoutputcadence.TimelineContinuation,
		spec.ExpectedBoundaryMicros,
		int64(sourceOriginDurationSeconds)*1_000_000,
	)
	if err != nil {
		return transcodeoutputcadence.Contract{}, fmt.Errorf("probe continuation output-cadence timeline: %w", err)
	}

	startupInputFrames := 0
	continuationInputFrames := 0
	for _, ptsMicros := range sourcePTS {
		relative := ptsMicros - spec.SourceOffsetMicros
		switch {
		case relative >= 0 && relative < spec.ExpectedBoundaryMicros:
			startupInputFrames++
		case relative >= spec.ExpectedBoundaryMicros && relative < int64(sourceOriginDurationSeconds)*1_000_000:
			continuationInputFrames++
		}
	}
	if startupInputFrames == 0 || continuationInputFrames == 0 {
		return transcodeoutputcadence.Contract{}, fmt.Errorf("source frame windows are incomplete: startup=%d continuation=%d", startupInputFrames, continuationInputFrames)
	}

	contract := transcodeoutputcadence.Contract{
		SchemaVersion:                   transcodeoutputcadence.SchemaVersion,
		CaseID:                         spec.ID,
		FixtureID:                      spec.FixtureID,
		SourceMode:                     spec.SourceMode,
		DeclaredFrameRateNumerator:     spec.DeclaredFrameRateNumerator,
		DeclaredFrameRateDenominator:   spec.DeclaredFrameRateDenominator,
		DeclaredFrameRateMilli:         spec.DeclaredFrameRateMilli(),
		ExpectedBoundaryMicros:         spec.ExpectedBoundaryMicros,
		ExpectedStartupVariable:        spec.SourceMode == transcodesourceorigin.ModeVFR,
		ExpectedContinuationVariable:   false,
		FFmpegVersion:                  ffmpegVersion,
		FFprobeVersion:                 ffprobeVersion,
		SourceOriginVersion:            sourceOriginVersion,
		SourceOriginHash:               sourceOriginHash,
		TimestampPlanVersion:           timestampVersion,
		TimestampPlanHash:              timestampHash,
		BoundaryEvidenceVersion:        boundaryVersion,
		BoundaryEvidenceHash:           boundaryHash,
		AVSyncEvidenceVersion:          avSyncVersion,
		AVSyncEvidenceHash:             avSyncHash,
		SourceTimeline:                 sourceTimeline,
		StartupTimeline:                startupTimeline,
		ContinuationTimeline:           continuationTimeline,
		StartupMapping:                 transcodeoutputcadence.NewFrameMapping(startupInputFrames, startupTimeline.FrameCount),
		ContinuationMapping:            transcodeoutputcadence.NewFrameMapping(continuationInputFrames, continuationTimeline.FrameCount),
		ContentDuplicateClassification: transcodeoutputcadence.ContentDuplicateNotMeasured,
		DiscontinuityRequired:          true,
	}
	contract.PreservationStatus = transcodeoutputcadence.PreservationFor(contract)
	if err := contract.Validate(); err != nil {
		return transcodeoutputcadence.Contract{}, err
	}
	return contract, nil
}

func probeVideoCadenceTimeline(
	ctx context.Context,
	ffprobePath string,
	input outputCadenceProbeInput,
	kind string,
	windowStartMicros,
	windowEndMicros int64,
) (transcodeoutputcadence.TimelineEvidence, []int64, error) {
	args := []string{"-v", "error"}
	if input.Lavfi {
		args = append(args, "-f", "lavfi")
	}
	args = append(args,
		"-i", input.Path,
		"-print_format", "json",
		"-show_streams",
		"-show_packets",
		"-show_entries", "stream=index,codec_type,time_base:packet=stream_index,pts,dts,duration",
	)
	command := exec.CommandContext(ctx, ffprobePath, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return transcodeoutputcadence.TimelineEvidence{}, nil, fmt.Errorf("ffprobe cadence failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var document sourceOriginProbeDocument
	if err := json.NewDecoder(bytes.NewReader(output)).Decode(&document); err != nil {
		return transcodeoutputcadence.TimelineEvidence{}, nil, fmt.Errorf("decode cadence probe: %w", err)
	}
	stream, ok := findSourceOriginStream(document.Streams, transcodesourceorigin.StreamVideo)
	if !ok {
		return transcodeoutputcadence.TimelineEvidence{}, nil, fmt.Errorf("cadence probe has no video stream")
	}
	selected := make([]sourceOriginProbePacket, 0, len(document.Packets))
	for _, packet := range document.Packets {
		if packet.StreamIndex == stream.Index {
			selected = append(selected, packet)
		}
	}
	if len(selected) < 2 {
		return transcodeoutputcadence.TimelineEvidence{}, nil, fmt.Errorf("cadence probe has %d video packets", len(selected))
	}

	ptsTicks := make([]int64, 0, len(selected))
	ptsMicros := make([]int64, 0, len(selected))
	for _, packet := range selected {
		pts, ok := packet.PTS.int64Value()
		if !ok {
			return transcodeoutputcadence.TimelineEvidence{}, nil, fmt.Errorf("video packet PTS is unavailable")
		}
		micros, err := ticksToMicrosCertification(pts, stream.TimeBase)
		if err != nil {
			return transcodeoutputcadence.TimelineEvidence{}, nil, err
		}
		ptsTicks = append(ptsTicks, pts)
		ptsMicros = append(ptsMicros, micros)
	}

	deltas := make([]int64, 0, len(ptsTicks)-1)
	duplicatePTS := 0
	nonMonotonicPTS := 0
	for index := 0; index < len(ptsTicks)-1; index++ {
		delta := ptsTicks[index+1] - ptsTicks[index]
		switch {
		case delta == 0:
			duplicatePTS++
		case delta < 0:
			nonMonotonicPTS++
		default:
			deltas = append(deltas, delta)
		}
	}
	if len(deltas) == 0 {
		return transcodeoutputcadence.TimelineEvidence{}, nil, fmt.Errorf("cadence probe has no positive video deltas")
	}
	minimum := deltas[0]
	maximum := deltas[0]
	distinct := make(map[int64]struct{}, 4)
	for _, delta := range deltas {
		if delta < minimum {
			minimum = delta
		}
		if delta > maximum {
			maximum = delta
		}
		distinct[delta] = struct{}{}
	}
	minimumMicros, err := ticksToMicrosCertification(minimum, stream.TimeBase)
	if err != nil {
		return transcodeoutputcadence.TimelineEvidence{}, nil, err
	}
	maximumMicros, err := ticksToMicrosCertification(maximum, stream.TimeBase)
	if err != nil {
		return transcodeoutputcadence.TimelineEvidence{}, nil, err
	}
	spread := maximumMicros - minimumMicros
	return transcodeoutputcadence.TimelineEvidence{
		Kind: kind,
		TimeBase: stream.TimeBase,
		WindowStartMicros: windowStartMicros,
		WindowEndMicros: windowEndMicros,
		FrameCount: len(ptsTicks),
		FirstPTS: ptsTicks[0],
		LastPTS: ptsTicks[len(ptsTicks)-1],
		FirstPTSMicros: ptsMicros[0],
		LastPTSMicros: ptsMicros[len(ptsMicros)-1],
		MinDeltaTicks: minimum,
		MaxDeltaTicks: maximum,
		MinDeltaMicros: minimumMicros,
		MaxDeltaMicros: maximumMicros,
		DurationSpreadMicros: spread,
		DistinctDeltas: len(distinct),
		VariableDuration: spread >= transcodesourceorigin.VFRSpreadThresholdMicros,
		DuplicatePTSCount: duplicatePTS,
		NonMonotonicPTSCount: nonMonotonicPTS,
	}, ptsMicros, nil
}
