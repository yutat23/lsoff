#ifdef __APPLE__

#include "helper_darwin.h"

#include <arpa/inet.h>
#include <netinet/in.h>
#include <stdint.h>
#include <string.h>
#include <sys/socket.h>

static void format_in_addr(const struct in_sockinfo *in, char *addr, int addrlen) {
	memset(addr, 0, (size_t)addrlen);
	if (in->insi_vflag & INI_IPV4) {
		inet_ntop(AF_INET, &in->insi_laddr.ina_46.i46a_addr4, addr, (socklen_t)addrlen);
		return;
	}
	inet_ntop(AF_INET6, &in->insi_laddr.ina_6, addr, (socklen_t)addrlen);
}

int lsoff_parse_listen(const struct socket_fdinfo *si, int *proto, int *port, char *addr, int addrlen) {
	if (si->psi.soi_family != AF_INET && si->psi.soi_family != AF_INET6) {
		return 0;
	}

	if (si->psi.soi_kind == SOCKINFO_TCP) {
		const struct tcp_sockinfo *tcp = &si->psi.soi_proto.pri_tcp;
		if (tcp->tcpsi_state != TSI_S_LISTEN) {
			return 0;
		}
		int p = ntohs((uint16_t)tcp->tcpsi_ini.insi_lport);
		if (p == 0) {
			return 0;
		}
		*proto = IPPROTO_TCP;
		*port = p;
		format_in_addr(&tcp->tcpsi_ini, addr, addrlen);
		return 1;
	}

	if (si->psi.soi_protocol == IPPROTO_UDP || si->psi.soi_type == SOCK_DGRAM) {
		const struct in_sockinfo *in = &si->psi.soi_proto.pri_in;
		int p = ntohs((uint16_t)in->insi_lport);
		int fp = ntohs((uint16_t)in->insi_fport);
		if (p == 0 || fp != 0) {
			return 0;
		}
		*proto = IPPROTO_UDP;
		*port = p;
		format_in_addr(in, addr, addrlen);
		return 1;
	}

	return 0;
}

#endif
