package certification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
)

type RealMediaCandidateConfig struct {
	Config
	CorpusRoot   string
	ManifestPath string
}

func loadRealMediaCorpus(config RealMediaCandidateConfig) (string, transcodecorpus.Spec, transcodecorpus.Manifest, string, string, error) {
	manifestPath := strings.TrimSpace(config.ManifestPath)
	root := strings.TrimSpace(config.CorpusRoot)
	if manifestPath == "" && root == "" {
		return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", fmt.Errorf("real-media corpus root or manifest path is required")
	}
	var err error
	if manifestPath == "" {
		manifestPath = filepath.Join(root, "real-media-corpus-manifest-v1.json")
	}
	manifestPath, err = filepath.Abs(manifestPath)
	if err != nil {
		return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", fmt.Errorf("resolve real-media manifest path: %w", err)
	}
	if root == "" {
		root = filepath.Dir(manifestPath)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", fmt.Errorf("resolve real-media corpus root: %w", err)
	}
	if err := requirePathWithin(root, manifestPath); err != nil {
		return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", fmt.Errorf("manifest path: %w", err)
	}
	file, err := os.Open(manifestPath)
	if err != nil {
		return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", fmt.Errorf("open real-media manifest: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest transcodecorpus.Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", fmt.Errorf("decode real-media manifest: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", err
	}
	spec := transcodecorpus.DefaultSpec()
	if err := manifest.ValidateFor(spec); err != nil {
		return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", err
	}
	manifestVersion, manifestHash, _, err := transcodecorpus.ManifestIdentity(manifest, spec)
	if err != nil {
		return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", err
	}
	for _, asset := range manifest.Assets {
		path := filepath.Join(root, filepath.FromSlash(asset.RelativePath))
		if err := requirePathWithin(root, path); err != nil {
			return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", fmt.Errorf("asset %s: %w", asset.CaseID, err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", fmt.Errorf("stat asset %s: %w", asset.CaseID, err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", fmt.Errorf("asset %s is not a regular file", asset.CaseID)
		}
		if info.Size() != asset.SizeBytes {
			return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", fmt.Errorf("asset %s size differs from manifest", asset.CaseID)
		}
		hash, err := realMediaFileSHA256(path)
		if err != nil {
			return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", fmt.Errorf("hash asset %s: %w", asset.CaseID, err)
		}
		if hash != asset.SHA256 {
			return "", transcodecorpus.Spec{}, transcodecorpus.Manifest{}, "", "", fmt.Errorf("asset %s SHA-256 differs from manifest", asset.CaseID)
		}
	}
	return root, spec, manifest, manifestVersion, manifestHash, nil
}

func requirePathWithin(root, path string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("path escapes corpus root")
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("real-media manifest contains trailing JSON")
		}
		return fmt.Errorf("decode real-media manifest trailing content: %w", err)
	}
	return nil
}

func realMediaFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
