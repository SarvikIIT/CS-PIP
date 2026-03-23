#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <sys/mount.h>
#include <sys/stat.h>
#include <sys/syscall.h>

#include "rootfs.h"

/*
 * pivot_root is not always wrapped by glibc; call it via syscall(2).
 */
static int do_pivot_root(const char *new_root, const char *put_old)
{
    return (int)syscall(SYS_pivot_root, new_root, put_old);
}

int rootfs_setup(const char *rootfs_path)
{
    char abs_rootfs[512];
    char old_root[640];

    /* Resolve to an absolute path so pivot_root gets a clean path. */
    if (realpath(rootfs_path, abs_rootfs) == NULL) {
        perror("realpath rootfs");
        return -1;
    }

    /* ----------------------------------------------------------------
     * Step 1: Bind-mount rootfs onto itself.
     *
     * pivot_root requires that new_root is a mount point.  The easiest
     * way to guarantee this is a bind mount.  MS_REC also re-binds any
     * existing sub-mounts (e.g. if rootfs is itself a bind mount).
     * ---------------------------------------------------------------- */
    if (mount(abs_rootfs, abs_rootfs, NULL,
              MS_BIND | MS_REC, NULL) < 0) {
        perror("mount --bind rootfs");
        return -1;
    }

    /* ----------------------------------------------------------------
     * Step 2: Create the directory that will receive the old root.
     * ---------------------------------------------------------------- */
    snprintf(old_root, sizeof(old_root), "%s/.old_root", abs_rootfs);

    if (mkdir(old_root, 0700) < 0 && errno != EEXIST) {
        perror("mkdir .old_root");
        return -1;
    }

    /* ----------------------------------------------------------------
     * Step 3: Swap roots.
     * After this call:
     *   /           → abs_rootfs
     *   /.old_root  → former host root
     * ---------------------------------------------------------------- */
    if (do_pivot_root(abs_rootfs, old_root) < 0) {
        perror("pivot_root");
        return -1;
    }

    /* ----------------------------------------------------------------
     * Step 4: Move into the new root.
     * ---------------------------------------------------------------- */
    if (chdir("/") < 0) {
        perror("chdir /");
        return -1;
    }

    /* ----------------------------------------------------------------
     * Step 5: Unmount the old root.
     *
     * MNT_DETACH detaches it from the mount tree immediately even if
     * busy, and cleans up once the last file descriptor closes.
     * After this the entire host filesystem is unreachable.
     * ---------------------------------------------------------------- */
    if (umount2("/.old_root", MNT_DETACH) < 0) {
        perror("umount2 .old_root");
        return -1;
    }

    /* ----------------------------------------------------------------
     * Step 6: Remove the now-empty pivot directory.
     * ---------------------------------------------------------------- */
    if (rmdir("/.old_root") < 0) {
        /* Non-fatal: directory may still be busy on some kernels. */
        fprintf(stderr, "warn: rmdir /.old_root: %s\n", strerror(errno));
    }

    return 0;
}
