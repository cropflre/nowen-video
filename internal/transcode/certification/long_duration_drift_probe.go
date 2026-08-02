package certification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodelongdrift "github.com/nowen-video/nowen-video/internal/transcode/longdrift"
)

type longDriftProbeDocument struct {
	Streams []longDriftProbeStream `json:"streams"`
	Packets []longDriftProbePacket `json:"packets"`
}

type longDriftProbeStream struct {
	TimeBase string `json:"time_base"`
}

type longDriftProbePacket struct {
	PTS      sourceOriginFlexibleNumber `json:"pts"`
	Duration sourceOriginFlexibleNumber `json:"duration"`
}

type longDriftPoint struct {
	PTSTicks      int64
	DurationTicks int64
}

func probeLongDriftStream(ctx context.Context, ffprobePath, manifestPath, selector, kind string) (transcodelongdrift.StreamEvidence, error) {
	return probeLongDriftStreamForPolicy(ctx, ffprobePath, manifestPath, selector, kind, transcodelongdrift.DefaultPolicy())
}

func probeLongDriftStreamForPolicy(
	ctx context.Context,
	ffprobePath,
	manifestPath,
	selector,
	kind string,
	policy transcodelongdrift.Policy,
) (transcodelongdrift.StreamEvidence, error) {
	if err := policy.Validate(); err != nil {
		return transcodelongdrift.StreamEvidence{}, err
	}
	command := exec.CommandContext(
		ctx,
		ffprobePath,
		"-v", "error",
		"-select_streams", selector,
		"-print_format", "json",
		"-show_streams",
		"-show_packets",
		"-show_entries", "stream=time_base:packet=pts,duration",
		manifestPath,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return transcodelongdrift.StreamEvidence{}, fmt.Errorf("ffprobe long-duration %s stream failed: %w: %s", kind, err, strings.TrimSpace(string(output)))
	}
	var document longDriftProbeDocument
	if err := json.NewDecoder(bytes.NewReader(output)).Decode(&document); err != nil {
		return transcodelongdrift.StreamEvidence{}, fmt.Errorf("decode long-duration %s probe: %w", kind, err)
	}
	if len(document.Streams) != 1 || strings.TrimSpace(document.Streams[0].TimeBase) == "" {
		return transcodelongdrift.StreamEvidence{}, fmt.Errorf("long-duration %s stream identity is incomplete", kind)
	}
	points := make([]longDriftPoint, 0, len(document.Packets))
	for _, packet := range document.Packets {
		pts, ok := packet.PTS.int64Value()
		if !ok {
			continue
		}
		duration, _ := packet.Duration.int64Value()
		points = append(points, longDriftPoint{PTSTicks: pts, DurationTicks: duration})
	}
	return buildLongDriftStreamEvidenceForPolicy(kind, document.Streams[0].TimeBase, points, policy)
}

func buildLongDriftStreamEvidence(kind, timeBase string, points []longDriftPoint) (transcodelongdrift.StreamEvidence, error) {
	return buildLongDriftStreamEvidenceForPolicy(kind, timeBase, points, transcodelongdrift.DefaultPolicy())
}

func buildLongDriftStreamEvidenceForPolicy(
	kind,
	timeBase string,
	points []longDriftPoint,
	policy transcodelongdrift.Policy,
) (transcodelongdrift.StreamEvidence, error) {
	if err := policy.Validate(); err != nil {
		return transcodelongdrift.StreamEvidence{}, err
	}
	if len(points) < 3 {
		return transcodelongdrift.StreamEvidence{}, fmt.Errorf("long-duration %s stream has %d packets", kind, len(points))
	}
	sort.SliceStable(points, func(i, j int) bool { return points[i].PTSTicks < points[j].PTSTicks })
	lastPositiveDuration := int64(0)
	presentation := make([]int64, 0, len(points))
	endMicros := int64(0)
	for index := range points {
		duration := points[index].DurationTicks
		if duration <= 0 && index+1 < len(points) {
			duration = points[index+1].PTSTicks - points[index].PTSTicks
		}
		if duration <= 0 {
			duration = lastPositiveDuration
		}
		if duration <= 0 {
			return transcodelongdrift.StreamEvidence{}, fmt.Errorf("long-duration %s packet duration is unavailable", kind)
		}
		lastPositiveDuration = duration
		ptsMicros, err := transcodeboundary.TicksToMicros(points[index].PTSTicks, timeBase)
		if err != nil {
			return transcodelongdrift.StreamEvidence{}, err
		}
		durationMicros, err := transcodeboundary.TicksToMicros(duration, timeBase)
		if err != nil {
			return transcodelongdrift.StreamEvidence{}, err
		}
		presentation = append(presentation, ptsMicros)
		if candidate := ptsMicros + durationMicros; candidate > endMicros {
			endMicros = candidate
		}
	}
	startMicros := presentation[0]
	normalizedPresentation := make([]int64, len(presentation))
	for index, value := range presentation {
		normalizedPresentation[index] = value - startMicros
	}
	durationMicros := endMicros - startMicros
	checkpoints := make([]transcodelongdrift.CheckpointEvidence, 0, len(transcodelongdrift.CheckpointTargetsForPolicy(policy)))
	for _, target := range transcodelongdrift.CheckpointTargetsForPolicy(policy) {
		observed := nearestPresentationMicros(normalizedPresentation, target)
		if target == policy.DurationMicros {
			observed = durationMicros
		}
		checkpoints = append(checkpoints, transcodelongdrift.CheckpointEvidence{
			TargetMicros:       target,
			PresentationMicros: observed,
			ErrorMicros:        observed - target,
		})
	}
	return transcodelongdrift.StreamEvidence{
		Kind:           kind,
		TimeBase:       timeBase,
		PacketCount:    len(points),
		StartMicros:    startMicros,
		EndMicros:      endMicros,
		DurationMicros: durationMicros,
		EndErrorMicros: durationMicros - policy.DurationMicros,
		Checkpoints:    checkpoints,
	}, nil
}

func nearestPresentationMicros(values []int64, target int64) int64 {
	index := sort.Search(len(values), func(index int) bool { return values[index] >= target })
	if index == 0 {
		return values[0]
	}
	if index == len(values) {
		return values[len(values)-1]
	}
	before := values[index-1]
	after := values[index]
	if target-before <= after-target {
		return before
	}
	return after
}
