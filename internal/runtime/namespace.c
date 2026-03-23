#define _GNU_SOURCE
#include <stdio.h>
#include <string.h>
#include <errno.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/mount.h>
#include <sys/stat.h>
#include <sys/syscall.h>

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

    /* Bind-mount host device nodes the container needs. */
    static const char *devnodes[] = {
        "/dev/null", "/dev/zero", "/dev/urandom",
        "/dev/random", "/dev/tty", NULL
    };

    for (int i = 0; devnodes[i]; i++) {
        /* Create an empty file as the bind-mount target. */
        int fd = open(devnodes[i] + 1,   /* relative under new /dev */
                      O_CREAT | O_WRONLY, 0666);
        if (fd >= 0) close(fd);

        char dest[64];
        snprintf(dest, sizeof(dest), "/dev/%s",
                 strrchr(devnodes[i], '/') + 1);

        if (mount(devnodes[i], dest, NULL, MS_BIND, NULL) < 0) {
            /* Non-fatal: some nodes may not exist on the host. */
            fprintf(stderr, "warn: bind-mount %s: %s\n",
                    devnodes[i], strerror(errno));
        }
    }
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
