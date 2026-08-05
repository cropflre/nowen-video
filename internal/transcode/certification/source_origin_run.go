package certification

import (
	"context"
	"fmt"
	"math"
	"path/filepath"

	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
	transcodeavsync "github.com/nowen-video/nowen-video/internal/transcode/avsync"
	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodeencoding "github.com/nowen-video/nowen-video/internal/transcode/encodingplan"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func RunSourceOriginMatrix(ctx context.Context, config Config) (SourceOriginMatrixReport, error) {
	reports := make([]SourceOriginCaseReport, 0, len(sourceOriginCaseSpecs))
	for _, spec := range sourceOriginCaseSpecs {
		caseConfig := config
		caseConfig.FixtureID = spec.FixtureID
		if config.WorkDir != "" {
			caseConfig.WorkDir = filepath.Join(config.WorkDir, "source-origin", spec.ID)
		}
		report, err := RunSourceOriginCase(ctx, caseConfig, spec.ID)
		if err != nil {
			return SourceOriginMatrixReport{}, fmt.Errorf("run source origin case %s: %w", spec.ID, err)
		}
		reports = append(reports, report)
	}
	return BuildSourceOriginMatrixReport(reports)
}

func RunSourceOriginCase(ctx context.Context, config Config, caseID string) (SourceOriginCaseReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	spec, ok := LookupSourceOriginCase(caseID)
	if !ok {
		return SourceOriginCaseReport{}, fmt.Errorf("unknown source origin case %q", caseID)
	}
	if err := spec.Validate(); err != nil {
		return SourceOriginCaseReport{}, err
	}
	ffmpegPath, err := resolveExecutable(config.FFmpegPath, "ffmpeg")
	if err != nil {
		return SourceOriginCaseReport{}, err
	}
	ffprobePath, err := resolveExecutable(config.FFprobePath, "ffprobe")
	if err != nil {
		return SourceOriginCaseReport{}, err
	}
	workDir, cleanup, err := prepareWorkDir(config)
	if err != nil {
		return SourceOriginCaseReport{}, err
	}
	defer cleanup()

	ffmpegVersion, err := commandVersion(ctx, ffmpegPath)
	if err != nil {
		return SourceOriginCaseReport{}, err
	}
	ffprobeVersion, err := commandVersion(ctx, ffprobePath)
	if err != nil {
		return SourceOriginCaseReport{}, err
	}
	encodingPlan := sourceOriginEncodingPlan(spec)
	encodingVersion, encodingHash, encodingJSON, err := transcodeencoding.Identity(encodingPlan)
	if err != nil {
		return SourceOriginCaseReport{}, fmt.Errorf("source origin encoding plan identity: %w", err)
	}
	timestampPlan := transcodetimestamp.Default()
	timestampVersion, timestampHash, _, err := transcodetimestamp.Identity(timestampPlan)
	if err != nil {
		return SourceOriginCaseReport{}, fmt.Errorf("source origin timestamp plan identity: %w", err)
	}

	sourceGraph := sourceOriginInputGraph(spec)
	sourceVideo, sourceAudio, err := probeSourceOrigin(ctx, ffprobePath, sourceGraph)
	if err != nil {
		return SourceOriginCaseReport{}, err
	}
	startupManifest, err := produceSourceOriginHLS(ctx, ffmpegPath, workDir, "startup", sourceGraph, timestampPlan, spec, 0, spec.ExpectedBoundaryMicros)
	if err != nil {
		return SourceOriginCaseReport{}, err
	}
	continuationManifest, err := produceSourceOriginHLS(ctx, ffmpegPath, workDir, "continuation", sourceGraph, timestampPlan, spec, spec.ExpectedBoundaryMicros, 0)
	if err != nil {
		return SourceOriginCaseReport{}, err
	}

	verifier := transcodeattestation.Verifier{FFprobePath: ffprobePath}
	startup, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        startupManifest,
		EncodingPlanVersion: encodingVersion,
		EncodingPlanHash:    encodingHash,
		EncodingPlanJSON:    encodingJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return SourceOriginCaseReport{}, fmt.Errorf("verify source origin startup: %w", err)
	}
	continuation, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        continuationManifest,
		EncodingPlanVersion: encodingVersion,
		EncodingPlanHash:    encodingHash,
		EncodingPlanJSON:    encodingJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return SourceOriginCaseReport{}, fmt.Errorf("verify source origin continuation: %w", err)
	}
	boundaryMS := int64(math.Round(float64(spec.ExpectedBoundaryMicros) / 1000))
	if err := timestampPlan.VerifyObservedStart(0, startup.First.Timeline.Video.StartMS, startup.First.Timeline.Audio.StartMS); err != nil {
		return SourceOriginCaseReport{}, fmt.Errorf("source origin startup normalization: %w", err)
	}
	if err := timestampPlan.VerifyObservedStart(boundaryMS, continuation.First.Timeline.Video.StartMS, continuation.First.Timeline.Audio.StartMS); err != nil {
		return SourceOriginCaseReport{}, fmt.Errorf("source origin continuation normalization: %w", err)
	}
	startupVersion, startupHash, _, err := transcodeattestation.Identity(startup)
	if err != nil {
		return SourceOriginCaseReport{}, err
	}
	continuationVersion, continuationHash, _, err := transcodeattestation.Identity(continuation)
	if err != nil {
		return SourceOriginCaseReport{}, err
	}

	boundaryCase := BoundaryCaseSpec{
		ID:                     spec.ID,
		Description:            spec.Description,
		FixtureID:              spec.FixtureID,
		ExpectedBoundaryMicros: spec.ExpectedBoundaryMicros,
	}
	boundary, err := probeBoundaryContract(ctx, ffprobePath, boundaryContractRequest{
		Case:                           boundaryCase,
		Fixture:                        FixtureSpec{ID: spec.FixtureID},
		StartupManifest:                startupManifest,
		ContinuationManifest:           continuationManifest,
		FFmpegVersion:                  ffmpegVersion,
		FFprobeVersion:                 ffprobeVersion,
		EncodingPlanVersion:            encodingVersion,
		EncodingPlanHash:               encodingHash,
		TimestampPlanVersion:           timestampVersion,
		TimestampPlanHash:              timestampHash,
		StartupAttestationVersion:      startupVersion,
		StartupAttestationHash:         startupHash,
		ContinuationAttestationVersion: continuationVersion,
		ContinuationAttestationHash:    continuationHash,
	})
	if err != nil {
		return SourceOriginCaseReport{}, err
	}
	boundaryVersion, boundaryHash, _, err := transcodeboundary.Identity(boundary)
	if err != nil {
		return SourceOriginCaseReport{}, err
	}
	avSync, err := transcodeavsync.FromBoundary(boundary)
	if err != nil {
		return SourceOriginCaseReport{}, err
	}
	avSyncVersion, avSyncHash, _, err := transcodeavsync.Identity(avSync)
	if err != nil {
		return SourceOriginCaseReport{}, err
	}

	evidence := transcodesourceorigin.Contract{
		SchemaVersion:                 transcodesourceorigin.SchemaVersion,
		CaseID:                        spec.ID,
		FixtureID:                     spec.FixtureID,
		SourceMode:                    spec.SourceMode,
		DeclaredFrameRateNumerator:    spec.DeclaredFrameRateNumerator,
		DeclaredFrameRateDenominator:  spec.DeclaredFrameRateDenominator,
		DeclaredFrameRateMilli:        spec.DeclaredFrameRateMilli(),
		SourceOffsetMicros:            spec.SourceOffsetMicros,
		OriginClass:                   sourceOriginClass(spec.SourceOffsetMicros),
		OriginToleranceMicros:         transcodesourceorigin.MaxOriginErrorMicros,
		ExpectedBoundaryMicros:        spec.ExpectedBoundaryMicros,
		FFmpegVersion:                 ffmpegVersion,
		FFprobeVersion:                ffprobeVersion,
		TimestampPlanVersion:          timestampVersion,
		TimestampPlanHash:             timestampHash,
		BoundaryEvidenceVersion:       boundaryVersion,
		BoundaryEvidenceHash:          boundaryHash,
		AVSyncEvidenceVersion:         avSyncVersion,
		AVSyncEvidenceHash:            avSyncHash,
		SourceVideo:                   sourceVideo,
		SourceAudio:                   sourceAudio,
		NormalizedStartupVideoStartMS: startup.First.Timeline.Video.StartMS,
		NormalizedStartupAudioStartMS: startup.First.Timeline.Audio.StartMS,
		NormalizedContinuationVideoMS: continuation.First.Timeline.Video.StartMS,
		NormalizedContinuationAudioMS: continuation.First.Timeline.Audio.StartMS,
		DiscontinuityRequired:         true,
	}
	contractVersion, contractHash, _, err := transcodesourceorigin.Identity(evidence)
	if err != nil {
		return SourceOriginCaseReport{}, err
	}
	report := SourceOriginCaseReport{
		Case:            spec,
		ContractVersion: contractVersion,
		ContractHash:    contractHash,
		Evidence:        evidence,
		BoundaryVersion: boundaryVersion,
		BoundaryHash:    boundaryHash,
		Boundary:        boundary,
		AVSyncVersion:   avSyncVersion,
		AVSyncHash:      avSyncHash,
		AVSync:          avSync,
	}
	if err := report.Validate(); err != nil {
		return SourceOriginCaseReport{}, err
	}
	return report, nil
}

func sourceOriginEncodingPlan(spec SourceOriginCaseSpec) transcodeencoding.Plan {
	return transcodeencoding.Plan{
		SchemaVersion: transcodeencoding.SchemaVersion,
		ProfileID:     "source-origin-180p",
		Transport: transcodeencoding.TransportPlan{
			Protocol:          "hls",
			Container:         "mpegts",
			SegmentFormat:     "mpegts",
			SegmentDurationMS: fixtureSegmentSeconds * 1000,
		},
		Video: transcodeencoding.VideoPlan{
			Codec:                "h264",
			Width:                fixtureWidth,
			Height:               fixtureHeight,
			PixelFormatContract:  "yuv420p-8bit",
			FrameRatePolicy:      "source",
			SourceFrameRateMilli: spec.DeclaredFrameRateMilli(),
			GOPSize:              spec.GOPSize,
			KeyframeIntervalMS:   fixtureSegmentSeconds * 1000,
			ForceKeyframes:       true,
			SceneCut:             false,
			ColorPolicy:          "source",
			ColorPrimaries:       "source",
			Transfer:             "source",
			Matrix:               "source",
		},
		Audio: transcodeencoding.AudioPlan{
			Codec:            "aac",
			Bitrate:          "128k",
			Channels:         2,
			Track:            0,
			SampleRatePolicy: fmt.Sprint(spec.AudioSampleRate),
		},
	}
}

func sourceOriginClass(offsetMicros int64) string {
	switch {
	case offsetMicros > 0:
		return transcodesourceorigin.OriginPositive
	case offsetMicros < 0:
		return transcodesourceorigin.OriginNegative
	default:
		return transcodesourceorigin.OriginZero
	}
}
