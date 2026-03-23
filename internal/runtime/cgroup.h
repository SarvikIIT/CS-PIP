#ifndef CSPIP_CGROUP_H
#define CSPIP_CGROUP_H

#include <sys/types.h>

/*
 * cgroup.h — cgroup v2 resource management
 *
 * Each container gets its own directory under:
 *   /sys/fs/cgroup/cspip/<container-id>/
 *
 * Layout (cgroup v2 unified hierarchy):
 *   cgroup.procs   — write PID to join this cgroup
 *   cpu.max        — "quota period"  e.g. "50000 100000" = 50%
 *   memory.max     — hard memory limit in bytes
 *   memory.current — (read-only) live usage
 */

/*
 * Create the cgroup directory for a container.
 * Also enables the cpu and memory controllers in the parent cgroup.
 * Returns 0 on success, -1 on error.
 */
int cgroup_create(const char *container_id);

/*
 * Set CPU limit.
 * cpu_limit format: "50%" or "0.5" (fraction of one core).
 * Internally written as "<quota> 100000" into cpu.max.
 * Returns 0 on success, -1 on error.
 */
int cgroup_set_cpu(const char *container_id, const char *cpu_limit);

/*
 * Set memory limit.
 * mem_limit format: "256m", "1g", or a raw byte count.
 * Returns 0 on success, -1 on error.
 */
int cgroup_set_mem(const char *container_id, const char *mem_limit);

/*
 * Add a process to the container's cgroup.
 * Returns 0 on success, -1 on error.
 */
int cgroup_add_pid(const char *container_id, pid_t pid);

/*
 * Remove the cgroup directory and all its files.
 * Should be called during container removal.
 * Returns 0 on success, -1 on error.
 */
int cgroup_destroy(const char *container_id);

/*
 * Read current memory usage (bytes) from memory.current.
 * Returns the value, or -1 on error.
 */
long long cgroup_read_mem_current(const char *container_id);

#endif /* CSPIP_CGROUP_H */
