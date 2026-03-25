#define _GNU_SOURCE
#include <stdio.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <sys/socket.h>
#include <sys/ioctl.h>
#include <net/if.h>
#include <arpa/inet.h>

#include "network.h"

/*
 * Bring up the loopback interface (lo) inside the container.
 *
 * Equivalent to:
 *   ip link set lo up
 *   ip addr add 127.0.0.1/8 dev lo
 *
 * We use raw POSIX socket ioctls (SIOCSIFFLAGS, SIOCSIFADDR) so there
 * is no dependency on iproute2 or libnetlink.
 */
int net_setup_loopback(void)
{
    int sock = socket(AF_INET, SOCK_DGRAM, 0);
    if (sock < 0) {
        perror("socket (loopback setup)");
        return -1;
    }

    /* ----------------------------------------------------------------
     * Assign 127.0.0.1 to lo
     * ---------------------------------------------------------------- */
    struct ifreq req_addr;
    memset(&req_addr, 0, sizeof(req_addr));
    strncpy(req_addr.ifr_name, "lo", IFNAMSIZ - 1);

    struct sockaddr_in *sin =
        (struct sockaddr_in *)&req_addr.ifr_addr;
    sin->sin_family = AF_INET;
    sin->sin_addr.s_addr = htonl(INADDR_LOOPBACK); /* 127.0.0.1 */

    if (ioctl(sock, SIOCSIFADDR, &req_addr) < 0) {
        /* May already be set — not fatal. */
        if (errno != EEXIST) {
            fprintf(stderr, "warn: SIOCSIFADDR lo: %s\n", strerror(errno));
        }
    }

    /* ----------------------------------------------------------------
     * Bring the interface UP
     * ---------------------------------------------------------------- */
    struct ifreq req_flags;
    memset(&req_flags, 0, sizeof(req_flags));
    strncpy(req_flags.ifr_name, "lo", IFNAMSIZ - 1);

    if (ioctl(sock, SIOCGIFFLAGS, &req_flags) < 0) {
        perror("SIOCGIFFLAGS lo");
        close(sock);
        return -1;
    }

    req_flags.ifr_flags |= IFF_UP | IFF_LOOPBACK | IFF_RUNNING;

    if (ioctl(sock, SIOCSIFFLAGS, &req_flags) < 0) {
        perror("SIOCSIFFLAGS lo");
        close(sock);
        return -1;
    }

    close(sock);
    return 0;
}
