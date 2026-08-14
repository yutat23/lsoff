#ifndef LSOFF_HELPER_DARWIN_H
#define LSOFF_HELPER_DARWIN_H

#include <libproc.h>

int lsoff_parse_listen(const struct socket_fdinfo *si, int *proto, int *port, char *addr, int addrlen);

#endif
