package certification

import (
	"context"
	"fmt"
	"path/filepath"

	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
)

type boundaryContractRequest struct {
	Case                           BoundaryCaseSpec
	Fixture                        FixtureSpec
	StartupManifest                string
	ContinuationManifest           string
	FFmpegVersion                  string
	FFprobeVersion                 string
	EncodingPlanVersion            string
	EncodingPlanHash               string
	TimestampPlanVersion           string
	TimestampPlanHash              string
	StartupAttestationVersion      string
	StartupAttestationHash         string
	ContinuationAttestationVersion string
	ContinuationAttestationHash    string
}

func probeBoundaryContract(ctx context.Context, ffprobePath string, request boundaryContractRequest) (transcodeboundary.Contract, error) {
	startupSegments, err := readBoundaryManifestSegments(request.StartupManifest)
	if err != nil {
		return transcodeboundary.Contract{}, err
	}
	continuationSegments, err := readBoundaryManifestSegments(request.ContinuationManifest)
	if err != nil {
		return transcodeboundary.Contract{}, err
	}
	if len(startupSegments) == 0 || len(continuationSegments) == 0 {
		return transcodeboundary.Contract{}, fmt.Errorf("boundary manifests must contain media segments")
	}
	startupName := startupSegments[len(startupSegments)-1]
	continuationName := continuationSegments[0]
	startup, err := probeBoundarySegment(ctx, ffprobePath, filepath.Join(filepath.Dir(request.StartupManifest), startupName), startupName)
	if err != nil {
		return transcodeboundary.Contract{}, fmt.Errorf("probe boundary startup tail: %w", err)
	}
	continuation, err := probeBoundarySegment(ctx, ffprobePath, filepath.Join(filepath.Dir(request.ContinuationManifest), continuationName), continuationName)
	if err != nil {
		return transcodeboundary.Contract{}, fmt.Errorf("probe boundary continuation head: %w", err)
	}
	video, err := buildBoundaryStream(transcodeboundary.StreamVideo, startup.Video, continuation.Video)
	if err != nil {
		return transcodeboundary.Contract{}, fmt.Errorf("build video boundary evidence: %w", err)
	}
	audio, err := buildBoundaryStream(transcodeboundary.StreamAudio, startup.Audio, continuation.Audio)
	if err != nil {
		return transcodeboundary.Contract{}, fmt.Errorf("build audio boundary evidence: %w", err)
	}
	contract := transcodeboundary.Contract{
		SchemaVersion:                  transcodeboundary.SchemaVersion,
		CaseID:                         request.Case.ID,
		FixtureID:                      request.Fixture.ID,
		ExpectedBoundaryMicros:         request.Case.ExpectedBoundaryMicros,
		FFmpegVersion:                  request.FFmpegVersion,
		FFprobeVersion:                 request.FFprobeVersion,
		EncodingPlanVersion:            request.EncodingPlanVersion,
		EncodingPlanHash:               request.EncodingPlanHash,
		TimestampPlanVersion:           request.TimestampPlanVersion,
		TimestampPlanHash:              request.TimestampPlanHash,
		StartupAttestationVersion:      request.StartupAttestationVersion,
		StartupAttestationHash:         request.StartupAttestationHash,
		ContinuationAttestationVersion: request.ContinuationAttestationVersion,
		ContinuationAttestationHash:    request.ContinuationAttestationHash,
		Video:                          video,
		Audio:                          audio,
		DiscontinuityRequired:          true,
	}
	if err := contract.Validate(); err != nil {
		return transcodeboundary.Contract{}, err
	}
	return contract, nil
}
