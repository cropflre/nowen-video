package certification

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
	transcodevfrisolation "github.com/nowen-video/nowen-video/internal/transcode/vfrisolation"
)

type vfrIsolationProducedVariant struct {
	Path string
	Args []string
}

func RunVFRIsolationMatrix(ctx context.Context, config Config) (VFRIsolationMatrixReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	spec, ok := LookupSourceOriginCase(vfrIsolationCaseID)
	if !ok {
		return VFRIsolationMatrixReport{}, fmt.Errorf("VFR isolation source case is not registered")
	}
	ffmpegPath, err := resolveExecutable(config.FFmpegPath, "ffmpeg")
	if err != nil {
		return VFRIsolationMatrixReport{}, err
	}
	ffprobePath, err := resolveExecutable(config.FFprobePath, "ffprobe")
	if err != nil {
		return VFRIsolationMatrixReport{}, err
	}
	workDir, cleanup, err := prepareWorkDir(config)
	if err != nil {
		return VFRIsolationMatrixReport{}, err
	}
	defer cleanup()

	baselineWorkDir := filepath.Join(workDir, "bound-output-cadence")
	baselineConfig := config
	baselineConfig.WorkDir = baselineWorkDir
	baselineConfig.KeepWorkDir = true
	baselineConfig.FixtureID = spec.FixtureID
	baseline, err := RunOutputCadenceCase(ctx, baselineConfig, spec.ID)
	if err != nil {
		return VFRIsolationMatrixReport{}, fmt.Errorf("build bound output cadence evidence: %w", err)
	}
	sourceTimeline := baseline.Evidence.SourceContinuationTimeline
	if sourceTimeline.Kind != transcodeoutputcadence.TimelineSourceContinuation {
		return VFRIsolationMatrixReport{}, fmt.Errorf("bound output cadence does not expose source continuation")
	}

	timestampPlan := transcodetimestamp.Default()
	sourceGraph := sourceOriginInputGraph(spec)
	produced := make(map[string]vfrIsolationProducedVariant, len(vfrIsolationVariantSpecs))
	variants := make([]transcodevfrisolation.VariantEvidence, 0, len(vfrIsolationVariantSpecs))

	baselineDir := filepath.Join(baselineWorkDir, "produced-output", "continuation")
	baselineArgs, err := sourceOriginHLSArgs(
		sourceGraph,
		baselineDir,
		timestampPlan,
		spec,
		spec.ExpectedBoundaryMicros,
		0,
	)
	if err != nil {
		return VFRIsolationMatrixReport{}, err
	}
	produced["production-hls-v1"] = vfrIsolationProducedVariant{
		Path: filepath.Join(baselineDir, "stream.m3u8"),
		Args: baselineArgs,
	}

	for _, variantSpec := range vfrIsolationVariantSpecs[1:] {
		var result vfrIsolationProducedVariant
		if variantSpec.CopyOnly {
			parent, exists := produced[variantSpec.ParentVariantID]
			if !exists {
				return VFRIsolationMatrixReport{}, fmt.Errorf("VFR isolation parent %q is unavailable", variantSpec.ParentVariantID)
			}
			result, err = produceVFRIsolationRemux(ctx, ffmpegPath, workDir, variantSpec, parent.Path)
		} else {
			result, err = produceVFRIsolationEncoded(ctx, ffmpegPath, workDir, sourceGraph, timestampPlan, spec, variantSpec)
		}
		if err != nil {
			return VFRIsolationMatrixReport{}, fmt.Errorf("produce VFR isolation variant %s: %w", variantSpec.ID, err)
		}
		produced[variantSpec.ID] = result
	}

	for _, variantSpec := range vfrIsolationVariantSpecs {
		result := produced[variantSpec.ID]
		evidence, err := probeVFRIsolationVariant(
			ctx,
			ffmpegPath,
			ffprobePath,
			result.Path,
			workDir,
			variantSpec,
			sourceTimeline,
			result.Args,
		)
		if err != nil {
			return VFRIsolationMatrixReport{}, err
		}
		variants = append(variants, evidence)
	}

	baselineSequence := variants[0].Fingerprint.SequenceSHA256
	byID := make(map[string]transcodevfrisolation.VariantEvidence, len(variants))
	for index := range variants {
		variant := &variants[index]
		switch {
		case variant.Spec.ID == "production-hls-v1":
			variant.SequenceReference = transcodevfrisolation.SequenceReferenceNone
		case variant.Spec.CopyOnly:
			parent, exists := byID[variant.Spec.ParentVariantID]
			if !exists {
				return VFRIsolationMatrixReport{}, fmt.Errorf("VFR isolation parent evidence %q is unavailable", variant.Spec.ParentVariantID)
			}
			variant.SequenceReference = transcodevfrisolation.SequenceReferenceParent
			variant.SequenceReferenceVariantID = parent.Spec.ID
			variant.SequenceMatchesReference = variant.Fingerprint.SequenceSHA256 == parent.Fingerprint.SequenceSHA256
		default:
			variant.SequenceReference = transcodevfrisolation.SequenceReferenceBaseline
			variant.SequenceReferenceVariantID = "production-hls-v1"
			variant.SequenceMatchesReference = variant.Fingerprint.SequenceSHA256 == baselineSequence
		}
		byID[variant.Spec.ID] = *variant
	}

	contract := transcodevfrisolation.Contract{
		SchemaVersion:                transcodevfrisolation.SchemaVersion,
		CaseID:                       spec.ID,
		FixtureID:                    spec.FixtureID,
		WindowStartMicros:            spec.ExpectedBoundaryMicros,
		WindowEndMicros:              int64(sourceOriginDurationSeconds) * 1_000_000,
		FFmpegVersion:                baseline.Evidence.FFmpegVersion,
		FFprobeVersion:               baseline.Evidence.FFprobeVersion,
		BaselineOutputCadenceVersion: baseline.ContractVersion,
		BaselineOutputCadenceHash:    baseline.ContractHash,
		SourceTimeline:               sourceTimeline,
		Variants:                     variants,
		DiscontinuityRequired:        true,
	}
	contractVersion, contractHash, _, err := transcodevfrisolation.Identity(contract)
	if err != nil {
		return VFRIsolationMatrixReport{}, err
	}
	report := VFRIsolationMatrixReport{
		SchemaVersion: VFRIsolationMatrixSchemaVersion,
		ContractVersion: contractVersion,
		ContractHash: contractHash,
		Evidence: contract,
	}
	if err := report.Validate(); err != nil {
		return VFRIsolationMatrixReport{}, err
	}
	return report, nil
}

func produceVFRIsolationEncoded(
	ctx context.Context,
	ffmpegPath,
	workDir,
	sourceGraph string,
	timestampPlan transcodetimestamp.Plan,
	caseSpec SourceOriginCaseSpec,
	variantSpec transcodevfrisolation.VariantSpec,
) (vfrIsolationProducedVariant, error) {
	directory := filepath.Join(workDir, "variants", variantSpec.ID)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return vfrIsolationProducedVariant{}, err
	}
	baseArgs, err := sourceOriginHLSArgs(
		sourceGraph,
		directory,
		timestampPlan,
		caseSpec,
		caseSpec.ExpectedBoundaryMicros,
		0,
	)
	if err != nil {
		return vfrIsolationProducedVariant{}, err
	}

	var args []string
	var outputPath string
	switch variantSpec.Container {
	case "hls-mpegts":
		args = replaceOutputOption(baseArgs, "-fps_mode", variantSpec.FPSMode)
		if variantSpec.EncoderTimeBase != "auto" {
			args = insertBeforeOutput(args, "-enc_time_base:v:0", variantSpec.EncoderTimeBase)
		}
		outputPath = filepath.Join(directory, "stream.m3u8")
	case "matroska":
		args, err = asMatroskaIsolationArgs(baseArgs, filepath.Join(directory, "stream.mkv"), variantSpec)
		if err != nil {
			return vfrIsolationProducedVariant{}, err
		}
		outputPath = filepath.Join(directory, "stream.mkv")
	default:
		return vfrIsolationProducedVariant{}, fmt.Errorf("unsupported encoded isolation container %q", variantSpec.Container)
	}
	if err := runCommand(ctx, ffmpegPath, args...); err != nil {
		return vfrIsolationProducedVariant{}, err
	}
	return vfrIsolationProducedVariant{Path: outputPath, Args: args}, nil
}

func asMatroskaIsolationArgs(
	baseArgs []string,
	outputPath string,
	variantSpec transcodevfrisolation.VariantSpec,
) ([]string, error) {
	muxIndex := findOptionPair(baseArgs, "-f", "hls")
	if muxIndex < 0 {
		return nil, fmt.Errorf("production command does not contain HLS muxer")
	}
	args := append([]string(nil), baseArgs[:muxIndex]...)
	if variantSpec.EncoderTimeBase != "auto" {
		args = append(args, "-enc_time_base:v:0", variantSpec.EncoderTimeBase)
	}
	args = append(args,
		"-avoid_negative_ts", "disabled",
		"-fps_mode", variantSpec.FPSMode,
		"-f", "matroska",
		outputPath,
	)
	return args, nil
}

func produceVFRIsolationRemux(
	ctx context.Context,
	ffmpegPath,
	workDir string,
	variantSpec transcodevfrisolation.VariantSpec,
	parentPath string,
) (vfrIsolationProducedVariant, error) {
	directory := filepath.Join(workDir, "variants", variantSpec.ID)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return vfrIsolationProducedVariant{}, err
	}
	args := []string{
		"-y", "-copyts", "-start_at_zero",
		"-i", parentPath,
		"-map", "0:v:0", "-map", "0:a:0",
		"-c", "copy",
		"-avoid_negative_ts", "disabled",
	}
	var outputPath string
	switch variantSpec.Container {
	case "mpegts":
		outputPath = filepath.Join(directory, "stream.ts")
		args = append(args, "-f", "mpegts", outputPath)
	case "hls-mpegts":
		outputPath = filepath.Join(directory, "stream.m3u8")
		args = append(args,
			"-f", "hls",
			"-hls_time", fmt.Sprint(fixtureSegmentSeconds),
			"-hls_list_size", "0",
			"-hls_segment_filename", filepath.Join(directory, "seg%04d.ts"),
			"-hls_flags", "independent_segments",
			outputPath,
		)
	default:
		return vfrIsolationProducedVariant{}, fmt.Errorf("unsupported remux isolation container %q", variantSpec.Container)
	}
	if err := runCommand(ctx, ffmpegPath, args...); err != nil {
		return vfrIsolationProducedVariant{}, err
	}
	return vfrIsolationProducedVariant{Path: outputPath, Args: args}, nil
}

func replaceOutputOption(args []string, option, value string) []string {
	result := append([]string(nil), args...)
	for index := len(result) - 2; index >= 0; index-- {
		if result[index] == option {
			result[index+1] = value
			return result
		}
	}
	return insertBeforeOutput(result, option, value)
}

func insertBeforeOutput(args []string, values ...string) []string {
	if len(args) == 0 {
		return append([]string(nil), values...)
	}
	result := make([]string, 0, len(args)+len(values))
	result = append(result, args[:len(args)-1]...)
	result = append(result, values...)
	result = append(result, args[len(args)-1])
	return result
}

func findOptionPair(args []string, option, value string) int {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == option && args[index+1] == value {
			return index
		}
	}
	return -1
}
