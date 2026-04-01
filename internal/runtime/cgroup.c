#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <fcntl.h>
#include <unistd.h>
#include <dirent.h>
#include <sys/stat.h>

#include "cgroup.h"

/* ------------------------------------------------------------------ */
/* Internal helpers                                                     */
/* ------------------------------------------------------------------ */

/*
 * Build the path to a file inside this container's cgroup directory.
 */
static void cgroup_path(char *buf, size_t size,
                        const char *container_id, const char *file)
{
    snprintf(buf, size, "/sys/fs/cgroup/cspip/%s/%s",
             container_id, file);
}

/*
 * Write a NUL-terminated string to a file, overwriting its contents.
 * Returns 0 on success, -1 on error.
 */
static int write_file(const char *path, const char *value)
{
    int fd = open(path, O_WRONLY | O_TRUNC);
    if (fd < 0) {
        fprintf(stderr, "write_file open %s: %s\n", path, strerror(errno));
        return -1;
    }
    ssize_t len = (ssize_t)strlen(value);
    if (write(fd, value, (size_t)len) != len) {
        fprintf(stderr, "write_file write %s: %s\n", path, strerror(errno));
        close(fd);
        return -1;
    }
    close(fd);
    return 0;
}

/*
 * Read a file into buf (up to buf_size-1 bytes) and NUL-terminate.
 * Returns number of bytes read, or -1 on error.
 */
static ssize_t read_file(const char *path, char *buf, size_t buf_size)
{
    int fd = open(path, O_RDONLY);
    if (fd < 0) {
        fprintf(stderr, "read_file open %s: %s\n", path, strerror(errno));
        return -1;
    }
    ssize_t n = read(fd, buf, buf_size - 1);
    close(fd);
    if (n < 0) return -1;
    buf[n] = '\0';
    return n;
}

/* ------------------------------------------------------------------ */
/* Parse limit strings                                                  */
/* ------------------------------------------------------------------ */

/*
 * Parse "50%" → quota=50000, period=100000.
 * The kernel interprets cpu.max as "<quota_us> <period_us>".
 * 50% of one core = 50 000 µs of CPU time per 100 000 µs period.
 */
static int parse_cpu_limit(const char *limit, long *quota, long *period)
{
    *period = 100000; /* fixed 100 ms period */

    char tmp[32];
    strncpy(tmp, limit, sizeof(tmp) - 1);
    tmp[sizeof(tmp) - 1] = '\0';

    char *pct = strchr(tmp, '%');
    if (pct) {
        *pct = '\0';
        double pct_val = atof(tmp);
        if (pct_val <= 0 || pct_val > 100000) {
            fprintf(stderr, "invalid cpu limit: %s\n", limit);
            return -1;
        }
        *quota = (long)(pct_val / 100.0 * (*period));
        return 0;
    }

    /* Fractional core, e.g. "0.5" */
    double frac = atof(tmp);
    if (frac <= 0) {
        fprintf(stderr, "invalid cpu limit: %s\n", limit);
        return -1;
    }
    *quota = (long)(frac * (*period));
    return 0;
}

/*
 * Parse "256m" / "1g" / "536870912" → bytes.
 */
static long long parse_mem_limit(const char *limit)
{
    char tmp[32];
    strncpy(tmp, limit, sizeof(tmp) - 1);
    tmp[sizeof(tmp) - 1] = '\0';

    size_t len = strlen(tmp);
    long long multiplier = 1;

    if (len == 0) return -1;

    char last = tmp[len - 1];
    if (last == 'm' || last == 'M') {
        multiplier = 1024LL * 1024;
        tmp[len - 1] = '\0';
    } else if (last == 'g' || last == 'G') {
        multiplier = 1024LL * 1024 * 1024;
        tmp[len - 1] = '\0';
    } else if (last == 'k' || last == 'K') {
        multiplier = 1024LL;
        tmp[len - 1] = '\0';
    }

    long long base = atoll(tmp);
    if (base <= 0) {
        fprintf(stderr, "invalid mem limit: %s\n", limit);
        return -1;
    }
    return base * multiplier;
}

/* ------------------------------------------------------------------ */
/* Enable a controller in the parent cgroup's subtree_control           */
/* ------------------------------------------------------------------ */

static void enable_controllers(void)
{
    /*
     * Enable controllers top-down: the root cgroup must expose a controller
     * in its subtree_control before a child cgroup can use it.
     * Both writes are best-effort — some systems (systemd-managed) may
     * restrict direct writes to the root cgroup.
     */
    write_file("/sys/fs/cgroup/cgroup.subtree_control", "+cpu +memory +io");
    write_file("/sys/fs/cgroup/cspip/cgroup.subtree_control", "+cpu +memory +io");
}

/* ------------------------------------------------------------------ */
/* Public API                                                           */
/* ------------------------------------------------------------------ */

int cgroup_create(const char *container_id)
{
    char dir[512];
    snprintf(dir, sizeof(dir), "/sys/fs/cgroup/cspip/%s", container_id);

    /* Create the top-level cspip directory if it doesn't exist yet. */
    if (mkdir("/sys/fs/cgroup/cspip", 0755) < 0 && errno != EEXIST) {
        perror("mkdir /sys/fs/cgroup/cspip");
        return -1;
    }

    enable_controllers();

    if (mkdir(dir, 0755) < 0 && errno != EEXIST) {
        fprintf(stderr, "mkdir %s: %s\n", dir, strerror(errno));
        return -1;
    }
    return 0;
}

int cgroup_set_cpu(const char *container_id, const char *cpu_limit)
{
    if (!cpu_limit || cpu_limit[0] == '\0')
        return 0; /* no limit */

    long quota, period;
    if (parse_cpu_limit(cpu_limit, &quota, &period) < 0)
        return -1;

    char value[64];
    snprintf(value, sizeof(value), "%ld %ld", quota, period);

    char path[512];
    cgroup_path(path, sizeof(path), container_id, "cpu.max");
    return write_file(path, value);
}

int cgroup_set_mem(const char *container_id, const char *mem_limit)
{
    if (!mem_limit || mem_limit[0] == '\0')
        return 0; /* no limit */

    long long bytes = parse_mem_limit(mem_limit);
    if (bytes < 0)
        return -1;

    char value[32];
    snprintf(value, sizeof(value), "%lld", bytes);

    char path[512];
    cgroup_path(path, sizeof(path), container_id, "memory.max");
    return write_file(path, value);
}

int cgroup_add_pid(const char *container_id, pid_t pid)
{
    char path[512];
    cgroup_path(path, sizeof(path), container_id, "cgroup.procs");

    char pid_str[32];
    snprintf(pid_str, sizeof(pid_str), "%d", pid);
    return write_file(path, pid_str);
}

int cgroup_destroy(const char *container_id)
{
    char dir[512];
    snprintf(dir, sizeof(dir), "/sys/fs/cgroup/cspip/%s", container_id);

    /* The kernel refuses to remove a cgroup that still has processes.
     * We move any remaining PIDs to the parent before removing. */
    char procs_path[600];
    snprintf(procs_path, sizeof(procs_path), "%s/cgroup.procs", dir);

    /* Read remaining PIDs and move them to parent. */
    char buf[4096];
    if (read_file(procs_path, buf, sizeof(buf)) > 0) {
        char *line = strtok(buf, "\n");
        while (line) {
            write_file("/sys/fs/cgroup/cspip/cgroup.procs", line);
            line = strtok(NULL, "\n");
        }
    }

    if (rmdir(dir) < 0) {
        fprintf(stderr, "rmdir %s: %s\n", dir, strerror(errno));
        return -1;
    }
    return 0;
}

long long cgroup_read_mem_current(const char *container_id)
{
    char path[512];
    cgroup_path(path, sizeof(path), container_id, "memory.current");

    char buf[32];
    if (read_file(path, buf, sizeof(buf)) < 0)
        return -1;
    return atoll(buf);
}
