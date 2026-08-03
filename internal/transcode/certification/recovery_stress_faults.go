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
		return nil, fmt.Errorf("resolve sudo for ENOSPC tmpfs: %w", err)
	}
	mountPath, err := exec.LookPath("mount")
	if err != nil {
		return nil, fmt.Errorf("resolve mount for ENOSPC tmpfs: %w", err)
	}
	umountPath, err := exec.LookPath("umount")
	if err != nil {
		return nil, fmt.Errorf("resolve umount for ENOSPC tmpfs: %w", err)
	}
	options := fmt.Sprintf("size=%d,mode=0755,uid=%d,gid=%d", capacityBytes, os.Getuid(), os.Getgid())
	command := exec.Command(sudo, "-n", mountPath, "-t", "tmpfs", "-o", options, "nowen-recovery-enospc", workspace)
	combined, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mount ENOSPC tmpfs: %w: %s", err, strings.TrimSpace(string(combined)))
	}
	cleanup := func() error {
		command := exec.Command(sudo, "-n", umountPath, workspace)
		combined, err := command.CombinedOutput()
		if err != nil {
			return fmt.Errorf("unmount ENOSPC tmpfs: %w: %s", err, strings.TrimSpace(string(combined)))
		}
		return nil
	}
	prefillBytes, err := enospcPrefillBytes(capacityBytes)
	if err != nil {
		_ = cleanup()
		return nil, err
	}
	reservePath := filepath.Join(workspace, ".nowen-enospc-reserve")
	if err := os.WriteFile(reservePath, make([]byte, int(prefillBytes)), 0o600); err != nil {
		_ = cleanup()
		return nil, fmt.Errorf("prefill ENOSPC tmpfs: %w", err)
	}
	return cleanup, nil
}

func enospcPrefillBytes(capacityBytes int64) (int64, error) {
	const ffmpegHeadroom = int64(64 * 1024)
	if capacityBytes <= ffmpegHeadroom {
		return 0, fmt.Errorf("tmpfs capacity %d does not leave fault headroom", capacityBytes)
	}
	return capacityBytes - ffmpegHeadroom, nil
}

const resourceLimitHelperSource = `
#define _GNU_SOURCE
#include <errno.h>
#include <fcntl.h>
#include <grp.h>
#include <limits.h>
#include <sched.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>

static int write_value(const char *path, const char *value) {
    int fd = open(path, O_WRONLY | O_CLOEXEC);
    if (fd < 0) return -1;
    size_t length = strlen(value);
    ssize_t written = write(fd, value, length);
    int saved = errno;
    close(fd);
    errno = saved;
    return written == (ssize_t)length ? 0 : -1;
}

static long long read_peak(const char *path) {
    char buffer[128];
    int fd = open(path, O_RDONLY | O_CLOEXEC);
    if (fd < 0) return -1;
    ssize_t length = read(fd, buffer, sizeof(buffer) - 1);
    int saved = errno;
    close(fd);
    errno = saved;
    if (length <= 0) return -1;
    buffer[length] = '\0';
    return atoll(buffer);
}

static int write_peak_file(const char *path, long long peak, uid_t uid, gid_t gid) {
    int fd = open(path, O_WRONLY | O_CREAT | O_TRUNC | O_CLOEXEC, 0644);
    if (fd < 0) return -1;
    char buffer[128];
    int length = snprintf(buffer, sizeof(buffer), "%lld\n", peak);
    if (write(fd, buffer, (size_t)length) != length) {
        int saved = errno;
        close(fd);
        errno = saved;
        return -1;
    }
    if (fchown(fd, uid, gid) != 0) {
        int saved = errno;
        close(fd);
        errno = saved;
        return -1;
    }
    return close(fd);
}

int main(int argc, char **argv) {
    if (argc < 9) {
        fprintf(stderr, "usage: helper memory_bytes cpu_count cpu peak_path uid gid command [args...]\n");
        return 125;
    }
    long long memory_max = atoll(argv[1]);
    int cpu_count = atoi(argv[2]);
    int cpu = atoi(argv[3]);
    const char *peak_path = argv[4];
    uid_t uid = (uid_t)strtoul(argv[5], NULL, 10);
    gid_t gid = (gid_t)strtoul(argv[6], NULL, 10);
    if (memory_max <= 0 || cpu_count != 1 || cpu < 0) {
        fprintf(stderr, "invalid cgroup limits\n");
        return 125;
    }

    char cgroup[PATH_MAX];
    snprintf(cgroup, sizeof(cgroup), "/sys/fs/cgroup/nowen-recovery-%ld", (long)getpid());
    if (mkdir(cgroup, 0755) != 0) {
        perror("mkdir recovery cgroup");
        return 125;
    }

    char path[PATH_MAX];
    char value[128];
    snprintf(path, sizeof(path), "%s/memory.max", cgroup);
    snprintf(value, sizeof(value), "%lld", memory_max);
    if (write_value(path, value) != 0) {
        perror("write memory.max");
        rmdir(cgroup);
        return 125;
    }
    snprintf(path, sizeof(path), "%s/memory.swap.max", cgroup);
    if (access(path, W_OK) == 0 && write_value(path, "0") != 0) {
        perror("write memory.swap.max");
        rmdir(cgroup);
        return 125;
    }
    snprintf(path, sizeof(path), "%s/cpu.max", cgroup);
    if (write_value(path, "100000 100000") != 0) {
        perror("write cpu.max");
        rmdir(cgroup);
        return 125;
    }

    pid_t child = fork();
    if (child < 0) {
        perror("fork");
        rmdir(cgroup);
        return 125;
    }
    if (child == 0) {
        char pid_value[64];
        snprintf(path, sizeof(path), "%s/cgroup.procs", cgroup);
        snprintf(pid_value, sizeof(pid_value), "%ld", (long)getpid());
        if (write_value(path, pid_value) != 0) {
            perror("join recovery cgroup");
            _exit(125);
        }
        cpu_set_t set;
        CPU_ZERO(&set);
        CPU_SET(cpu, &set);
        if (sched_setaffinity(0, sizeof(set), &set) != 0) {
            perror("set CPU affinity");
            _exit(125);
        }
        if (setgroups(0, NULL) != 0 || setgid(gid) != 0 || setuid(uid) != 0) {
            perror("drop helper privileges");
            _exit(125);
        }
        execv(argv[7], &argv[7]);
        perror("exec bounded command");
        _exit(127);
    }

    int status = 0;
    if (waitpid(child, &status, 0) < 0) {
        perror("waitpid");
        status = 125 << 8;
    }
    snprintf(path, sizeof(path), "%s/memory.peak", cgroup);
    long long peak = read_peak(path);
    if (peak <= 0 || write_peak_file(peak_path, peak, uid, gid) != 0) {
        perror("record memory.peak");
        if (WIFEXITED(status) && WEXITSTATUS(status) == 0) status = 125 << 8;
    }
    if (rmdir(cgroup) != 0) perror("remove recovery cgroup");

    if (WIFEXITED(status)) return WEXITSTATUS(status);
    if (WIFSIGNALED(status)) return 128 + WTERMSIG(status);
    return 125;
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
