#ifndef CSPIP_NETWORK_H
#define CSPIP_NETWORK_H

#include <sys/types.h>

/*
 * network.h — Optional network namespace setup (Stage 1 stub)
 *
 * A full network implementation would:
 *   1. Create a veth pair on the host.
 *   2. Move one end into the container's network namespace.
 *   3. Assign an IP address and bring both ends up.
 *   4. Set up NAT/masquerade rules via iptables.
 *
 * For Stage 1 the container runs in its own network namespace but with
 * only a loopback interface.  The functions below set up the minimum
 * required for the container to have a working lo interface.
 */

/*
 * Bring up the loopback interface inside the container's network
 * namespace.  Call this from inside the child process after
 * rootfs_setup() and before exec-ing the user command.
 *
 * Returns 0 on success, -1 on error.
 */
int net_setup_loopback(void);

#endif /* CSPIP_NETWORK_H */
