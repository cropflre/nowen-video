package artifactstore

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const manifestName = "stream.m3u8"

type Store struct {
	root string
}

type Validation struct {
	ManifestPath string
	SizeBytes    int64
	SegmentCount int
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("artifact store root is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve artifact store root: %w", err)
	}
	absolute = filepath.Clean(absolute)
	for _, path := range []string{
		absolute,
		filepath.Join(absolute, "workspaces"),
		filepath.Join(absolute, "artifacts"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return nil, fmt.Errorf("create artifact store namespace %s: %w", path, err)
		}
	}
	return &Store{root: absolute}, nil
}

func (s *Store) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

func (s *Store) WorkspaceDir(jobID, attemptID string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("artifact store is unavailable")
	}
	if err := validateComponent("job id", jobID); err != nil {
		return "", err
	}
	if err := validateComponent("attempt id", attemptID); err != nil {
		return "", err
	}
	return s.safeJoin("workspaces", jobID, attemptID, "hls")
}

func (s *Store) PublishedDir(mediaID, profileID, artifactID string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("artifact store is unavailable")
	}
	if err := validateComponent("media id", mediaID); err != nil {
		return "", err
	}
	if err := validateComponent("profile id", profileID); err != nil {
		return "", err
	}
	if err := validateComponent("artifact id", artifactID); err != nil {
		return "", err
	}
	return s.safeJoin("artifacts", mediaID, profileID, artifactID)
}

func (s *Store) PrepareWorkspace(jobID, attemptID string) (string, error) {
	workspace, err := s.WorkspaceDir(jobID, attemptID)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(workspace); err != nil {
		return "", fmt.Errorf("reset attempt workspace: %w", err)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return "", fmt.Errorf("create attempt workspace: %w", err)
	}
	return workspace, nil
}

func (s *Store) ManifestPath(dir string) (string, error) {
	if err := s.ensureInside(dir); err != nil {
		return "", err
	}
	return filepath.Join(dir, manifestName), nil
}

func (s *Store) ValidateHLS(dir string) (Validation, error) {
	if err := s.ensureInside(dir); err != nil {
		return Validation{}, err
	}
	manifestPath := filepath.Join(dir, manifestName)
	manifest, err := os.Open(manifestPath)
	if err != nil {
		return Validation{}, fmt.Errorf("open hls manifest: %w", err)
	}
	defer manifest.Close()

	validation := Validation{ManifestPath: manifestPath}
	scanner := bufio.NewScanner(manifest)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "://") || filepath.IsAbs(line) {
			return Validation{}, fmt.Errorf("manifest contains external segment path: %s", line)
		}
		clean := filepath.Clean(filepath.FromSlash(line))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return Validation{}, fmt.Errorf("manifest segment escapes workspace: %s", line)
		}
		segmentPath := filepath.Join(dir, clean)
		if err := s.ensureInside(segmentPath); err != nil {
			return Validation{}, err
		}
		info, statErr := os.Stat(segmentPath)
		if statErr != nil {
			return Validation{}, fmt.Errorf("manifest segment is missing: %s: %w", line, statErr)
		}
		if info.IsDir() || info.Size() <= 0 {
			return Validation{}, fmt.Errorf("manifest segment is empty: %s", line)
		}
		validation.SegmentCount++
	}
	if err := scanner.Err(); err != nil {
		return Validation{}, fmt.Errorf("read hls manifest: %w", err)
	}
	if validation.SegmentCount == 0 {
		return Validation{}, errors.New("hls manifest contains no completed segments")
	}

	walkErr := filepath.Walk(dir, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			validation.SizeBytes += info.Size()
		}
		return nil
	})
	if walkErr != nil {
		return Validation{}, fmt.Errorf("measure artifact workspace: %w", walkErr)
	}
	return validation, nil
}

// Publish atomically renames one completed Attempt workspace into an immutable
// artifact version. The workspace and target must be under the same Store root;
// cross-filesystem publication is rejected rather than copied non-atomically.
func (s *Store) Publish(workspace, target string) error {
	if err := s.ensureInside(workspace); err != nil {
		return err
	}
	if err := s.ensureInside(target); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("published artifact target already exists: %s", target)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect artifact target: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create artifact parent: %w", err)
	}
	if err := os.Rename(workspace, target); err != nil {
		return fmt.Errorf("atomically publish artifact: %w", err)
	}
	return nil
}

func (s *Store) Remove(path string) error {
	if err := s.ensureInside(path); err != nil {
		return err
	}
	return os.RemoveAll(path)
}

func (s *Store) safeJoin(parts ...string) (string, error) {
	joined := filepath.Join(append([]string{s.root}, parts...)...)
	if err := s.ensureInside(joined); err != nil {
		return "", err
	}
	return joined, nil
}

func (s *Store) ensureInside(path string) error {
	if s == nil || s.root == "" {
		return fmt.Errorf("artifact store is unavailable")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve artifact path: %w", err)
	}
	relative, err := filepath.Rel(s.root, filepath.Clean(absolute))
	if err != nil {
		return fmt.Errorf("compare artifact path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("artifact path escapes store root: %s", path)
	}
	return nil
}

func validateComponent(name, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if value == "." || value == ".." || strings.ContainsAny(value, `/\\`) {
		return fmt.Errorf("invalid %s: %q", name, value)
	}
	return nil
}
