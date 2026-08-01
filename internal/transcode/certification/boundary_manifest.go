package certification

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func readBoundaryManifestSegments(manifestPath string) ([]string, error) {
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("open boundary manifest: %w", err)
	}
	defer file.Close()
	segments := make([]string, 0, 16)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parsed, err := url.Parse(line)
		if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return nil, fmt.Errorf("boundary manifest contains unsupported segment URI %q", line)
		}
		name := filepath.ToSlash(parsed.Path)
		if name == "" || strings.Contains(name, "/") || name != filepath.Base(name) || strings.ToLower(filepath.Ext(name)) != ".ts" {
			return nil, fmt.Errorf("boundary manifest segment URI is unsafe: %q", line)
		}
		segments = append(segments, name)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read boundary manifest: %w", err)
	}
	return segments, nil
}
