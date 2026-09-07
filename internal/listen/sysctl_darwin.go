//go:build darwin

package listen

/*
#include <stdlib.h>
#include "helper_darwin.h"
*/
import "C"

import (
	"errors"
	"strconv"
	"unsafe"
)

// rawSocket is a listening socket seen by the kernel, without any process
// attribution. It is the identity part of an Entry: proto, addr and port.
type rawSocket struct {
	Proto Proto
	Port  uint16
	Addr  string
}

// listSockets returns every listening TCP socket and every bound, unconnected
// UDP socket on the host by walking the sysctl PCB lists. Unlike the libproc
// scan this needs no privilege over the owning process, so it also sees
// sockets owned by root and by other users.
func listSockets() ([]rawSocket, error) {
	var arr *C.struct_lsoff_socket
	n := C.lsoff_list_sockets(&arr)
	if n < 0 {
		return nil, errors.New("sysctl pcblist walk failed")
	}
	if n == 0 || arr == nil {
		return nil, nil
	}
	defer C.free(unsafe.Pointer(arr))

	items := unsafe.Slice(arr, int(n))
	out := make([]rawSocket, 0, len(items))
	for i := range items {
		p := TCP
		if items[i].proto == C.IPPROTO_UDP {
			p = UDP
		}
		out = append(out, rawSocket{
			Proto: p,
			Port:  uint16(items[i].port),
			Addr:  normalizeAddr(C.GoString(&items[i].addr[0])),
		})
	}
	return out, nil
}

// socketKey identifies a socket by proto/addr/port only, ignoring the PID, so
// that a kernel-visible socket can be matched against a libproc row.
func socketKey(proto Proto, addr string, port uint16) string {
	return proto.String() + "/" + addr + "/" + strconv.Itoa(int(port))
}

// mergeUnowned appends a PID-0 Entry for every kernel socket that the libproc
// scan could not attribute to a readable process. Sockets already covered by a
// libproc row (same proto/addr/port) are left alone, so rows keep their PID and
// process details. The result is unsorted; callers sort.
func mergeUnowned(entries []Entry, socks []rawSocket) []Entry {
	owned := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		owned[socketKey(e.Proto, e.Addr, e.Port)] = struct{}{}
	}
	out := entries
	for _, s := range socks {
		k := socketKey(s.Proto, s.Addr, s.Port)
		if _, ok := owned[k]; ok {
			continue
		}
		owned[k] = struct{}{}
		out = append(out, Entry{Proto: s.Proto, Port: s.Port, Addr: s.Addr})
	}
	return out
}
