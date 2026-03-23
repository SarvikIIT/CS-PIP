#ifndef CSPIP_ROOTFS_H
#define CSPIP_ROOTFS_H

/*
 * rootfs.h — Container filesystem isolation
 *
 * Uses pivot_root(2) to make the container see only rootfs/ as its
 * entire filesystem.  The host filesystem becomes completely unreachable
 * after setup_rootfs() completes.
 */

/*
 * Isolate the container's filesystem by pivot_root-ing into rootfs_path.
 *
 * Steps performed:
 *   1. Bind-mount rootfs_path onto itself (required by pivot_root).
 *   2. Create rootfs_path/.old_root as the landing spot for the old root.
 *   3. Call pivot_root(rootfs_path, rootfs_path/.old_root).
 *   4. chdir("/") into the new root.
 *   5. Unmount .old_root with MNT_DETACH — host filesystem disappears.
 *   6. rmdir .old_root.
 *
 * Must be called inside the child process (new mount namespace) before
 * ns_mount_proc() and ns_mount_dev().
 *
 * Returns 0 on success, -1 on error.
 */
int rootfs_setup(const char *rootfs_path);

#endif /* CSPIP_ROOTFS_H */
