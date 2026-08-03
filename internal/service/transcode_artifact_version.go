package service

import (
	"fmt"
	"os"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

// resolveHLSArtifactSnapshot resolves the current readable directory and then
// binds that exact physical path back to its Artifact row. The second lookup is
// path-based so a concurrent publication cannot silently replace the Artifact
// identity attached to a playlist that was read from the previous directory.
func (s *TranscodeService) resolveHLSArtifactSnapshot(media *model.Media, quality string) (*model.TranscodeArtifactRecord, string, error) {
	if s == nil || s.executionRepo == nil || media == nil {
		return nil, "", fmt.Errorf("transcode artifact resolver is unavailable")
	}
	outputDir, err := s.ResolveHLSOutputDir(media, quality)
	if err != nil {
		return nil, "", err
	}
	artifact, err := s.executionRepo.FindReadableHLSArtifactByPath(
		media.ID,
		quality,
		transcodeSourceFingerprint(media),
		transcodePlannerVersion,
		outputDir,
		time.Now(),
	)
	if err != nil {
		return nil, "", err
	}
	return artifact, outputDir, nil
}

// resolveHLSArtifactVersion resolves one explicit Artifact identity. Published
// and retained superseded versions are immutable and may continue serving old
// playlist clients. Active workspace versions still require a live Job Lease.
func (s *TranscodeService) resolveHLSArtifactVersion(media *model.Media, quality, artifactID string) (*model.TranscodeArtifactRecord, string, error) {
	if s == nil || s.executionRepo == nil || media == nil {
		return nil, "", fmt.Errorf("transcode artifact resolver is unavailable")
	}
	artifact, err := s.executionRepo.FindReadableHLSArtifactVersion(
		media.ID,
		quality,
		transcodeSourceFingerprint(media),
		transcodePlannerVersion,
		artifactID,
		time.Now(),
	)
	if err != nil {
		return nil, "", err
	}
	outputDir, err := readableHLSArtifactDirectory(artifact)
	if err != nil {
		return nil, "", err
	}
	return artifact, outputDir, nil
}

func readableHLSArtifactDirectory(artifact *model.TranscodeArtifactRecord) (string, error) {
	if artifact == nil {
		return "", fmt.Errorf("transcode artifact is missing")
	}
	switch artifact.Status {
	case "staging":
		if artifact.TempPath != "" {
			return artifact.TempPath, nil
		}
	case "publishing":
		// Publication first persists the immutable target, then atomically moves
		// the workspace, then commits the row. During that narrow interval either
		// path may be the currently readable one.
		if artifact.Path != "" {
			if _, err := os.Stat(artifact.Path); err == nil {
				return artifact.Path, nil
			}
		}
		if artifact.TempPath != "" {
			return artifact.TempPath, nil
		}
	case "published", "superseded":
		if artifact.Path != "" {
			return artifact.Path, nil
		}
	}
	return "", fmt.Errorf("artifact %s has no readable directory in status %s", artifact.ID, artifact.Status)
}
