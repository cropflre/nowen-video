package certification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	transcoderecovery "github.com/nowen-video/nowen-video/internal/transcode/recoverystress"
)

func inspectPartialHLS(workspace string) (int, bool) {
	manifest := filepath.Join(workspace, "stream.m3u8")
	_, manifestErr := os.Stat(manifest)
	matches, _ := filepath.Glob(filepath.Join(workspace, "seg*.ts"))
	segments := 0
	for _, path := range matches {
		info, err := os.Stat(path)
		if err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			segments++
		}
	}
	return segments, manifestErr == nil
}

func stderrMarkers(text string) []string {
	lower := strings.ToLower(text)
	markers := make([]string, 0, 3)
	if strings.Contains(lower, "no space left on device") || strings.Contains(lower, "enospc") {
		markers = append(markers, "ENOSPC")
	}
	if strings.Contains(lower, "killed") {
		markers = append(markers, "KILLED")
	}
	if strings.Contains(lower, "cancel") {
		markers = append(markers, "CANCELLED")
	}
	return markers
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sha256Text(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func recoveryCommandHash(path string, args, env []string, workDir, sourcePath string) string {
	normalize := func(value string) string {
		value = strings.ReplaceAll(value, workDir, "$WORKDIR")
		value = strings.ReplaceAll(value, sourcePath, "$SOURCE")
		return value
	}
	canonical := struct {
		Path string   `json:"path"`
		Args []string `json:"args"`
		Env  []string `json:"env"`
	}{Path: normalize(path), Args: append([]string(nil), args...), Env: append([]string(nil), env...)}
	for index := range canonical.Args {
		canonical.Args[index] = normalize(canonical.Args[index])
	}
	for index := range canonical.Env {
		canonical.Env[index] = normalize(canonical.Env[index])
	}
	content, _ := json.Marshal(canonical)
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func setAtomicMaximum(target *atomic.Int64, value int64) {
	for {
		current := target.Load()
		if value <= current || target.CompareAndSwap(current, value) {
			return
		}
	}
}

func monitorRSS(pid int, done <-chan struct{}, maximum *atomic.Int64) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			sampleRSS(pid, maximum)
			return
		case <-ticker.C:
			sampleRSS(pid, maximum)
		}
	}
}

func sampleRSS(pid int, maximum *atomic.Int64) {
	content, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return
		}
		kilobytes, err := strconv.ParseInt(fields[1], 10, 64)
		if err == nil {
			setAtomicMaximum(maximum, kilobytes*1024)
		}
		return
	}
}

func boundedCommand(workDir, ffmpegPath string, args []string, limits transcoderecovery.ResourceLimits) (string, []string, string, error) {
	if limits.CPUCount != 1 || limits.MemoryMaxBytes <= 0 {
		return "", nil, "", fmt.Errorf("unsupported bounded resource limits: cpu=%d memory=%d", limits.CPUCount, limits.MemoryMaxBytes)
	}
	sudo, err := exec.LookPath("sudo")
	if err != nil {
		return "", nil, "", fmt.Errorf("resolve sudo for cgroup helper: %w", err)
	}
	helper, err := buildResourceHelper(workDir)
	if err != nil {
		return "", nil, "", err
	}
	cpu, err := firstAllowedCPU()
	if err != nil {
		return "", nil, "", err
	}
	peakPath := filepath.Join(workDir, "bounded-memory-peak.txt")
	commandArgs := []string{
		"-n",
		helper,
		strconv.FormatInt(limits.MemoryMaxBytes, 10),
		strconv.Itoa(limits.CPUCount),
		cpu,
		peakPath,
		strconv.Itoa(os.Getuid()),
		strconv.Itoa(os.Getgid()),
		ffmpegPath,
	}
	commandArgs = append(commandArgs, args...)
	return sudo, commandArgs, peakPath, nil
}

func firstAllowedCPU() (string, error) {
	content, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(line, "Cpus_allowed_list:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "Cpus_allowed_list:"))
		if value == "" {
			break
		}
		first := strings.Split(value, ",")[0]
		return strings.Split(first, "-")[0], nil
	}
	return "", fmt.Errorf("could not determine allowed CPU")
}

func readMemoryPeak(path string) (int64, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid cgroup memory peak %q: %w", strings.TrimSpace(string(content)), err)
	}
	return value, nil
}

func buildResourceHelper(workDir string) (string, error) {
	cc, err := exec.LookPath("cc")
	if err != nil {
		return "", fmt.Errorf("resolve C compiler for cgroup helper: %w", err)
	}
	source := filepath.Join(workDir, "resource_limit_helper.c")
	output := filepath.Join(workDir, "resource_limit_helper")
	if err := os.WriteFile(source, []byte(resourceLimitHelperSource), 0o644); err != nil {
		return "", err
	}
	command := exec.Command(cc, "-O2", "-std=c11", "-Wall", "-Wextra", "-o", output, source)
	combined, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("build cgroup helper: %w: %s", err, strings.TrimSpace(string(combined)))
	}
	return output, nil
}

func mountENOSPCWorkspace(workspace string, capacityBytes int64) (func() error, error) {
	if capacityBytes <= 0 {
		return nil, fmt.Errorf("invalid tmpfs capacity %d", capacityBytes)
	}
	sudo, err := exec.LookPath("sudo")
	if err != nil {
		return nil, fmt.Errorf("resolve sudo for ENOSPC mount: %w", err)
	}
	mountPath, err := exec.LookPath("mount")
	if err != nil {
		return nil, fmt.Errorf("resolve mount for ENOSPC backend: %w", err)
	}
	umountPath, err := exec.LookPath("umount")
	if err != nil {
		return nil, fmt.Errorf("resolve umount for ENOSPC backend: %w", err)
	}

	options := fmt.Sprintf("size=%d,mode=0755,uid=%d,gid=%d", capacityBytes, os.Getuid(), os.Getgid())
	command := exec.Command(sudo, "-n", mountPath, "-t", "tmpfs", "-o", options, "nowen-recovery-enospc", workspace)
	combined, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mount ENOSPC tmpfs: %w: %s", err, strings.TrimSpace(string(combined)))
	}

	segmentPath := filepath.Join(workspace, "seg0000.ts")
	if err := os.WriteFile(segmentPath, nil, 0o644); err != nil {
		_ = exec.Command(sudo, "-n", umountPath, workspace).Run()
		return nil, fmt.Errorf("precreate ENOSPC segment target: %w", err)
	}
	command = exec.Command(sudo, "-n", mountPath, "--bind", "/dev/full", segmentPath)
	combined, err = command.CombinedOutput()
	if err != nil {
		_ = exec.Command(sudo, "-n", umountPath, workspace).Run()
		return nil, fmt.Errorf("bind /dev/full over first HLS segment: %w: %s", err, strings.TrimSpace(string(combined)))
	}
	if err := verifyENOSPCPath(segmentPath); err != nil {
		_ = exec.Command(sudo, "-n", umountPath, segmentPath).Run()
		_ = exec.Command(sudo, "-n", umountPath, workspace).Run()
		return nil, err
	}

	cleanup := func() error {
		var cleanupErr error
		if combined, err := exec.Command(sudo, "-n", umountPath, segmentPath).CombinedOutput(); err != nil {
			cleanupErr = fmt.Errorf("unmount ENOSPC segment bind: %w: %s", err, strings.TrimSpace(string(combined)))
		}
		if combined, err := exec.Command(sudo, "-n", umountPath, workspace).CombinedOutput(); err != nil {
			workspaceErr := fmt.Errorf("unmount ENOSPC tmpfs: %w: %s", err, strings.TrimSpace(string(combined)))
			if cleanupErr != nil {
				cleanupErr = errors.Join(cleanupErr, workspaceErr)
			} else {
				cleanupErr = workspaceErr
			}
		}
		return cleanupErr
	}
	return cleanup, nil
}

func verifyENOSPCPath(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open ENOSPC segment target: %w", err)
	}
	_, writeErr := file.Write([]byte{0})
	closeErr := file.Close()
	if !errors.Is(writeErr, syscall.ENOSPC) {
		return fmt.Errorf("segment target write returned %v, want ENOSPC", writeErr)
	}
	if closeErr != nil && !errors.Is(closeErr, syscall.ENOSPC) {
		return fmt.Errorf("close ENOSPC segment target: %w", closeErr)
	}
	return nil
}

func slicesContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
