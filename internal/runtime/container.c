#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <fcntl.h>
#include <unistd.h>
#include <signal.h>
#include <time.h>
#include <dirent.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <sys/mount.h>
#include <sched.h>

#include "container.h"
#include "namespace.h"
#include "rootfs.h"
#include "cgroup.h"
#include "network.h"

/* ================================================================== */
/* Constants                                                            */
/* ================================================================== */

#define CLONE_STACK_SIZE (2 * 1024 * 1024) /* 2 MiB stack for child  */

/* ================================================================== */
/* Internal helpers                                                     */
/* ================================================================== */

/* Build path to a container's state directory. */
static void state_dir(char *buf, size_t sz, const char *id)
{
    snprintf(buf, sz, "%s/%s", CSPIP_STATE_DIR, id);
}

/* Build path to a container's state.json file. */
static void state_file(char *buf, size_t sz, const char *id)
{
    snprintf(buf, sz, "%s/%s/state.json", CSPIP_STATE_DIR, id);
}

/* Return a string for ContainerStatus. */
static const char *status_str(ContainerStatus s)
{
    switch (s) {
    case STATUS_CREATED: return "created";
    case STATUS_RUNNING: return "running";
    case STATUS_STOPPED: return "stopped";
    default:             return "unknown";
    }
}

/* Parse a status string back to enum. */
static ContainerStatus parse_status(const char *s)
{
    if (strcmp(s, "running") == 0) return STATUS_RUNNING;
    if (strcmp(s, "stopped") == 0) return STATUS_STOPPED;
    return STATUS_CREATED;
}

/* ------------------------------------------------------------------ */
/* State file I/O                                                       */
/* ------------------------------------------------------------------ */

static int write_state(const ContainerState *st)
{
    char dir[MAX_PATH_LEN];
    state_dir(dir, sizeof(dir), st->id);

    if (mkdir(CSPIP_STATE_DIR, 0755) < 0 && errno != EEXIST) {
        perror("mkdir " CSPIP_STATE_DIR);
        return -1;
    }
    if (mkdir(dir, 0755) < 0 && errno != EEXIST) {
        perror("mkdir state dir");
        return -1;
    }

    char path[MAX_PATH_LEN];
    state_file(path, sizeof(path), st->id);

    FILE *f = fopen(path, "w");
    if (!f) {
        perror("fopen state.json");
        return -1;
    }

    /* Escape the command string for JSON (replace " with '). */
    char cmd_safe[MAX_CMD_STR];
    strncpy(cmd_safe, st->command, sizeof(cmd_safe) - 1);
    cmd_safe[sizeof(cmd_safe) - 1] = '\0';
    for (char *p = cmd_safe; *p; p++)
        if (*p == '"') *p = '\'';

    fprintf(f,
        "{\n"
        "  \"id\": \"%s\",\n"
        "  \"status\": \"%s\",\n"
        "  \"pid\": %d,\n"
        "  \"command\": \"%s\",\n"
        "  \"created_at\": \"%s\",\n"
        "  \"cpu_limit\": \"%s\",\n"
        "  \"mem_limit\": \"%s\",\n"
        "  \"rootfs\": \"%s\"\n"
        "}\n",
        st->id,
        status_str(st->status),
        st->pid,
        cmd_safe,
        st->created_at,
        st->cpu_limit,
        st->mem_limit,
        st->rootfs);

    fclose(f);
    return 0;
}

/*
 * Extract a JSON string value for key from a flat JSON buffer.
 * Writes into out (size out_sz).  Returns 0 on success, -1 if missing.
 */
static int json_str(const char *json, const char *key,
                    char *out, size_t out_sz)
{
    char search[128];
    snprintf(search, sizeof(search), "\"%s\": \"", key);
    const char *p = strstr(json, search);
    if (!p) return -1;
    p += strlen(search);
    const char *end = strchr(p, '"');
    if (!end) return -1;
    size_t len = (size_t)(end - p);
    if (len >= out_sz) len = out_sz - 1;
    memcpy(out, p, len);
    out[len] = '\0';
    return 0;
}

/*
 * Extract a JSON integer value for key.
 * Returns 0 on success, -1 if missing.
 */
static int json_int(const char *json, const char *key, int *out)
{
    char search[128];
    snprintf(search, sizeof(search), "\"%s\": ", key);
    const char *p = strstr(json, search);
    if (!p) return -1;
    p += strlen(search);
    *out = atoi(p);
    return 0;
}

static int read_state(const char *id, ContainerState *st)
{
    char path[MAX_PATH_LEN];
    state_file(path, sizeof(path), id);

    FILE *f = fopen(path, "r");
    if (!f) return -1;

    char buf[4096];
    size_t n = fread(buf, 1, sizeof(buf) - 1, f);
    fclose(f);
    buf[n] = '\0';

    memset(st, 0, sizeof(*st));

    char status_s[32] = "created";
    json_str(buf, "id",         st->id,         sizeof(st->id));
    json_str(buf, "status",     status_s,        sizeof(status_s));
    json_str(buf, "command",    st->command,     sizeof(st->command));
    json_str(buf, "created_at", st->created_at,  sizeof(st->created_at));
    json_str(buf, "cpu_limit",  st->cpu_limit,   sizeof(st->cpu_limit));
    json_str(buf, "mem_limit",  st->mem_limit,   sizeof(st->mem_limit));
    json_str(buf, "rootfs",     st->rootfs,      sizeof(st->rootfs));
    json_int(buf, "pid",        &st->pid);

    st->status = parse_status(status_s);
    return 0;
}

/* ------------------------------------------------------------------ */
/* Child process entrypoint (runs inside the new namespaces)            */
/* ------------------------------------------------------------------ */

typedef struct {
    ContainerConfig *config;
} ChildArgs;

/*
 * This function is called by clone().  It runs as PID 1 inside the
 * new namespaces.  After setting up the environment it exec()s the
 * user command — at that point it IS the user process.
 */
static int container_child(void *arg)
{
    ChildArgs *a = (ChildArgs *)arg;
    ContainerConfig *cfg = a->config;

    /* 1. Set the container hostname (UTS namespace). */
    if (ns_set_hostname(cfg->hostname) < 0)
        return 1;

    /* 2. Isolate the filesystem (Mount namespace). */
    if (rootfs_setup(cfg->rootfs) < 0)
        return 1;

    /* 3. Mount a fresh /proc (PID namespace). */
    if (ns_mount_proc() < 0)
        return 1;

    /* 4. Minimal /dev. */
    ns_mount_dev(); /* non-fatal — container can still function */

    /* 5. Bring up loopback in the network namespace. */
    net_setup_loopback(); /* non-fatal */

    /* 6. exec() the user command — this replaces this process image. */
    execv(cfg->cmd[0], cfg->cmd);

    /* execv only returns on error. */
    fprintf(stderr, "execv %s: %s\n", cfg->cmd[0], strerror(errno));
    return 1;
}

/* ================================================================== */
/* ID generation                                                        */
/* ================================================================== */

void container_generate_id(char *buf)
{
    unsigned char raw[CONTAINER_ID_LEN / 2];

    int fd = open("/dev/urandom", O_RDONLY);
    if (fd < 0 || read(fd, raw, sizeof(raw)) != (ssize_t)sizeof(raw)) {
        /* Fallback: use time + PID */
        unsigned int seed = (unsigned int)(time(NULL) ^ getpid());
        srand(seed);
        for (size_t i = 0; i < sizeof(raw); i++)
            raw[i] = (unsigned char)(rand() & 0xFF);
    }
    if (fd >= 0) close(fd);

    for (size_t i = 0; i < sizeof(raw); i++)
        sprintf(buf + i * 2, "%02x", raw[i]);
    buf[CONTAINER_ID_LEN] = '\0';
}

/* ================================================================== */
/* container_run                                                        */
/* ================================================================== */

int container_run(ContainerConfig *config)
{
    /* Generate an ID if one wasn't provided. */
    if (config->id[0] == '\0')
        container_generate_id(config->id);

    /* Default hostname = first 8 chars of id. */
    if (config->hostname[0] == '\0') {
        strncpy(config->hostname, config->id, 8);
        config->hostname[8] = '\0';
    }

    /* -------------------------------------------------------------- */
    /* 1. Set up the cgroup BEFORE the child starts so limits are      */
    /*    enforced from the first instruction.                          */
    /* -------------------------------------------------------------- */
    if (cgroup_create(config->id) < 0)
        return -1;
    if (config->cpu_limit[0])
        cgroup_set_cpu(config->id, config->cpu_limit);
    if (config->mem_limit[0])
        cgroup_set_mem(config->id, config->mem_limit);

    /* -------------------------------------------------------------- */
    /* 2. Write initial state (created).                               */
    /* -------------------------------------------------------------- */
    ContainerState st;
    memset(&st, 0, sizeof(st));
    strncpy(st.id,        config->id,        sizeof(st.id) - 1);
    strncpy(st.cpu_limit, config->cpu_limit, sizeof(st.cpu_limit) - 1);
    strncpy(st.mem_limit, config->mem_limit, sizeof(st.mem_limit) - 1);
    strncpy(st.rootfs,    config->rootfs,    sizeof(st.rootfs) - 1);
    st.status = STATUS_CREATED;
    st.pid    = 0;

    /* Build display command string. */
    for (int i = 0; i < config->cmd_argc && config->cmd[i]; i++) {
        if (i) strncat(st.command, " ", sizeof(st.command) - strlen(st.command) - 1);
        strncat(st.command, config->cmd[i], sizeof(st.command) - strlen(st.command) - 1);
    }

    /* ISO-8601 timestamp. */
    time_t now = time(NULL);
    struct tm *tm_info = gmtime(&now);
    strftime(st.created_at, sizeof(st.created_at),
             "%Y-%m-%dT%H:%M:%SZ", tm_info);

    if (write_state(&st) < 0) {
        cgroup_destroy(config->id);
        return -1;
    }

    /* -------------------------------------------------------------- */
    /* 3. Allocate a stack for the child process.                      */
    /* -------------------------------------------------------------- */
    char *stack = malloc(CLONE_STACK_SIZE);
    if (!stack) {
        perror("malloc clone stack");
        cgroup_destroy(config->id);
        return -1;
    }
    char *stack_top = stack + CLONE_STACK_SIZE; /* stack grows down */

    ChildArgs child_args = { .config = config };

    /* -------------------------------------------------------------- */
    /* 4. clone() — creates child in new namespaces.                   */
    /*                                                                  */
    /*    CLONE_NEWPID  — container gets its own PID 1                 */
    /*    CLONE_NEWNS   — independent mount table                      */
    /*    CLONE_NEWUTS  — independent hostname                         */
    /*    CLONE_NEWNET  — independent network stack                    */
    /*    SIGCHLD       — notify parent when child exits               */
    /* -------------------------------------------------------------- */
    pid_t pid = clone(container_child, stack_top,
                      CLONE_NEWPID | CLONE_NEWNS |
                      CLONE_NEWUTS | CLONE_NEWNET |
                      SIGCHLD,
                      &child_args);

    if (pid < 0) {
        perror("clone");
        free(stack);
        cgroup_destroy(config->id);
        return -1;
    }

    /* -------------------------------------------------------------- */
    /* 5. Parent: add child to cgroup, update state to running.        */
    /* -------------------------------------------------------------- */
    if (cgroup_add_pid(config->id, pid) < 0) {
        fprintf(stderr, "warn: failed to add PID %d to cgroup\n", pid);
    }

    st.status = STATUS_RUNNING;
    st.pid    = pid;
    write_state(&st);

    /* Write the "started" announcement to stderr, not stdout.
     * The Go wrapper pipes stderr to extract this line; stdout is passed
     * through directly to the terminal so the container process (e.g. a
     * shell) sees a real TTY and behaves interactively. */
    fprintf(stderr, "Container %s started (PID %d)\n", config->id, pid);
    fflush(stderr);

    /* -------------------------------------------------------------- */
    /* 6. Wait for the container to finish.                            */
    /* -------------------------------------------------------------- */
    int wstatus = 0;
    waitpid(pid, &wstatus, 0);
    free(stack);

    int exit_code = WIFEXITED(wstatus) ? WEXITSTATUS(wstatus) : 1;

    /* -------------------------------------------------------------- */
    /* 7. Update state to stopped.                                     */
    /* -------------------------------------------------------------- */
    st.status = STATUS_STOPPED;
    st.pid    = 0;
    write_state(&st);

    return exit_code;
}

/* ================================================================== */
/* container_exec                                                       */
/* ================================================================== */

int container_exec(const char *container_id,
                   char *const cmd[], int cmd_argc)
{
    (void)cmd_argc;

    ContainerState st;
    if (container_inspect(container_id, &st) < 0) {
        fprintf(stderr, "container %s not found\n", container_id);
        return -1;
    }
    if (st.status != STATUS_RUNNING) {
        fprintf(stderr, "container %s is not running (status: %s)\n",
                container_id, status_str(st.status));
        return -1;
    }

    /* Enter the container's namespaces (no network join for exec). */
    if (ns_enter((pid_t)st.pid, 0) < 0)
        return -1;

    /* exec replaces this process — does not return on success. */
    execv(cmd[0], cmd);
    perror("execv");
    return -1;
}

/* ================================================================== */
/* container_stop / container_kill                                      */
/* ================================================================== */

int container_stop(const char *container_id)
{
    ContainerState st;
    if (container_inspect(container_id, &st) < 0)
        return -1;

    if (st.status != STATUS_RUNNING) {
        fprintf(stderr, "container %s is not running\n", container_id);
        return -1;
    }

    if (kill((pid_t)st.pid, SIGTERM) < 0) {
        perror("kill SIGTERM");
        return -1;
    }

    /* Wait up to 10 seconds for graceful shutdown. */
    for (int i = 0; i < 100; i++) {
        usleep(100000); /* 100 ms */
        if (kill((pid_t)st.pid, 0) < 0 && errno == ESRCH)
            break; /* process gone */
    }

    st.status = STATUS_STOPPED;
    st.pid    = 0;
    write_state(&st);
    printf("Container %s stopped\n", container_id);
    return 0;
}

int container_kill(const char *container_id)
{
    ContainerState st;
    if (container_inspect(container_id, &st) < 0)
        return -1;

    if (st.status != STATUS_RUNNING) {
        fprintf(stderr, "container %s is not running\n", container_id);
        return -1;
    }

    if (kill((pid_t)st.pid, SIGKILL) < 0) {
        perror("kill SIGKILL");
        return -1;
    }

    st.status = STATUS_STOPPED;
    st.pid    = 0;
    write_state(&st);
    printf("Container %s killed\n", container_id);
    return 0;
}

/* ================================================================== */
/* container_remove                                                     */
/* ================================================================== */

int container_remove(const char *container_id)
{
    ContainerState st;
    if (container_inspect(container_id, &st) < 0) {
        fprintf(stderr, "container %s not found\n", container_id);
        return -1;
    }
    if (st.status == STATUS_RUNNING) {
        fprintf(stderr,
                "container %s is running — stop it first\n",
                container_id);
        return -1;
    }

    /* Remove cgroup. */
    cgroup_destroy(container_id); /* best-effort */

    /* Remove state directory. */
    char path[MAX_PATH_LEN];
    state_file(path, sizeof(path), container_id);
    unlink(path);

    char dir[MAX_PATH_LEN];
    state_dir(dir, sizeof(dir), container_id);
    if (rmdir(dir) < 0) {
        perror("rmdir state dir");
        return -1;
    }

    printf("Container %s removed\n", container_id);
    return 0;
}

/* ================================================================== */
/* container_inspect                                                    */
/* ================================================================== */

int container_inspect(const char *container_id, ContainerState *out)
{
    return read_state(container_id, out);
}

/* ================================================================== */
/* container_list                                                       */
/* ================================================================== */

void container_list(void)
{
    DIR *d = opendir(CSPIP_STATE_DIR);
    if (!d) {
        /* No containers yet — print empty header. */
        printf("%-14s %-10s %-8s %s\n",
               "CONTAINER ID", "STATUS", "PID", "COMMAND");
        return;
    }

    printf("%-14s %-10s %-8s %s\n",
           "CONTAINER ID", "STATUS", "PID", "COMMAND");

    struct dirent *entry;
    while ((entry = readdir(d)) != NULL) {
        if (entry->d_name[0] == '.')
            continue;

        ContainerState st;
        if (read_state(entry->d_name, &st) == 0) {
            char cmd_trunc[32];
            strncpy(cmd_trunc, st.command, 31);
            cmd_trunc[31] = '\0';

            printf("%-14s %-10s %-8d %s\n",
                   st.id,
                   status_str(st.status),
                   st.pid,
                   cmd_trunc);
        }
    }
    closedir(d);
}
