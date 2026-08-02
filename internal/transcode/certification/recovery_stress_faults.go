package certification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
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
		if err == nil && !info.IsDir() && info.Size() > 0 {
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

func boundedCommand(ffmpegPath string, args []string, limits transcoderecovery.ResourceLimits) (string, []string, error) {
	prlimit, err := exec.LookPath("prlimit")
	if err != nil {
		return "", nil, fmt.Errorf("resolve prlimit: %w", err)
	}
	taskset, err := exec.LookPath("taskset")
	if err != nil {
		return "", nil, fmt.Errorf("resolve taskset: %w", err)
	}
	cpu, err := firstAllowedCPU()
	if err != nil {
		return "", nil, err
	}
	commandArgs := []string{"--as=" + strconv.FormatInt(limits.AddressSpaceBytes, 10), "--", taskset, "-c", cpu, ffmpegPath}
	commandArgs = append(commandArgs, args...)
	return prlimit, commandArgs, nil
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

func buildENOSPCShim(workDir string) (string, error) {
	cc, err := exec.LookPath("cc")
	if err != nil {
		return "", fmt.Errorf("resolve C compiler for ENOSPC shim: %w", err)
	}
	source := filepath.Join(workDir, "enospc_write_shim.c")
	output := filepath.Join(workDir, "enospc_write_shim.so")
	if err := os.WriteFile(source, []byte(enospcShimSource), 0o644); err != nil {
		return "", err
	}
	command := exec.Command(cc, "-shared", "-fPIC", "-O2", "-std=c11", "-o", output, source, "-ldl")
	combined, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("build ENOSPC shim: %w: %s", err, strings.TrimSpace(string(combined)))
	}
	return output, nil
}

const enospcShimSource = `
#define _GNU_SOURCE
#include <dlfcn.h>
#include <errno.h>
#include <limits.h>
#include <stdatomic.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/types.h>
#include <sys/uio.h>
#include <unistd.h>

static ssize_t (*real_write_fn)(int, const void *, size_t);
static ssize_t (*real_writev_fn)(int, const struct iovec *, int);
static ssize_t (*real_pwrite_fn)(int, const void *, size_t, off_t);
static ssize_t (*real_pwritev_fn)(int, const struct iovec *, int, off_t);
static _Atomic long long written_bytes = 0;
static long long limit_bytes = -1;
static char target_prefix[PATH_MAX];

__attribute__((constructor)) static void init_shim(void) {
    real_write_fn = dlsym(RTLD_NEXT, "write");
    real_writev_fn = dlsym(RTLD_NEXT, "writev");
    real_pwrite_fn = dlsym(RTLD_NEXT, "pwrite");
    real_pwritev_fn = dlsym(RTLD_NEXT, "pwritev");
    const char *limit = getenv("NOWEN_ENOSPC_AFTER_BYTES");
    const char *prefix = getenv("NOWEN_ENOSPC_PATH");
    if (limit) limit_bytes = atoll(limit);
    if (prefix) snprintf(target_prefix, sizeof(target_prefix), "%s", prefix);
}

static int target_fd(int fd) {
    if (limit_bytes < 0 || target_prefix[0] == '\0') return 0;
    char link_path[64];
    char resolved[PATH_MAX];
    snprintf(link_path, sizeof(link_path), "/proc/self/fd/%d", fd);
    ssize_t length = readlink(link_path, resolved, sizeof(resolved) - 1);
    if (length <= 0) return 0;
    resolved[length] = '\0';
    return strncmp(resolved, target_prefix, strlen(target_prefix)) == 0;
}

static int exhaust(int fd, size_t count) {
    if (!target_fd(fd)) return 0;
    long long before = atomic_fetch_add(&written_bytes, (long long)count);
    if (before + (long long)count <= limit_bytes) return 0;
    errno = ENOSPC;
    return 1;
}

ssize_t write(int fd, const void *buffer, size_t count) {
    if (!real_write_fn) real_write_fn = dlsym(RTLD_NEXT, "write");
    if (exhaust(fd, count)) return -1;
    return real_write_fn(fd, buffer, count);
}

ssize_t writev(int fd, const struct iovec *iov, int iovcnt) {
    if (!real_writev_fn) real_writev_fn = dlsym(RTLD_NEXT, "writev");
    size_t count = 0;
    for (int i = 0; i < iovcnt; ++i) count += iov[i].iov_len;
    if (exhaust(fd, count)) return -1;
    return real_writev_fn(fd, iov, iovcnt);
}

ssize_t pwrite(int fd, const void *buffer, size_t count, off_t offset) {
    if (!real_pwrite_fn) real_pwrite_fn = dlsym(RTLD_NEXT, "pwrite");
    if (exhaust(fd, count)) return -1;
    return real_pwrite_fn(fd, buffer, count, offset);
}

ssize_t pwritev(int fd, const struct iovec *iov, int iovcnt, off_t offset) {
    if (!real_pwritev_fn) real_pwritev_fn = dlsym(RTLD_NEXT, "pwritev");
    size_t count = 0;
    for (int i = 0; i < iovcnt; ++i) count += iov[i].iov_len;
    if (exhaust(fd, count)) return -1;
    return real_pwritev_fn(fd, iov, iovcnt, offset);
}
`

func slicesContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
