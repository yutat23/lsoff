#ifdef __APPLE__

#include "helper_darwin.h"

#include <sys/types.h>
#include <sys/socket.h>
#include <sys/socketvar.h>
#include <sys/sysctl.h>
#include <netinet/in.h>
#include <netinet/in_pcb.h>
#include <netinet/tcp.h>
#include <netinet/tcp_var.h>
#include <netinet/tcp_fsm.h>
#include <arpa/inet.h>
#include <errno.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

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

/* Same rendering as format_in_addr, for the xinpcb64 flavour of a PCB. */
static void format_xinpcb_addr(const struct xinpcb64 *xi, char *addr, size_t addrlen) {
	memset(addr, 0, addrlen);
	if (xi->inp_vflag & INP_IPV4) {
		inet_ntop(AF_INET, &xi->inp_dependladdr.inp46_local.ia46_addr4, addr, (socklen_t)addrlen);
		return;
	}
	inet_ntop(AF_INET6, &xi->inp_dependladdr.inp6_local, addr, (socklen_t)addrlen);
}

/*
 * fetch_pcblist reads a whole sysctl PCB list. The list can grow between the
 * size query and the fetch, so over-allocate and retry on ENOMEM like netstat.
 */
static char *fetch_pcblist(const char *name, size_t *lenp) {
	size_t len = 0;
	if (sysctlbyname(name, NULL, &len, NULL, 0) != 0 || len == 0) {
		return NULL;
	}
	for (int attempt = 0; attempt < 5; attempt++) {
		size_t alloc = len + len / 8 + 4096;
		char *buf = malloc(alloc);
		if (buf == NULL) {
			return NULL;
		}
		size_t got = alloc;
		if (sysctlbyname(name, buf, &got, NULL, 0) == 0 && got >= sizeof(struct xinpgen)) {
			*lenp = got;
			return buf;
		}
		int saved = errno;
		free(buf);
		if (saved != ENOMEM) {
			return NULL;
		}
		len = alloc;
	}
	return NULL;
}

struct sock_sink {
	struct lsoff_socket *buf;
	int len;
	int cap;
};

static int sink_push(struct sock_sink *s, int proto, int port, const char *addr) {
	if (s->len == s->cap) {
		int cap = s->cap == 0 ? 64 : s->cap * 2;
		struct lsoff_socket *grown = realloc(s->buf, (size_t)cap * sizeof(*grown));
		if (grown == NULL) {
			return 0;
		}
		s->buf = grown;
		s->cap = cap;
	}
	struct lsoff_socket *e = &s->buf[s->len++];
	e->proto = proto;
	e->port = port;
	memset(e->addr, 0, sizeof(e->addr));
	strncpy(e->addr, addr, sizeof(e->addr) - 1);
	return 1;
}

/*
 * walk_pcblist walks one sysctl PCB list: a leading struct xinpgen, then one
 * record per PCB (each starting with its own length), then a trailing
 * struct xinpgen. Every step is bounded by the end of the buffer and the
 * cursor always advances by a length larger than the terminator, so the loop
 * cannot spin on a malformed or truncated buffer.
 */
static int walk_pcblist(const char *buf, size_t len, int is_tcp, struct sock_sink *sink) {
	const char *p = buf;
	const char *end = buf + len;

	const struct xinpgen *xig = (const struct xinpgen *)(const void *)p;
	size_t head = xig->xig_len;
	if (head < sizeof(struct xinpgen) || head > (size_t)(end - p)) {
		return 0;
	}
	p += head;

	size_t min_rec = is_tcp ? sizeof(struct xtcpcb64) : sizeof(struct xinpcb64);
	while ((size_t)(end - p) > sizeof(struct xinpgen)) {
		const struct xinpcb64 *xi;
		size_t reclen;
		int listening;

		if (is_tcp) {
			const struct xtcpcb64 *xt = (const struct xtcpcb64 *)(const void *)p;
			reclen = xt->xt_len;
			if (reclen <= sizeof(struct xinpgen) || reclen < min_rec || reclen > (size_t)(end - p)) {
				break; /* trailing xinpgen, or a record we cannot trust */
			}
			xi = &xt->xt_inpcb;
			listening = xt->t_state == TCPS_LISTEN;
		} else {
			xi = (const struct xinpcb64 *)(const void *)p;
			reclen = (size_t)xi->xi_len;
			if (reclen <= sizeof(struct xinpgen) || reclen < min_rec || reclen > (size_t)(end - p)) {
				break;
			}
			/* UDP has no LISTEN state: bound and unconnected is the analogue. */
			listening = ntohs(xi->inp_fport) == 0;
		}

		int lport = ntohs(xi->inp_lport);
		if (listening && lport != 0) {
			char addr[INET6_ADDRSTRLEN];
			format_xinpcb_addr(xi, addr, sizeof(addr));
			if (!sink_push(sink, is_tcp ? IPPROTO_TCP : IPPROTO_UDP, lport, addr)) {
				return -1;
			}
		}
		p += reclen;
	}
	return 0;
}

int lsoff_list_sockets(struct lsoff_socket **out) {
	struct sock_sink sink = {NULL, 0, 0};
	int failed = 0;
	int found_any = 0;

	static const struct {
		const char *name;
		int is_tcp;
	} lists[2] = {
		{"net.inet.tcp.pcblist64", 1},
		{"net.inet.udp.pcblist64", 0},
	};

	for (int i = 0; i < 2; i++) {
		size_t len = 0;
		char *buf = fetch_pcblist(lists[i].name, &len);
		if (buf == NULL) {
			continue;
		}
		found_any = 1;
		if (walk_pcblist(buf, len, lists[i].is_tcp, &sink) != 0) {
			failed = 1;
		}
		free(buf);
		if (failed) {
			break;
		}
	}

	if (failed || !found_any) {
		free(sink.buf);
		return -1;
	}
	*out = sink.buf;
	return sink.len;
}

#endif
