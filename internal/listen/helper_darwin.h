#ifndef LSOFF_HELPER_DARWIN_H
#define LSOFF_HELPER_DARWIN_H

#include <libproc.h>
#include <netinet/in.h>

int lsoff_parse_listen(const struct socket_fdinfo *si, int *proto, int *port, char *addr, int addrlen);

/* One listening socket found by walking the kernel PCB lists. */
struct lsoff_socket {
	int proto; /* IPPROTO_TCP or IPPROTO_UDP */
	int port;  /* local port, host byte order */
	char addr[INET6_ADDRSTRLEN];
};

/*
 * lsoff_list_sockets enumerates every listening TCP socket and every bound
 * (unconnected) UDP socket on the host via sysctl, which unlike libproc needs
 * no privilege over the owning process.
 *
 * On success it returns the number of sockets and stores a malloc'd array of
 * that many elements in *out (NULL when the count is 0); the caller must
 * free(*out). On failure it returns -1 and leaves *out untouched.
 */
int lsoff_list_sockets(struct lsoff_socket **out);

#endif
