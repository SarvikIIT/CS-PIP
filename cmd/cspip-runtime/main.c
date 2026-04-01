#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <errno.h>

#include "internal/runtime/container.h"
#include "internal/runtime/types.h"

/* ================================================================== */
/* Usage / help                                                         */
/* ================================================================== */

static void usage(void)
{
    fprintf(stderr,
        "Usage: cspip <command> [options]\n"
        "\n"
        "Commands:\n"
        "  run   [--cpu-limit <pct>] [--mem-limit <size>] <rootfs> <cmd> [args...]\n"
        "           Create and run a container.\n"
        "           --cpu-limit  CPU limit, e.g. '50%%' or '0.5'\n"
        "           --mem-limit  Memory limit, e.g. '256m', '1g'\n"
        "\n"
        "  exec  <container-id> <cmd> [args...]\n"
        "           Run a command inside a running container.\n"
        "\n"
        "  inspect <container-id>\n"
        "           Print container state as JSON.\n"
        "\n"
        "  ps      List all containers.\n"
        "\n"
        "  stop    <container-id>   Send SIGTERM and wait for exit.\n"
        "  kill    <container-id>   Send SIGKILL immediately.\n"
        "  rm      <container-id>   Remove a stopped container.\n"
        "\n"
        "Note: most operations require root (sudo).\n"
    );
}

/* ================================================================== */
/* `run` command                                                        */
/* ================================================================== */

static int cmd_run(int argc, char *argv[])
{
    /*
     * argv[0] = "run"
     * Parse: [--cpu-limit X] [--mem-limit Y] <rootfs> <command> [args...]
     */
    ContainerConfig cfg;
    memset(&cfg, 0, sizeof(cfg));

    int i = 1; /* skip "run" */
    for (; i < argc; i++) {
        if (strcmp(argv[i], "--cpu-limit") == 0) {
            if (i + 1 >= argc) {
                fprintf(stderr, "error: --cpu-limit requires an argument\n");
                return 1;
            }
            strncpy(cfg.cpu_limit, argv[++i], sizeof(cfg.cpu_limit) - 1);
        } else if (strcmp(argv[i], "--mem-limit") == 0) {
            if (i + 1 >= argc) {
                fprintf(stderr, "error: --mem-limit requires an argument\n");
                return 1;
            }
            strncpy(cfg.mem_limit, argv[++i], sizeof(cfg.mem_limit) - 1);
        } else {
            break; /* first positional arg = rootfs path */
        }
    }

    if (i >= argc) {
        fprintf(stderr, "error: missing <rootfs> argument\n");
        usage();
        return 1;
    }
    strncpy(cfg.rootfs, argv[i++], sizeof(cfg.rootfs) - 1);

    if (i >= argc) {
        fprintf(stderr, "error: missing <command> argument\n");
        usage();
        return 1;
    }

    /* Collect command + arguments into cfg.cmd (NULL-terminated). */
    int j = 0;
    for (; i < argc && j < MAX_CMD_ARGS - 1; i++, j++) {
        cfg.cmd[j] = argv[i];
    }
    cfg.cmd[j]  = NULL;
    cfg.cmd_argc = j;

    return container_run(&cfg);
}

/* ================================================================== */
/* `exec` command                                                       */
/* ================================================================== */

static int cmd_exec(int argc, char *argv[])
{
    /*
     * argv[0] = "exec"
     * argv[1] = <container-id>
     * argv[2..] = <command> [args...]
     */
    if (argc < 3) {
        fprintf(stderr, "error: exec requires <container-id> and <command>\n");
        usage();
        return 1;
    }

    const char *id = argv[1];
    char **cmd      = &argv[2];
    int cmd_argc    = argc - 2;

    return container_exec(id, cmd, cmd_argc);
}

/* ================================================================== */
/* `inspect` command                                                    */
/* ================================================================== */

static int cmd_inspect(int argc, char *argv[])
{
    if (argc < 2) {
        fprintf(stderr, "error: inspect requires <container-id>\n");
        return 1;
    }

    ContainerState st;
    if (container_inspect(argv[1], &st) < 0) {
        fprintf(stderr, "container not found: %s\n", argv[1]);
        return 1;
    }

    const char *status;
    switch (st.status) {
    case STATUS_CREATED: status = "created"; break;
    case STATUS_RUNNING: status = "running"; break;
    case STATUS_STOPPED: status = "stopped"; break;
    default:             status = "unknown"; break;
    }

    printf("{\n");
    printf("  \"id\":         \"%s\",\n",  st.id);
    printf("  \"status\":     \"%s\",\n",  status);
    printf("  \"pid\":        %d,\n",      st.pid);
    printf("  \"command\":    \"%s\",\n",  st.command);
    printf("  \"created_at\": \"%s\",\n",  st.created_at);
    printf("  \"cpu_limit\":  \"%s\",\n",  st.cpu_limit);
    printf("  \"mem_limit\":  \"%s\",\n",  st.mem_limit);
    printf("  \"rootfs\":     \"%s\"\n",   st.rootfs);
    printf("}\n");
    return 0;
}

/* ================================================================== */
/* `ps` command                                                         */
/* ================================================================== */

static int cmd_ps(void)
{
    container_list();
    return 0;
}

/* ================================================================== */
/* `stop`, `kill`, `rm` commands                                        */
/* ================================================================== */

static int cmd_stop(int argc, char *argv[])
{
    if (argc < 2) {
        fprintf(stderr, "error: stop requires <container-id>\n");
        return 1;
    }
    return (container_stop(argv[1]) == 0) ? 0 : 1;
}

static int cmd_kill(int argc, char *argv[])
{
    if (argc < 2) {
        fprintf(stderr, "error: kill requires <container-id>\n");
        return 1;
    }
    return (container_kill(argv[1]) == 0) ? 0 : 1;
}

static int cmd_rm(int argc, char *argv[])
{
    if (argc < 2) {
        fprintf(stderr, "error: rm requires <container-id>\n");
        return 1;
    }
    return (container_remove(argv[1]) == 0) ? 0 : 1;
}

/* ================================================================== */
/* main                                                                 */
/* ================================================================== */

int main(int argc, char *argv[])
{
    if (argc < 2) {
        usage();
        return 1;
    }

    const char *command = argv[1];

    /* Shift argv so each sub-command handler receives its own argv[0]. */
    int  sub_argc = argc - 1;
    char **sub_argv = argv + 1;

    if (strcmp(command, "run") == 0) {
        return cmd_run(sub_argc, sub_argv);

    } else if (strcmp(command, "exec") == 0) {
        return cmd_exec(sub_argc, sub_argv);

    } else if (strcmp(command, "inspect") == 0) {
        return cmd_inspect(sub_argc, sub_argv);

    } else if (strcmp(command, "ps") == 0) {
        return cmd_ps();

    } else if (strcmp(command, "stop") == 0) {
        return cmd_stop(sub_argc, sub_argv);

    } else if (strcmp(command, "kill") == 0) {
        return cmd_kill(sub_argc, sub_argv);

    } else if (strcmp(command, "rm") == 0) {
        return cmd_rm(sub_argc, sub_argv);

    } else if (strcmp(command, "--help") == 0 ||
               strcmp(command, "-h") == 0) {
        usage();
        return 0;

    } else {
        fprintf(stderr, "error: unknown command '%s'\n\n", command);
        usage();
        return 1;
    }
}
