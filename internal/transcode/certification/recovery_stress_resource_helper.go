package certification

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
