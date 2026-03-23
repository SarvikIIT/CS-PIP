#ifndef CSPIP_CONTAINER_H
#define CSPIP_CONTAINER_H

#include "types.h"

/*
 * container.h — Container lifecycle management
 *
 * State transitions:
 *
 *   [created] ──run/start()──▶ [running] ──stop()──▶ [stopped]
 *                                        ──kill()──▶ [stopped]
 *   [stopped] ──remove()──▶ (deleted)
 */

/*
 * Create and immediately start a container.
 *
 * This is the primary entry point called by `cspip run`.  It:
 *   1. Generates a container ID.
 *   2. Creates the cgroup and applies resource limits.
 *   3. Writes state.json (status = created).
 *   4. clone()s a child process with PID/Mount/UTS/Net namespaces.
 *   5. Child: sets hostname, pivot_root, mounts /proc and /dev, execs
 *             the user command.
 *   6. Parent: adds child PID to cgroup, updates state to running,
 *              waits for the child to exit.
 *   7. Updates state to stopped after exit.
 *
 * Blocks until the container process exits.
 * Returns the container exit code, or -1 on error.
 */
int container_run(ContainerConfig *config);

/*
 * Run a command inside an already-running container (like `docker exec`).
 *
 * Enters the namespaces of the container's init PID, then exec()s cmd.
 * Returns -1 on error; on success this function does not return
 * (replaced by the exec'd process).
 */
int container_exec(const char *container_id,
                   char *const cmd[], int cmd_argc);

/*
 * Send SIGTERM to the container's init process and wait for it to exit.
 * Updates state to stopped.
 * Returns 0 on success, -1 on error.
 */
int container_stop(const char *container_id);

/*
 * Send SIGKILL to the container's init process immediately.
 * Returns 0 on success, -1 on error.
 */
int container_kill(const char *container_id);

/*
 * Remove a stopped container: delete cgroup, state directory.
 * Refuses to remove a running container.
 * Returns 0 on success, -1 on error.
 */
int container_remove(const char *container_id);

/*
 * Read the state of a container from disk.
 * Returns 0 on success, -1 if not found or parse error.
 */
int container_inspect(const char *container_id, ContainerState *out);

/*
 * Print a table of all containers to stdout (like `docker ps -a`).
 */
void container_list(void);

/*
 * Generate a short random hex container ID (CONTAINER_ID_LEN chars).
 * Writes into buf which must be at least CONTAINER_ID_LEN+1 bytes.
 */
void container_generate_id(char *buf);

#endif /* CSPIP_CONTAINER_H */
