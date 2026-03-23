#ifndef CSPIP_TYPES_H
#define CSPIP_TYPES_H

#include <sys/types.h>
#include <time.h>

#define CONTAINER_ID_LEN  12
#define MAX_PATH_LEN      512
#define MAX_CMD_ARGS      64
#define MAX_CMD_STR       1024
#define CSPIP_STATE_DIR   "/var/run/cspip"
#define CSPIP_CGROUP_ROOT "/sys/fs/cgroup/cspip"

/* Container lifecycle states */
typedef enum {
    STATUS_CREATED = 0,
    STATUS_RUNNING,
    STATUS_STOPPED
} ContainerStatus;

/*
 * Configuration provided by the user when creating/running a container.
 * All pointer fields are borrowed — caller owns the memory.
 */
typedef struct {
    char     id[CONTAINER_ID_LEN + 1]; /* auto-generated if empty */
    char     rootfs[MAX_PATH_LEN];
    char    *cmd[MAX_CMD_ARGS];         /* NULL-terminated argv */
    int      cmd_argc;
    char     cpu_limit[32];             /* e.g. "50%"  */
    char     mem_limit[32];             /* e.g. "256m" */
    char     hostname[256];             /* defaults to container id */
} ContainerConfig;

/*
 * Persisted container state — written to
 * /var/run/cspip/<id>/state.json
 */
typedef struct {
    char            id[CONTAINER_ID_LEN + 1];
    ContainerStatus status;
    pid_t           pid;
    char            command[MAX_CMD_STR];   /* joined argv for display */
    char            created_at[64];         /* ISO-8601 timestamp      */
    char            cpu_limit[32];
    char            mem_limit[32];
    char            rootfs[MAX_PATH_LEN];
} ContainerState;

#endif /* CSPIP_TYPES_H */
