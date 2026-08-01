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
		SchemaVersion:                         transcodeoutputcadence.SchemaVersion,
		CaseID:                               spec.ID,
		FixtureID:                            spec.FixtureID,
		SourceMode:                           spec.SourceMode,
		DeclaredFrameRateNumerator:           spec.DeclaredFrameRateNumerator,
		DeclaredFrameRateDenominator:         spec.DeclaredFrameRateDenominator,
		DeclaredFrameRateMilli:               spec.DeclaredFrameRateMilli(),
		ExpectedBoundaryMicros:               spec.ExpectedBoundaryMicros,
		ExpectedStartupMaterialVariable:      spec.SourceMode == transcodesourceorigin.ModeVFR,
		ExpectedContinuationMaterialVariable: false,
		FFmpegVersion:                        ffmpegVersion,
		FFprobeVersion:                       ffprobeVersion,
		SourceOriginVersion:                  sourceOriginVersion,
		SourceOriginHash:                     sourceOriginHash,
		TimestampPlanVersion:                 timestampVersion,
		TimestampPlanHash:                    timestampHash,
		BoundaryEvidenceVersion:              boundaryVersion,
		BoundaryEvidenceHash:                 boundaryHash,
		AVSyncEvidenceVersion:                avSyncVersion,
		AVSyncEvidenceHash:                   avSyncHash,
		SourceTimeline:                       sourceTimeline,
		StartupTimeline:                      startupTimeline,
		ContinuationTimeline:                 continuationTimeline,
		StartupMapping:                       transcodeoutputcadence.NewFrameMapping(startupInputFrames, startupTimeline.FrameCount),
		ContinuationMapping:                  transcodeoutputcadence.NewFrameMapping(continuationInputFrames, continuationTimeline.FrameCount),
		ContentDuplicateClassification:       transcodeoutputcadence.ContentDuplicateNotMeasured,
		DiscontinuityRequired:                true,
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
	ptsTicks := make([]int64, 0, len(document.Packets))
	ptsMicros := make([]int64, 0, len(document.Packets))
	for _, packet := range document.Packets {
		if packet.StreamIndex != stream.Index {
			continue
		}
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
	evidence, err := transcodeoutputcadence.NewTimelineEvidence(
		kind,
		stream.TimeBase,
		windowStartMicros,
		windowEndMicros,
		ptsTicks,
	)
	if err != nil {
		return transcodeoutputcadence.TimelineEvidence{}, nil, err
	}
	return evidence, ptsMicros, nil
}
