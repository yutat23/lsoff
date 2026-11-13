//go:build darwin

package listen

/*
#include <sys/types.h>
#include <sys/socket.h>
#include <sys/socketvar.h>
#include <netinet/in.h>
#include <netinet/in_pcb.h>
#include <arpa/inet.h>
#include "helper_darwin.h"
*/
import "C"

import (
	"encoding/binary"
	"unsafe"
)

// The two helpers below feed a synthetic PCB through the C address
// formatters of the libproc and sysctl paths. cgo cannot be used from
// _test.go files, so they live here; they let tests pin down how each
// vflag combination is rendered without opening real listening sockets.
//
// laddr is the raw 16-byte local-address union: an IPv6 address, or for IPv4
// 12 bytes of padding followed by the IPv4 address.

// libprocListenAddr returns the address lsoff_parse_listen reports for a TCP
// LISTEN socket with the given version flags (INI_IPV4/INI_IPV6) and local
// address, or "" if it rejects the socket.
func libprocListenAddr(vflag uint8, laddr [16]byte) string {
	var si C.struct_socket_fdinfo
	si.psi.soi_family = C.AF_INET6
	si.psi.soi_kind = C.SOCKINFO_TCP
	tcp := (*C.struct_tcp_sockinfo)(unsafe.Pointer(&si.psi.soi_proto[0]))
	tcp.tcpsi_state = C.TSI_S_LISTEN
	tcp.tcpsi_ini.insi_lport = C.int(netPort(8080))
	tcp.tcpsi_ini.insi_vflag = C.uint8_t(vflag)
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&tcp.tcpsi_ini.insi_laddr)), 16), laddr[:])

	var proto, port C.int
	addr := make([]byte, C.INET6_ADDRSTRLEN)
	if C.lsoff_parse_listen(&si, &proto, &port, (*C.char)(unsafe.Pointer(&addr[0])), C.int(len(addr))) == 0 {
		return ""
	}
	return C.GoString((*C.char)(unsafe.Pointer(&addr[0])))
}

// xinpcbListenAddr returns the address the sysctl walk reports for a PCB
// record with the given version flags (INP_IPV4/INP_IPV6) and local address.
func xinpcbListenAddr(vflag uint8, laddr [16]byte) string {
	var xi C.struct_xinpcb64
	xi.inp_vflag = C.u_char(vflag)
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&xi.inp_dependladdr)), 16), laddr[:])

	addr := make([]byte, C.INET6_ADDRSTRLEN)
	C.lsoff_format_xinpcb_addr(&xi, (*C.char)(unsafe.Pointer(&addr[0])), C.size_t(len(addr)))
	return C.GoString((*C.char)(unsafe.Pointer(&addr[0])))
}

// netPort returns p in network byte order, as the kernel stores it.
func netPort(p uint16) uint16 {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], p)
	return binary.NativeEndian.Uint16(b[:])
}

// Version flag values, exposed so tests can name them without cgo.
const (
	iniIPv4 = uint8(C.INI_IPV4)
	iniIPv6 = uint8(C.INI_IPV6)
	inpIPv4 = uint8(C.INP_IPV4)
	inpIPv6 = uint8(C.INP_IPV6)
)
