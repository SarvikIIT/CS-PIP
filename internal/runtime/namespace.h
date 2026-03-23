#ifndef CSPIP_NAMESPACE_H
#define CSPIP_NAMESPACE_H

/*
 * namespace.h — Linux namespace setup
 *
 * These functions are called inside the child process after clone().
 * They configure the UTS, mount, and PID namespaces so the container
 * sees its own hostname, filesystem, and process table.
 */

/*
 * Set the container hostname (visible only inside the UTS namespace).
 * Returns 0 on success, -1 on error (errno set).
 */
int ns_set_hostname(const char *hostname);

/*
 * Mount a fresh /proc inside the container's PID namespace.
 * Must be called after setup_rootfs() so /proc refers to the new root.
 * Returns 0 on success, -1 on error.
 */
int ns_mount_proc(void);

/*
 * Mount a minimal /dev inside the container (null, zero, urandom, tty).
 * Returns 0 on success, -1 on error.
 */
int ns_mount_dev(void);

/*
 * Enter the namespaces of an already-running container identified by
 * its init PID.  Called by `cspip exec` to join an existing container.
 * Enters: mnt, pid, uts  (net if join_net is non-zero).
 * Returns 0 on success, -1 on error.
 */
int ns_enter(pid_t container_pid, int join_net);

#endif /* CSPIP_NAMESPACE_H */
