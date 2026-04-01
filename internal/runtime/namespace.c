#define _GNU_SOURCE
#include <stdio.h>
#include <string.h>
#include <errno.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/mount.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/sysmacros.h>

#include "namespace.h"

/* ------------------------------------------------------------------ */
/* Hostname                                                             */
/* ------------------------------------------------------------------ */

int ns_set_hostname(const char *hostname)
{
    if (sethostname(hostname, strlen(hostname)) < 0) {
        perror("sethostname");
        return -1;
    }
    return 0;
}

/* ------------------------------------------------------------------ */
/* /proc                                                                */
/* ------------------------------------------------------------------ */

int ns_mount_proc(void)
{
    /* Detach any existing /proc first so we don't see host processes. */
    umount2("/proc", MNT_DETACH); /* ignore errors — may not be mounted */

    if (mkdir("/proc", 0555) < 0 && errno != EEXIST) {
        perror("mkdir /proc");
        return -1;
    }

    if (mount("proc", "/proc", "proc",
              MS_NOSUID | MS_NOEXEC | MS_NODEV, NULL) < 0) {
        perror("mount /proc");
        return -1;
    }
    return 0;
}

/* ------------------------------------------------------------------ */
/* /dev (minimal)                                                       */
/* ------------------------------------------------------------------ */

int ns_mount_dev(void)
{
    if (mkdir("/dev", 0755) < 0 && errno != EEXIST) {
        perror("mkdir /dev");
        return -1;
    }

    /* Mount a tmpfs so we can create device nodes safely. */
    if (mount("tmpfs", "/dev", "tmpfs",
              MS_NOSUID | MS_STRICTATIME,
              "mode=755,size=65536k") < 0) {
        perror("mount /dev tmpfs");
        return -1;
    }

    /*
     * Create the device nodes directly with mknod(2).
     *
     * We cannot bind-mount host device paths here because this function
     * runs after pivot_root has swapped the root and the old host
     * filesystem has been unmounted — host paths like /dev/null are
     * no longer accessible.  mknod creates the nodes directly inside
     * the container's /dev tmpfs.
     */
    static const struct {
        const char *path;
        mode_t      mode;          /* S_IFCHR | permissions */
        unsigned int major_num;
        unsigned int minor_num;
    } devs[] = {
        { "/dev/null",    S_IFCHR | 0666, 1, 3 },
        { "/dev/zero",    S_IFCHR | 0666, 1, 5 },
        { "/dev/full",    S_IFCHR | 0666, 1, 7 },
        { "/dev/random",  S_IFCHR | 0444, 1, 8 },
        { "/dev/urandom", S_IFCHR | 0444, 1, 9 },
        { "/dev/tty",     S_IFCHR | 0666, 5, 0 },
        { NULL, 0, 0, 0 },
    };

    for (int i = 0; devs[i].path; i++) {
        if (mknod(devs[i].path, devs[i].mode,
                  makedev(devs[i].major_num, devs[i].minor_num)) < 0
            && errno != EEXIST) {
            fprintf(stderr, "warn: mknod %s: %s\n",
                    devs[i].path, strerror(errno));
        }
    }

    /* /dev/stdin, /dev/stdout, /dev/stderr as symlinks. */
    (void)symlink("/proc/self/fd/0", "/dev/stdin");
    (void)symlink("/proc/self/fd/1", "/dev/stdout");
    (void)symlink("/proc/self/fd/2", "/dev/stderr");

    return 0;
}

/* ------------------------------------------------------------------ */
/* Enter existing namespaces (for `exec`)                               */
/* ------------------------------------------------------------------ */

/*
 * setns(2) wrapper — not always exposed by older glibc headers.
 */
static int do_setns(int fd, int nstype)
{
    return (int)syscall(SYS_setns, fd, nstype);
}

int ns_enter(pid_t container_pid, int join_net)
{
    /* Namespace types to enter, in the correct order:
     *   user  (if needed — skipped here, we run as root)
     *   mnt   — must come before pid so /proc reflects new ns
     *   uts
     *   pid
     *   net   (optional)
     */
    struct {
        const char *name;
        int         flag;   /* CLONE_NEW* constant, 0 = always enter */
        int         skip;
    } ns_list[] = {
        { "mnt", 0,              0        },
        { "uts", 0,              0        },
        { "pid", 0,              0        },
        { "net", 0,              !join_net},
        { NULL,  0,              0        },
    };

    for (int i = 0; ns_list[i].name; i++) {
        if (ns_list[i].skip)
            continue;

        char path[64];
        snprintf(path, sizeof(path), "/proc/%d/ns/%s",
                 container_pid, ns_list[i].name);

        int fd = open(path, O_RDONLY | O_CLOEXEC);
        if (fd < 0) {
            fprintf(stderr, "open %s: %s\n", path, strerror(errno));
            return -1;
        }

        if (do_setns(fd, ns_list[i].flag) < 0) {
            fprintf(stderr, "setns %s: %s\n", path, strerror(errno));
            close(fd);
            return -1;
        }
        close(fd);
    }
    return 0;
}
