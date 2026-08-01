package realmediacorpus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func SpecIdentity(spec Spec) (version, hash, canonical string, err error) {
	if err := spec.Validate(); err != nil {
		return "", "", "", err
	}
	content, err := json.Marshal(spec)
	if err != nil {
		return "", "", "", fmt.Errorf("marshal real-media corpus spec: %w", err)
	}
	digest := sha256.Sum256(content)
	return spec.SchemaVersion, hex.EncodeToString(digest[:]), string(content), nil
}

func ManifestIdentity(manifest Manifest, spec Spec) (version, hash, canonical string, err error) {
	if err := manifest.ValidateFor(spec); err != nil {
		return "", "", "", err
	}
	content, err := json.Marshal(manifest)
	if err != nil {
		return "", "", "", fmt.Errorf("marshal real-media corpus manifest: %w", err)
	}
	digest := sha256.Sum256(content)
	return manifest.SchemaVersion, hex.EncodeToString(digest[:]), string(content), nil
}
