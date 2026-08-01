package certification

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	serviceffmpeg "github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
	transcodeencoding "github.com/nowen-video/nowen-video/internal/transcode/encodingplan"
	transcodetimeline "github.com/nowen-video/nowen-video/internal/transcode/timeline"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

const (
	ReportSchemaVersion = "ffmpeg-handoff-fixture-report-v1"
	FixtureCFR48K       = "cfr-h264-aac-48k-software-v1"
)

const (
	fixtureWidth          = 320
	fixtureHeight         = 180
	fixtureFrameRate      = 30
	fixtureSampleRate     = 48000
	fixtureSegmentSeconds = 2
	fixtureBoundaryMS     = 30_000
	fixtureSourceSeconds  = 38
)

type Config struct {
	FFmpegPath  string
	FFprobePath string
	WorkDir     string
	KeepWorkDir bool
}

type Report struct {
	SchemaVersion        string         `json:"schema_version"`
	FixtureID            string         `json:"fixture_id"`
	Backend              string         `json:"backend"`
	FFmpegVersion        string         `json:"ffmpeg_version"`
	FFprobeVersion       string         `json:"ffprobe_version"`
	EncodingPlanVersion  string         `json:"encoding_plan_version"`
	EncodingPlanHash     string         `json:"encoding_plan_hash"`
	TimestampPlanVersion string         `json:"timestamp_plan_version"`
	TimestampPlanHash    string         `json:"timestamp_plan_hash"`
	Startup              ArtifactReport `json:"startup"`
	Continuation         ArtifactReport `json:"continuation"`
	Handoff              HandoffReport  `json:"handoff"`
}

type ArtifactReport struct {
	AttestationVersion string `json:"attestation_version"`
	AttestationHash    string `json:"attestation_hash"`
	SegmentCount       int    `json:"segment_count"`
	VideoStartMS       int64  `json:"video_start_ms"`
	AudioStartMS       int64  `json:"audio_start_ms"`
	VideoEndMS         int64  `json:"video_end_ms"`
	AudioEndMS         int64  `json:"audio_end_ms"`
}

type HandoffReport struct {
	ContractVersion              string `json:"contract_version"`
	ContractHash                 string `json:"contract_hash"`
	Status                       string `json:"status"`
	DecisionReason               string `json:"decision_reason"`
	SeamlessAllowed              bool   `json:"seamless_allowed"`
	DiscontinuityRequired        bool   `json:"discontinuity_required"`
	VideoPresentationDeltaMicros int64  `json:"video_presentation_delta_micros"`
	VideoDecodeDeltaMicros       int64  `json:"video_decode_delta_micros"`
	AudioPresentationDeltaMicros int64  `json:"audio_presentation_delta_micros"`
	AudioDecodeDeltaMicros       int64  `json:"audio_decode_delta_micros"`
}

func (r Report) Validate() error {
	if r.SchemaVersion != ReportSchemaVersion {
		return fmt.Errorf("unsupported fixture report schema %q", r.SchemaVersion)
	}
	if r.FixtureID != FixtureCFR48K || r.Backend != transcodetimestamp.BackendSoftware {
		return fmt.Errorf("fixture identity is invalid")
	}
	if r.FFmpegVersion == "" || r.FFprobeVersion == "" ||
		r.EncodingPlanVersion != transcodeencoding.SchemaVersion || r.EncodingPlanHash == "" ||
		r.TimestampPlanVersion != transcodetimestamp.SchemaVersion || r.TimestampPlanHash == "" {
		return fmt.Errorf("fixture report identities are incomplete")
	}
	for label, artifact := range map[string]ArtifactReport{
		"startup":      r.Startup,
		"continuation": r.Continuation,
	} {
		if artifact.AttestationVersion != transcodeattestation.SchemaVersion || artifact.AttestationHash == "" || artifact.SegmentCount <= 0 {
			return fmt.Errorf("%s artifact report is incomplete", label)
		}
		if artifact.VideoEndMS <= artifact.VideoStartMS || artifact.AudioEndMS <= artifact.AudioStartMS {
			return fmt.Errorf("%s artifact timeline is invalid", label)
		}
	}
	if r.Handoff.ContractVersion != transcodetimeline.SchemaVersion || r.Handoff.ContractHash == "" {
		return fmt.Errorf("handoff identity is incomplete")
	}
	expectedReason := ""
	switch r.Handoff.Status {
	case transcodetimeline.StatusAligned:
		expectedReason = transcodetimeline.DecisionClientCertificationPending
	case transcodetimeline.StatusGap:
		expectedReason = transcodetimeline.DecisionTimelineGap
	case transcodetimeline.StatusOverlap:
		expectedReason = transcodetimeline.DecisionTimelineOverlap
	case transcodetimeline.StatusMixed:
		expectedReason = transcodetimeline.DecisionTimelineMixed
	default:
		return fmt.Errorf("unsupported handoff status %q", r.Handoff.Status)
	}
	if r.Handoff.DecisionReason != expectedReason {
		return fmt.Errorf("handoff decision reason is inconsistent")
	}
	if r.Handoff.SeamlessAllowed || !r.Handoff.DiscontinuityRequired {
		return fmt.Errorf("uncertified fixture cannot authorize seamless playback")
	}
	return nil
}

func Run(ctx context.Context, config Config) (Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ffmpegPath, err := resolveExecutable(config.FFmpegPath, "ffmpeg")
	if err != nil {
		return Report{}, err
	}
	ffprobePath, err := resolveExecutable(config.FFprobePath, "ffprobe")
	if err != nil {
		return Report{}, err
	}
	workDir, cleanup, err := prepareWorkDir(config)
	if err != nil {
		return Report{}, err
	}
	defer cleanup()

	ffmpegVersion, err := commandVersion(ctx, ffmpegPath)
	if err != nil {
		return Report{}, err
	}
	ffprobeVersion, err := commandVersion(ctx, ffprobePath)
	if err != nil {
		return Report{}, err
	}

	plan := fixtureEncodingPlan()
	planVersion, planHash, planJSON, err := transcodeencoding.Identity(plan)
	if err != nil {
		return Report{}, fmt.Errorf("encoding plan identity: %w", err)
	}
	timestampPlan := transcodetimestamp.Default()
	timestampVersion, timestampHash, _, err := transcodetimestamp.Identity(timestampPlan)
	if err != nil {
		return Report{}, fmt.Errorf("timestamp plan identity: %w", err)
	}

	sourcePath := filepath.Join(workDir, "source.mp4")
	if err := runCommand(ctx, ffmpegPath, sourceArgs(sourcePath)...); err != nil {
		return Report{}, fmt.Errorf("generate source fixture: %w", err)
	}
	startupManifest, err := produceHLS(ctx, ffmpegPath, workDir, "startup", sourcePath, timestampPlan, 0, fixtureBoundaryMS/1000)
	if err != nil {
		return Report{}, err
	}
	continuationManifest, err := produceHLS(ctx, ffmpegPath, workDir, "continuation", sourcePath, timestampPlan, fixtureBoundaryMS/1000, 0)
	if err != nil {
		return Report{}, err
	}

	verifier := transcodeattestation.Verifier{FFprobePath: ffprobePath}
	startup, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        startupManifest,
		EncodingPlanVersion: planVersion,
		EncodingPlanHash:    planHash,
		EncodingPlanJSON:    planJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return Report{}, fmt.Errorf("verify startup fixture: %w", err)
	}
	continuation, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        continuationManifest,
		EncodingPlanVersion: planVersion,
		EncodingPlanHash:    planHash,
		EncodingPlanJSON:    planJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return Report{}, fmt.Errorf("verify continuation fixture: %w", err)
	}
	if err := timestampPlan.VerifyObservedStart(0, startup.First.Timeline.Video.StartMS, startup.First.Timeline.Audio.StartMS); err != nil {
		return Report{}, fmt.Errorf("startup timestamp certification: %w", err)
	}
	if err := timestampPlan.VerifyObservedStart(fixtureBoundaryMS, continuation.First.Timeline.Video.StartMS, continuation.First.Timeline.Audio.StartMS); err != nil {
		return Report{}, fmt.Errorf("continuation timestamp certification: %w", err)
	}

	startupVersion, startupHash, _, err := transcodeattestation.Identity(startup)
	if err != nil {
		return Report{}, fmt.Errorf("startup attestation identity: %w", err)
	}
	continuationVersion, continuationHash, _, err := transcodeattestation.Identity(continuation)
	if err != nil {
		return Report{}, fmt.Errorf("continuation attestation identity: %w", err)
	}
	contract, err := transcodetimeline.Evaluate(
		startup,
		startupVersion,
		startupHash,
		continuation,
		continuationVersion,
		continuationHash,
		timestampVersion,
		timestampHash,
		0,
		fixtureBoundaryMS,
		fixtureBoundaryMS,
	)
	if err != nil {
		return Report{}, fmt.Errorf("evaluate handoff fixture: %w", err)
	}
	contractVersion, contractHash, _, err := transcodetimeline.Identity(contract)
	if err != nil {
		return Report{}, fmt.Errorf("handoff contract identity: %w", err)
	}

	report := Report{
		SchemaVersion:        ReportSchemaVersion,
		FixtureID:            FixtureCFR48K,
		Backend:              transcodetimestamp.BackendSoftware,
		FFmpegVersion:        ffmpegVersion,
		FFprobeVersion:       ffprobeVersion,
		EncodingPlanVersion:  planVersion,
		EncodingPlanHash:     planHash,
		TimestampPlanVersion: timestampVersion,
		TimestampPlanHash:    timestampHash,
		Startup:              artifactReport(startup, startupVersion, startupHash),
		Continuation:         artifactReport(continuation, continuationVersion, continuationHash),
		Handoff: HandoffReport{
			ContractVersion:              contractVersion,
			ContractHash:                 contractHash,
			Status:                       contract.Status,
			DecisionReason:               contract.DecisionReason,
			SeamlessAllowed:              contract.SeamlessAllowed,
			DiscontinuityRequired:        contract.DiscontinuityRequired,
			VideoPresentationDeltaMicros: contract.Video.PresentationDeltaMicros,
			VideoDecodeDeltaMicros:       contract.Video.DecodeDeltaMicros,
			AudioPresentationDeltaMicros: contract.Audio.PresentationDeltaMicros,
			AudioDecodeDeltaMicros:       contract.Audio.DecodeDeltaMicros,
		},
	}
	if err := report.Validate(); err != nil {
		return Report{}, err
	}
	return report, nil
}

func fixtureEncodingPlan() transcodeencoding.Plan {
	return transcodeencoding.Plan{
		SchemaVersion: transcodeencoding.SchemaVersion,
		ProfileID:     "fixture-180p",
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
			SourceFrameRateMilli: fixtureFrameRate * 1000,
			GOPSize:              fixtureFrameRate * fixtureSegmentSeconds,
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
			SampleRatePolicy: "48000",
		},
	}
}

func sourceArgs(output string) []string {
	return []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc2=size=%dx%d:rate=%d:duration=%d", fixtureWidth, fixtureHeight, fixtureFrameRate, fixtureSourceSeconds),
		"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=1000:sample_rate=%d:duration=%d", fixtureSampleRate, fixtureSourceSeconds),
		"-map", "0:v:0", "-map", "1:a:0",
		"-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p",
		"-g", fmt.Sprint(fixtureFrameRate * fixtureSegmentSeconds),
		"-keyint_min", fmt.Sprint(fixtureFrameRate * fixtureSegmentSeconds),
		"-sc_threshold", "0",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", fmt.Sprint(fixtureSampleRate),
		"-shortest", "-movflags", "+faststart", output,
	}
}

func produceHLS(
	ctx context.Context,
	ffmpegPath,
	workDir,
	name,
	sourcePath string,
	timestampPlan transcodetimestamp.Plan,
	startSeconds,
	durationSeconds int64,
) (string, error) {
	directory := filepath.Join(workDir, name)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create %s fixture directory: %w", name, err)
	}
	args, err := fixtureHLSArgs(sourcePath, directory, timestampPlan, startSeconds, durationSeconds)
	if err != nil {
		return "", fmt.Errorf("build %s fixture command: %w", name, err)
	}
	if err := runCommand(ctx, ffmpegPath, args...); err != nil {
		return "", fmt.Errorf("produce %s fixture: %w", name, err)
	}
	return filepath.Join(directory, "stream.m3u8"), nil
}

func fixtureHLSArgs(
	sourcePath,
	outputDir string,
	timestampPlan transcodetimestamp.Plan,
	startSeconds,
	durationSeconds int64,
) ([]string, error) {
	args := serviceffmpeg.BuildHLSArgs(serviceffmpeg.BuildOptions{
		InputPath: sourcePath,
		OutputDir: outputDir,
		HWAccel:   serviceffmpeg.HWAccelNone,
		Profile: serviceffmpeg.Profile{
			Width:        fixtureWidth,
			Height:       fixtureHeight,
			VideoBitrate: "800k",
			AudioBitrate: "128k",
			MaxBitrate:   "900k",
			BufSize:      "1600k",
		},
		X264Preset:      "veryfast",
		Threads:         1,
		UseCRF:          true,
		CRF:             23,
		VideoFilter:     fmt.Sprintf("scale=%d:%d", fixtureWidth, fixtureHeight),
		HLSTime:         fixtureSegmentSeconds,
		HLSFlags:        "independent_segments+append_list+program_date_time",
		HLSPlaylistType: "event",
		StartNumber:     int(startSeconds / fixtureSegmentSeconds),
		ForceKeyFrames:  true,
		StartOffsetSec:  float64(startSeconds),
		GOPSize:         fixtureFrameRate * fixtureSegmentSeconds,
	})
	if durationSeconds > 0 {
		var err error
		args, err = asBoundedStartupVOD(args, durationSeconds)
		if err != nil {
			return nil, err
		}
	}
	return transcodetimestamp.ApplyFFmpeg(args, timestampPlan)
}

func asBoundedStartupVOD(args []string, durationSeconds int64) ([]string, error) {
	if len(args) == 0 || strings.TrimSpace(args[len(args)-1]) == "" {
		return nil, fmt.Errorf("ffmpeg arguments do not contain an output path")
	}
	body := make([]string, 0, len(args)+2)
	for index := 0; index < len(args)-1; index++ {
		arg := args[index]
		if arg == "-hls_playlist_type" && index+1 < len(args)-1 {
			index++
			continue
		}
		if arg == "-hls_flags" && index+1 < len(args)-1 {
			flags := strings.Split(args[index+1], "+")
			kept := make([]string, 0, len(flags))
			for _, flag := range flags {
				if flag != "append_list" {
					kept = append(kept, flag)
				}
			}
			body = append(body, arg, strings.Join(kept, "+"))
			index++
			continue
		}
		body = append(body, arg)
	}
	body = append(body, "-t", fmt.Sprint(durationSeconds), "-hls_playlist_type", "vod")
	return append(body, args[len(args)-1]), nil
}

func artifactReport(attestation transcodeattestation.Attestation, version, hash string) ArtifactReport {
	return ArtifactReport{
		AttestationVersion: version,
		AttestationHash:    hash,
		SegmentCount:       attestation.SegmentCount,
		VideoStartMS:       attestation.First.Timeline.Video.StartMS,
		AudioStartMS:       attestation.First.Timeline.Audio.StartMS,
		VideoEndMS:         attestation.Last.Timeline.Video.EndMS,
		AudioEndMS:         attestation.Last.Timeline.Audio.EndMS,
	}
}

func resolveExecutable(configured, fallback string) (string, error) {
	candidate := strings.TrimSpace(configured)
	if candidate == "" {
		candidate = fallback
	}
	path, err := exec.LookPath(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", fallback, err)
	}
	return path, nil
}

func commandVersion(ctx context.Context, path string) (string, error) {
	output, err := exec.CommandContext(ctx, path, "-version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read %s version: %w: %s", filepath.Base(path), err, strings.TrimSpace(string(output)))
	}
	line := strings.TrimSpace(string(output))
	if index := strings.IndexByte(line, '\n'); index >= 0 {
		line = line[:index]
	}
	if line == "" {
		return "", fmt.Errorf("%s returned an empty version", filepath.Base(path))
	}
	return line, nil
}

func runCommand(ctx context.Context, path string, args ...string) error {
	command := exec.CommandContext(ctx, path, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w: %s", filepath.Base(path), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func prepareWorkDir(config Config) (string, func(), error) {
	if strings.TrimSpace(config.WorkDir) == "" {
		directory, err := os.MkdirTemp("", "nowen-transcode-certification-")
		if err != nil {
			return "", nil, fmt.Errorf("create fixture work directory: %w", err)
		}
		cleanup := func() {
			if !config.KeepWorkDir {
				_ = os.RemoveAll(directory)
			}
		}
		return directory, cleanup, nil
	}
	directory, err := filepath.Abs(config.WorkDir)
	if err != nil {
		return "", nil, fmt.Errorf("resolve fixture work directory: %w", err)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", nil, fmt.Errorf("create fixture work directory: %w", err)
	}
	return directory, func() {}, nil
}

func MarshalReport(report Report) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal fixture report: %w", err)
	}
	return append(content, '\n'), nil
}
