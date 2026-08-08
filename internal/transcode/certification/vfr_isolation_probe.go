package certification

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
	transcodevfrisolation "github.com/nowen-video/nowen-video/internal/transcode/vfrisolation"
)

func probeVFRIsolationVariant(
	ctx context.Context,
	ffmpegPath,
	ffprobePath,
	inputPath,
	workDir string,
	spec transcodevfrisolation.VariantSpec,
	sourceTimeline transcodeoutputcadence.TimelineEvidence,
	commandArgs []string,
) (transcodevfrisolation.VariantEvidence, error) {
	timeline, _, err := probeVideoCadenceTimeline(
		ctx,
		ffprobePath,
		outputCadenceProbeInput{Path: inputPath},
		"isolation_"+spec.ID,
		sourceTimeline.WindowStartMicros,
		sourceTimeline.WindowEndMicros,
	)
	if err != nil {
		return transcodevfrisolation.VariantEvidence{}, fmt.Errorf("probe %s cadence: %w", spec.ID, err)
	}
	fingerprint, err := probeDecodedFrameFingerprint(ctx, ffmpegPath, inputPath)
	if err != nil {
		return transcodevfrisolation.VariantEvidence{}, fmt.Errorf("probe %s decoded frames: %w", spec.ID, err)
	}
	mapping := transcodeoutputcadence.NewFrameMapping(sourceTimeline.FrameCount, timeline.FrameCount)
	return transcodevfrisolation.VariantEvidence{
		Spec:                  spec,
		CommandHash:           hashNormalizedArgs(commandArgs, workDir),
		Timeline:              timeline,
		Mapping:               mapping,
		Fingerprint:           fingerprint,
		CadenceClassification: transcodevfrisolation.CadenceClassification(sourceTimeline, timeline, mapping),
		SequenceReference:     transcodevfrisolation.SequenceReferenceNone,
	}, nil
}

func probeDecodedFrameFingerprint(ctx context.Context, ffmpegPath, inputPath string) (transcodevfrisolation.FrameFingerprint, error) {
	args := []string{
		"-v", "error",
		"-i", inputPath,
		"-map", "0:v:0",
		"-fps_mode", "passthrough",
		"-f", "framemd5",
		"-hash", "sha256",
		"-",
	}
	command := exec.CommandContext(ctx, ffmpegPath, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return transcodevfrisolation.FrameFingerprint{}, fmt.Errorf("ffmpeg framemd5 failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	hashes := make([]string, 0, 320)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 6 {
			return transcodevfrisolation.FrameFingerprint{}, fmt.Errorf("unexpected framemd5 row %q", line)
		}
		value := strings.TrimSpace(fields[len(fields)-1])
		if len(value) != 64 {
			return transcodevfrisolation.FrameFingerprint{}, fmt.Errorf("unexpected decoded frame hash %q", value)
		}
		if _, err := hex.DecodeString(value); err != nil {
			return transcodevfrisolation.FrameFingerprint{}, fmt.Errorf("invalid decoded frame hash: %w", err)
		}
		hashes = append(hashes, value)
	}
	if err := scanner.Err(); err != nil {
		return transcodevfrisolation.FrameFingerprint{}, err
	}
	if len(hashes) == 0 {
		return transcodevfrisolation.FrameFingerprint{}, fmt.Errorf("framemd5 produced no video frames")
	}

	unique := make(map[string]struct{}, len(hashes))
	adjacentDuplicates := 0
	for index, value := range hashes {
		unique[value] = struct{}{}
		if index > 0 && value == hashes[index-1] {
			adjacentDuplicates++
		}
	}
	sequenceDigest := sha256.Sum256([]byte(strings.Join(hashes, "\n")))
	return transcodevfrisolation.FrameFingerprint{
		FrameCount:             len(hashes),
		UniqueFrameCount:       len(unique),
		AdjacentDuplicateCount: adjacentDuplicates,
		SequenceSHA256:         hex.EncodeToString(sequenceDigest[:]),
		FirstFrameSHA256:       hashes[0],
		LastFrameSHA256:        hashes[len(hashes)-1],
	}, nil
}

func hashNormalizedArgs(args []string, workDir string) string {
	normalized := make([]string, len(args))
	for index, value := range args {
		if workDir != "" {
			value = strings.ReplaceAll(value, workDir, "$WORK")
		}
		normalized[index] = value
	}
	digest := sha256.Sum256([]byte(strings.Join(normalized, "\x00")))
	return hex.EncodeToString(digest[:])
}
