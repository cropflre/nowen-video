package corpusgenerator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
)

type Options struct {
	FFmpegPath  string
	FFprobePath string
	OutputDir   string
	Spec        transcodecorpus.Spec
	Runner      Runner
}

func Generate(ctx context.Context, options Options) (transcodecorpus.Manifest, string, error) {
	if options.FFmpegPath == "" {
		options.FFmpegPath = "ffmpeg"
	}
	if options.FFprobePath == "" {
		options.FFprobePath = "ffprobe"
	}
	if strings.TrimSpace(options.OutputDir) == "" {
		return transcodecorpus.Manifest{}, "", fmt.Errorf("output directory is required")
	}
	if options.Runner == nil {
		options.Runner = ExecRunner{}
	}
	if len(options.Spec.Cases) == 0 {
		options.Spec = transcodecorpus.DefaultSpec()
	}
	if err := options.Spec.Validate(); err != nil {
		return transcodecorpus.Manifest{}, "", err
	}
	specVersion, specHash, _, err := transcodecorpus.SpecIdentity(options.Spec)
	if err != nil {
		return transcodecorpus.Manifest{}, "", err
	}
	ffmpegVersion, err := toolVersion(ctx, options.Runner, options.FFmpegPath)
	if err != nil {
		return transcodecorpus.Manifest{}, "", err
	}
	ffprobeVersion, err := toolVersion(ctx, options.Runner, options.FFprobePath)
	if err != nil {
		return transcodecorpus.Manifest{}, "", err
	}
	outputDir, err := filepath.Abs(options.OutputDir)
	if err != nil {
		return transcodecorpus.Manifest{}, "", err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return transcodecorpus.Manifest{}, "", err
	}
	stagingDir, err := os.MkdirTemp(outputDir, ".corpus-staging-")
	if err != nil {
		return transcodecorpus.Manifest{}, "", err
	}
	defer os.RemoveAll(stagingDir)

	assets := make([]transcodecorpus.AssetEvidence, 0, len(options.Spec.Cases))
	stagedFiles := make([]string, 0, len(options.Spec.Cases))
	for _, caseSpec := range options.Spec.Cases {
		repeatHashes := make([]string, 0, RepeatCount)
		repeatPaths := make([]string, 0, RepeatCount)
		var commandPlan CommandPlan
		for repeat := 1; repeat <= RepeatCount; repeat++ {
			extension := extensionFor(caseSpec.Source.Container)
			repeatPath := filepath.Join(stagingDir, fmt.Sprintf("%s.run-%02d%s", caseSpec.ID, repeat, extension))
			plan, err := BuildCommand(caseSpec, repeatPath)
			if err != nil {
				return transcodecorpus.Manifest{}, "", err
			}
			if repeat == 1 {
				commandPlan = plan
			} else if plan.CommandSHA256 != commandPlan.CommandSHA256 || plan.RelativePath != commandPlan.RelativePath {
				return transcodecorpus.Manifest{}, "", fmt.Errorf("case %s command identity changed between repeats", caseSpec.ID)
			}
			if _, err := options.Runner.Run(ctx, options.FFmpegPath, plan.Args...); err != nil {
				return transcodecorpus.Manifest{}, "", fmt.Errorf("generate corpus case %s repeat %d: %w", caseSpec.ID, repeat, err)
			}
			hash, size, err := fileIdentity(repeatPath)
			if err != nil {
				return transcodecorpus.Manifest{}, "", err
			}
			if size == 0 {
				return transcodecorpus.Manifest{}, "", fmt.Errorf("case %s repeat %d produced an empty file", caseSpec.ID, repeat)
			}
			repeatHashes = append(repeatHashes, hash)
			repeatPaths = append(repeatPaths, repeatPath)
		}
		for _, hash := range repeatHashes[1:] {
			if hash != repeatHashes[0] {
				return transcodecorpus.Manifest{}, "", fmt.Errorf("case %s is not byte-deterministic across repeats", caseSpec.ID)
			}
		}
		probe, err := ProbeAsset(ctx, options.Runner, options.FFprobePath, repeatPaths[0], caseSpec.Source)
		if err != nil {
			return transcodecorpus.Manifest{}, "", fmt.Errorf("probe corpus case %s: %w", caseSpec.ID, err)
		}
		_, size, err := fileIdentity(repeatPaths[0])
		if err != nil {
			return transcodecorpus.Manifest{}, "", err
		}
		assets = append(assets, transcodecorpus.AssetEvidence{
			CaseID:        caseSpec.ID,
			RelativePath:  commandPlan.RelativePath,
			CommandSHA256: commandPlan.CommandSHA256,
			SHA256:        repeatHashes[0],
			RepeatSHA256:  append([]string(nil), repeatHashes...),
			SizeBytes:     size,
			Probe:         probe,
		})
		stagedFiles = append(stagedFiles, repeatPaths[0])
	}

	manifest := transcodecorpus.Manifest{
		SchemaVersion:         transcodecorpus.ManifestSchemaVersion,
		SpecVersion:           specVersion,
		SpecHash:              specHash,
		GeneratorVersion:      GeneratorVersion,
		GenerationRepeatCount: RepeatCount,
		FFmpegVersion:         ffmpegVersion,
		FFprobeVersion:        ffprobeVersion,
		Assets:                assets,
		SeamlessAllowed:       false,
		DiscontinuityRequired: true,
	}
	manifestVersion, manifestHash, canonical, err := transcodecorpus.ManifestIdentity(manifest, options.Spec)
	if err != nil {
		return transcodecorpus.Manifest{}, "", err
	}
	if manifestVersion != transcodecorpus.ManifestSchemaVersion {
		return transcodecorpus.Manifest{}, "", fmt.Errorf("unexpected manifest version %q", manifestVersion)
	}
	assetsDir := filepath.Join(outputDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		return transcodecorpus.Manifest{}, "", err
	}
	for index, asset := range assets {
		finalPath := filepath.Join(outputDir, filepath.FromSlash(asset.RelativePath))
		if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
			return transcodecorpus.Manifest{}, "", err
		}
		if err := replaceFile(stagedFiles[index], finalPath); err != nil {
			return transcodecorpus.Manifest{}, "", err
		}
	}
	manifestPath := filepath.Join(outputDir, "real-media-corpus-manifest-v1.json")
	if err := os.WriteFile(manifestPath, append([]byte(canonical), '\n'), 0o644); err != nil {
		return transcodecorpus.Manifest{}, "", err
	}
	return manifest, manifestHash, nil
}

func toolVersion(ctx context.Context, runner Runner, path string) (string, error) {
	output, err := runner.Run(ctx, path, "-version")
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0])
	if line == "" {
		return "", fmt.Errorf("tool %s returned an empty version", path)
	}
	return line, nil
}

func fileIdentity(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(digest.Sum(nil)), size, nil
}

func replaceFile(source, destination string) error {
	if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(source, destination); err == nil {
		return nil
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	return os.Remove(source)
}
