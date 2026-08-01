package certification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
	transcodeencoding "github.com/nowen-video/nowen-video/internal/transcode/encodingplan"
	transcodetimeline "github.com/nowen-video/nowen-video/internal/transcode/timeline"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

const (
	ReportSchemaVersion = "transcode-fixture-certification-v1"
	FixtureIDSoftwareCFR = "software-cfr-h264-aac-30fps-48khz-v1"
)

type Config struct {
	FFmpegPath  string
	FFprobePath string
	WorkDir     string
	KeepWorkDir bool
}

type Report struct {
	SchemaVersion string               `json:"schema_version"`
	FixtureID     string               `json:"fixture_id"`
	GeneratedAt   time.Time            `json:"generated_at"`
	Workspace     string               `json:"workspace,omitempty"`
	Tools         ToolReport           `json:"tools"`
	Source        SourceReport         `json:"source"`
	EncodingPlan  IdentityReport       `json:"encoding_plan"`
	TimestampPlan IdentityReport       `json:"timestamp_plan"`
	Startup       ArtifactReport       `json:"startup"`
	Continuation  ArtifactReport       `json:"continuation"`
	Handoff       HandoffReport        `json:"handoff"`
}

type ToolReport struct {
	FFmpegPath    string `json:"ffmpeg_path"`
	FFmpegVersion string `json:"ffmpeg_version"`
	FFprobePath   string `json:"ffprobe_path"`
	FFprobeVersion string `json:"ffprobe_version"`
}

type SourceReport struct {
	Path       string `json:"path,omitempty"`
	SHA256     string `json:"sha256"`
	DurationMS int64  `json:"duration_ms"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	FPSMilli   int    `json:"fps_milli"`
	SampleRate int    `json:"sample_rate"`
}

type IdentityReport struct {
	Version string `json:"version"`
	Hash    string `json:"hash"`
	JSON    string `json:"canonical_json"`
}

type ArtifactReport struct {
	ManifestPath       string                           `json:"manifest_path,omitempty"`
	TimelineOriginMS   int64                            `json:"timeline_origin_ms"`
	CommandSummary     string                           `json:"command_summary"`
	AttestationVersion string                           `json:"attestation_version"`
	AttestationHash    string                           `json:"attestation_hash"`
	Attestation        transcodeattestation.Attestation `json:"attestation"`
}

type HandoffReport struct {
	Version               string                     `json:"version"`
	Hash                  string                     `json:"hash"`
	Status                string                     `json:"status"`
	DecisionReason        string                     `json:"decision_reason"`
	SeamlessAllowed       bool                       `json:"seamless_allowed"`
	DiscontinuityRequired bool                       `json:"discontinuity_required"`
	Contract              transcodetimeline.Contract `json:"contract"`
}

func Run(ctx context.Context, cfg Config) (Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ffmpegPath, err := resolveExecutable(cfg.FFmpegPath, "ffmpeg")
	if err != nil {
		return Report{}, err
	}
	ffprobePath, err := resolveExecutable(cfg.FFprobePath, "ffprobe")
	if err != nil {
		return Report{}, err
	}
	workspace, cleanup, err := prepareWorkspace(cfg)
	if err != nil {
		return Report{}, err
	}
	defer cleanup()

	ffmpegVersion, err := executableVersion(ctx, ffmpegPath)
	if err != nil {
		return Report{}, err
	}
	ffprobeVersion, err := executableVersion(ctx, ffprobePath)
	if err != nil {
		return Report{}, err
	}

	encodingPlan := fixtureEncodingPlan()
	encodingVersion, encodingHash, encodingJSON, err := transcodeencoding.Identity(encodingPlan)
	if err != nil {
		return Report{}, fmt.Errorf("build fixture encoding plan: %w", err)
	}
	timestampPlan := transcodetimestamp.Default()
	timestampVersion, timestampHash, timestampJSON, err := transcodetimestamp.Identity(timestampPlan)
	if err != nil {
		return Report{}, fmt.Errorf("build fixture timestamp plan: %w", err)
	}

	sourcePath := filepath.Join(workspace, "source.mp4")
	if err := runCommand(ctx, ffmpegPath, sourceFixtureArgs(sourcePath)...); err != nil {
		return Report{}, fmt.Errorf("generate source fixture: %w", err)
	}
	sourceHash, err := fileSHA256(sourcePath)
	if err != nil {
		return Report{}, fmt.Errorf("hash source fixture: %w", err)
	}

	startupDir := filepath.Join(workspace, "startup")
	continuationDir := filepath.Join(workspace, "continuation")
	for _, directory := range []string{startupDir, continuationDir} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return Report{}, fmt.Errorf("create fixture output directory: %w", err)
		}
	}
	startupArgs, err := artifactArgs(sourcePath, startupDir, 0, 4, timestampPlan)
	if err != nil {
		return Report{}, fmt.Errorf("build startup fixture command: %w", err)
	}
	continuationArgs, err := artifactArgs(sourcePath, continuationDir, 4, 0, timestampPlan)
	if err != nil {
		return Report{}, fmt.Errorf("build continuation fixture command: %w", err)
	}
	if err := runCommand(ctx, ffmpegPath, startupArgs...); err != nil {
		return Report{}, fmt.Errorf("encode startup fixture: %w", err)
	}
	if err := runCommand(ctx, ffmpegPath, continuationArgs...); err != nil {
		return Report{}, fmt.Errorf("encode continuation fixture: %w", err)
	}

	verifier := transcodeattestation.Verifier{FFprobePath: ffprobePath}
	startupManifest := filepath.Join(startupDir, "stream.m3u8")
	continuationManifest := filepath.Join(continuationDir, "stream.m3u8")
	startupAttestation, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        startupManifest,
		EncodingPlanVersion: encodingVersion,
		EncodingPlanHash:    encodingHash,
		EncodingPlanJSON:    encodingJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return Report{}, fmt.Errorf("attest startup fixture: %w", err)
	}
	continuationAttestation, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        continuationManifest,
		EncodingPlanVersion: encodingVersion,
		EncodingPlanHash:    encodingHash,
		EncodingPlanJSON:    encodingJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return Report{}, fmt.Errorf("attest continuation fixture: %w", err)
	}
	if err := timestampPlan.VerifyObservedStart(
		0,
		startupAttestation.First.Timeline.Video.StartMS,
		startupAttestation.First.Timeline.Audio.StartMS,
	); err != nil {
		return Report{}, fmt.Errorf("verify startup origin: %w", err)
	}
	if err := timestampPlan.VerifyObservedStart(
		4000,
		continuationAttestation.First.Timeline.Video.StartMS,
		continuationAttestation.First.Timeline.Audio.StartMS,
	); err != nil {
		return Report{}, fmt.Errorf("verify continuation origin: %w", err)
	}
	startupAttestationVersion, startupAttestationHash, _, err := transcodeattestation.Identity(startupAttestation)
	if err != nil {
		return Report{}, fmt.Errorf("identify startup attestation: %w", err)
	}
	continuationAttestationVersion, continuationAttestationHash, _, err := transcodeattestation.Identity(continuationAttestation)
	if err != nil {
		return Report{}, fmt.Errorf("identify continuation attestation: %w", err)
	}
	handoff, err := transcodetimeline.Evaluate(
		startupAttestation,
		startupAttestationVersion,
		startupAttestationHash,
		continuationAttestation,
		continuationAttestationVersion,
		continuationAttestationHash,
		timestampVersion,
		timestampHash,
		0,
		4000,
		4000,
	)
	if err != nil {
		return Report{}, fmt.Errorf("evaluate fixture handoff: %w", err)
	}
	handoffVersion, handoffHash, _, err := transcodetimeline.Identity(handoff)
	if err != nil {
		return Report{}, fmt.Errorf("identify fixture handoff: %w", err)
	}

	report := Report{
		SchemaVersion: ReportSchemaVersion,
		FixtureID:     FixtureIDSoftwareCFR,
		GeneratedAt:   time.Now().UTC(),
		Tools: ToolReport{
			FFmpegPath:     ffmpegPath,
			FFmpegVersion:  ffmpegVersion,
			FFprobePath:    ffprobePath,
			FFprobeVersion: ffprobeVersion,
		},
		Source: SourceReport{
			SHA256:     sourceHash,
			DurationMS: 8000,
			Width:      320,
			Height:     180,
			FPSMilli:   30000,
			SampleRate: 48000,
		},
		EncodingPlan: IdentityReport{Version: encodingVersion, Hash: encodingHash, JSON: encodingJSON},
		TimestampPlan: IdentityReport{Version: timestampVersion, Hash: timestampHash, JSON: timestampJSON},
		Startup: ArtifactReport{
			TimelineOriginMS:   0,
			CommandSummary:     transcodetimestamp.CommandSummary(startupArgs),
			AttestationVersion: startupAttestationVersion,
			AttestationHash:    startupAttestationHash,
			Attestation:        startupAttestation,
		},
		Continuation: ArtifactReport{
			TimelineOriginMS:   4000,
			CommandSummary:     transcodetimestamp.CommandSummary(continuationArgs),
			AttestationVersion: continuationAttestationVersion,
			AttestationHash:    continuationAttestationHash,
			Attestation:        continuationAttestation,
		},
		Handoff: HandoffReport{
			Version:               handoffVersion,
			Hash:                  handoffHash,
			Status:                handoff.Status,
			DecisionReason:        handoff.DecisionReason,
			SeamlessAllowed:       handoff.SeamlessAllowed,
			DiscontinuityRequired: handoff.DiscontinuityRequired,
			Contract:              handoff,
		},
	}
	if cfg.KeepWorkDir || strings.TrimSpace(cfg.WorkDir) != "" {
		report.Workspace = workspace
		report.Source.Path = sourcePath
		report.Startup.ManifestPath = startupManifest
		report.Continuation.ManifestPath = continuationManifest
	}
	if err := report.Validate(); err != nil {
		return Report{}, err
	}
	return report, nil
}

func (r Report) Validate() error {
	if r.SchemaVersion != ReportSchemaVersion || r.FixtureID != FixtureIDSoftwareCFR {
		return fmt.Errorf("fixture report identity is invalid")
	}
	if r.Tools.FFmpegPath == "" || r.Tools.FFmpegVersion == "" || r.Tools.FFprobePath == "" || r.Tools.FFprobeVersion == "" {
		return fmt.Errorf("fixture tool evidence is incomplete")
	}
	if r.Source.SHA256 == "" || r.Source.DurationMS <= 0 || r.Source.Width <= 0 || r.Source.Height <= 0 || r.Source.FPSMilli <= 0 || r.Source.SampleRate <= 0 {
		return fmt.Errorf("fixture source evidence is incomplete")
	}
	if r.EncodingPlan.Version == "" || r.EncodingPlan.Hash == "" || r.EncodingPlan.JSON == "" ||
		r.TimestampPlan.Version == "" || r.TimestampPlan.Hash == "" || r.TimestampPlan.JSON == "" {
		return fmt.Errorf("fixture plan identities are incomplete")
	}
	if r.Startup.AttestationHash == "" || r.Continuation.AttestationHash == "" || r.Handoff.Hash == "" {
		return fmt.Errorf("fixture media evidence is incomplete")
	}
	if r.Startup.TimelineOriginMS != 0 || r.Continuation.TimelineOriginMS != 4000 {
		return fmt.Errorf("fixture timeline origins are invalid")
	}
	if r.Handoff.Contract.Status != r.Handoff.Status || r.Handoff.Contract.DecisionReason != r.Handoff.DecisionReason ||
		r.Handoff.Contract.SeamlessAllowed != r.Handoff.SeamlessAllowed ||
		r.Handoff.Contract.DiscontinuityRequired != r.Handoff.DiscontinuityRequired {
		return fmt.Errorf("fixture handoff projection is invalid")
	}
	if r.Handoff.SeamlessAllowed || !r.Handoff.DiscontinuityRequired {
		return fmt.Errorf("fixture certification cannot authorize seamless playback")
	}
	return nil
}

func MarshalReport(report Report) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal fixture certification report: %w", err)
	}
	return append(content, '\n'), nil
}

func fixtureEncodingPlan() transcodeencoding.Plan {
	return transcodeencoding.Plan{
		SchemaVersion: transcodeencoding.SchemaVersion,
		ProfileID:     "fixture-320x180",
		Transport: transcodeencoding.TransportPlan{
			Protocol:          "hls",
			Container:         "mpegts",
			SegmentFormat:     "mpegts",
			SegmentDurationMS: 2000,
		},
		Video: transcodeencoding.VideoPlan{
			Codec:                "h264",
			Width:                320,
			Height:               180,
			PixelFormatContract:  "yuv420p-8bit",
			FrameRatePolicy:      "source_passthrough",
			SourceFrameRateMilli: 30000,
			GOPSize:              60,
			KeyframeIntervalMS:   2000,
			ForceKeyframes:       true,
			SceneCut:             false,
			ColorPolicy:          "source_sdr",
			ColorPrimaries:       "source",
			Transfer:             "source",
			Matrix:               "source",
		},
		Audio: transcodeencoding.AudioPlan{
			Codec:            "aac",
			Bitrate:          "128k",
			Channels:         2,
			Track:            0,
			SampleRatePolicy: "source",
		},
	}
}

func sourceFixtureArgs(outputPath string) []string {
	return []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=320x180:rate=30:duration=8",
		"-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=48000:duration=8",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "ultrafast",
		"-g", "60", "-keyint_min", "60", "-sc_threshold", "0",
		"-c:a", "aac", "-b:a", "128k", "-ar", "48000", "-ac", "2",
		"-shortest", outputPath,
	}
}

func artifactArgs(sourcePath, outputDir string, startSeconds, durationSeconds int, plan transcodetimestamp.Plan) ([]string, error) {
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}
	if startSeconds > 0 {
		args = append(args, "-ss", fmt.Sprintf("%d", startSeconds))
	}
	args = append(args,
		"-i", sourcePath,
		"-map", "0:v:0", "-map", "0:a:0",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "ultrafast",
		"-g", "60", "-keyint_min", "60", "-sc_threshold", "0",
		"-c:a", "aac", "-b:a", "128k", "-ar", "48000", "-ac", "2",
	)
	if durationSeconds > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", durationSeconds))
	}
	args = append(args,
		"-f", "hls", "-hls_time", "2", "-hls_list_size", "0",
		"-hls_playlist_type", "vod", "-hls_flags", "independent_segments",
		"-hls_segment_filename", filepath.Join(outputDir, "seg%04d.ts"),
		filepath.Join(outputDir, "stream.m3u8"),
	)
	return transcodetimestamp.ApplyFFmpeg(args, plan)
}

func resolveExecutable(configured, fallback string) (string, error) {
	candidate := strings.TrimSpace(configured)
	if candidate == "" {
		candidate = fallback
	}
	resolved, err := exec.LookPath(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve %s executable: %w", fallback, err)
	}
	return resolved, nil
}

func executableVersion(ctx context.Context, path string) (string, error) {
	command := exec.CommandContext(ctx, path, "-version")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read %s version: %w: %s", filepath.Base(path), err, strings.TrimSpace(string(output)))
	}
	line := strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0])
	if line == "" {
		return "", fmt.Errorf("%s returned an empty version", filepath.Base(path))
	}
	return line, nil
}

func prepareWorkspace(cfg Config) (string, func(), error) {
	base := strings.TrimSpace(cfg.WorkDir)
	if base != "" {
		if err := os.MkdirAll(base, 0o755); err != nil {
			return "", nil, fmt.Errorf("create fixture work root: %w", err)
		}
		workspace, err := os.MkdirTemp(base, "certification-")
		if err != nil {
			return "", nil, fmt.Errorf("create fixture workspace: %w", err)
		}
		return workspace, func() {}, nil
	}
	workspace, err := os.MkdirTemp("", "nowen-transcode-certification-")
	if err != nil {
		return "", nil, fmt.Errorf("create fixture workspace: %w", err)
	}
	cleanup := func() {
		if !cfg.KeepWorkDir {
			_ = os.RemoveAll(workspace)
		}
	}
	return workspace, cleanup, nil
}

func runCommand(ctx context.Context, path string, args ...string) error {
	command := exec.CommandContext(ctx, path, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w: %s", filepath.Base(path), strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}
