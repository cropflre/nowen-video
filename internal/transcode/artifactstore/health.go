package artifactstore

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const healthProbePayload = "nowen-artifact-store-health\n"

type HealthProbeResult struct {
	Root      string        `json:"root"`
	Writable  bool          `json:"writable"`
	Latency   time.Duration `json:"latency"`
	CheckedAt time.Time     `json:"checked_at"`
}

// ProbeWritable verifies the complete metadata path used by Artifact publish:
// directory lookup, file creation, write, fsync, atomic rename and removal.
// A successful stat or disk-usage sample alone is not sufficient for SMB/NFS
// mounts that have become read-only or disconnected after being mounted.
func (s *Store) ProbeWritable(now time.Time) (HealthProbeResult, error) {
	started := time.Now()
	result := HealthProbeResult{CheckedAt: now}
	if s == nil || s.root == "" {
		return result, fmt.Errorf("artifact store is unavailable")
	}
	result.Root = s.root
	info, err := os.Stat(s.root)
	if err != nil {
		return result, fmt.Errorf("stat artifact store root: %w", err)
	}
	if !info.IsDir() {
		return result, fmt.Errorf("artifact store root is not a directory: %s", s.root)
	}

	probeDir := filepath.Join(s.root, ".health")
	if err := os.MkdirAll(probeDir, 0o755); err != nil {
		return result, fmt.Errorf("create artifact store health namespace: %w", err)
	}
	file, err := os.CreateTemp(probeDir, ".probe-*")
	if err != nil {
		return result, fmt.Errorf("create artifact store health probe: %w", err)
	}
	tempPath := file.Name()
	committedPath := tempPath + ".committed"
	defer os.Remove(tempPath)
	defer os.Remove(committedPath)

	if _, err := file.WriteString(healthProbePayload); err != nil {
		file.Close()
		return result, fmt.Errorf("write artifact store health probe: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return result, fmt.Errorf("sync artifact store health probe: %w", err)
	}
	if err := file.Close(); err != nil {
		return result, fmt.Errorf("close artifact store health probe: %w", err)
	}
	if err := os.Rename(tempPath, committedPath); err != nil {
		return result, fmt.Errorf("rename artifact store health probe: %w", err)
	}
	if err := os.Remove(committedPath); err != nil {
		return result, fmt.Errorf("remove artifact store health probe: %w", err)
	}

	result.Writable = true
	result.Latency = time.Since(started)
	return result, nil
}
