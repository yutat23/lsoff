//go:build darwin

package listen

/*
#include "helper_darwin.h"
#include <libproc.h>
#include <arpa/inet.h>
*/
import "C"

import (
	"fmt"
	"path/filepath"
	"unsafe"
)

// List returns LISTEN TCP sockets and bound UDP sockets via libproc.
func List() ([]Entry, error) {
	pids, err := listPIDs()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var out []Entry
	for _, pid := range pids {
		for _, e := range socketsForPID(pid) {
			k := e.Key()
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, e)
		}
	}
	Sort(out)
	return out, nil
}

func listPIDs() ([]int, error) {
	n := C.proc_listpids(C.PROC_ALL_PIDS, 0, nil, 0)
	if n <= 0 {
		return nil, fmt.Errorf("proc_listpids failed")
	}
	count := int(n)/int(C.sizeof_pid_t) + 32
	buf := make([]C.pid_t, count)
	n = C.proc_listpids(C.PROC_ALL_PIDS, 0, unsafe.Pointer(&buf[0]), C.int(count)*C.int(C.sizeof_pid_t))
	if n <= 0 {
		return nil, fmt.Errorf("proc_listpids failed")
	}
	got := int(n) / int(C.sizeof_pid_t)
	out := make([]int, 0, got)
	for i := 0; i < got; i++ {
		if buf[i] > 0 {
			out = append(out, int(buf[i]))
		}
	}
	return out, nil
}

func socketsForPID(pid int) []Entry {
	need := C.proc_pidinfo(C.int(pid), C.PROC_PIDLISTFDS, 0, nil, 0)
	if need <= 0 {
		return nil
	}
	bufSize := need * 2
	buf := make([]byte, int(bufSize))
	n := C.proc_pidinfo(C.int(pid), C.PROC_PIDLISTFDS, 0, unsafe.Pointer(&buf[0]), bufSize)
	if n <= 0 {
		return nil
	}

	fdSize := int(C.sizeof_struct_proc_fdinfo)
	count := int(n) / fdSize
	var name, path, cmdline, cwd string
	var start uint64
	loaded := false
	var out []Entry

	for i := 0; i < count; i++ {
		fdinfo := (*C.struct_proc_fdinfo)(unsafe.Pointer(&buf[i*fdSize]))
		if fdinfo.proc_fdtype != C.PROX_FDTYPE_SOCKET {
			continue
		}
		var si C.struct_socket_fdinfo
		got := C.proc_pidfdinfo(
			C.int(pid),
			C.int(fdinfo.proc_fd),
			C.PROC_PIDFDSOCKETINFO,
			unsafe.Pointer(&si),
			C.int(C.sizeof_struct_socket_fdinfo),
		)
		if got <= 0 {
			continue
		}

		var proto, port C.int
		addr := make([]byte, C.INET6_ADDRSTRLEN)
		if C.lsoff_parse_listen(&si, &proto, &port, (*C.char)(unsafe.Pointer(&addr[0])), C.int(len(addr))) == 0 {
			continue
		}
		if !loaded {
			name, path = procNamePath(pid)
			cmdline = procCmdline(pid)
			cwd = procCwd(pid)
			start, _ = procStartToken(pid)
			loaded = true
		}

		p := TCP
		if int(proto) == C.IPPROTO_UDP {
			p = UDP
		}
		out = append(out, Entry{
			Proto:   p,
			Port:    uint16(port),
			Addr:    normalizeAddr(C.GoString((*C.char)(unsafe.Pointer(&addr[0])))),
			PID:     pid,
			Name:    name,
			Path:    path,
			Cmdline: cmdline,
			Cwd:     cwd,
			Project: ProjectName(cwd),
			Start:   start,
		})
	}
	return out
}

func procNamePath(pid int) (string, string) {
	var pathBuf [C.PROC_PIDPATHINFO_MAXSIZE]byte
	n := C.proc_pidpath(C.int(pid), unsafe.Pointer(&pathBuf[0]), C.uint32_t(len(pathBuf)))
	path := ""
	if n > 0 {
		path = C.GoString((*C.char)(unsafe.Pointer(&pathBuf[0])))
	}

	var nameBuf [32]byte
	n = C.proc_name(C.int(pid), unsafe.Pointer(&nameBuf[0]), C.uint32_t(len(nameBuf)))
	name := ""
	if n > 0 {
		name = C.GoString((*C.char)(unsafe.Pointer(&nameBuf[0])))
	}
	if name == "" && path != "" {
		name = filepath.Base(path)
	}
	return name, path
}

func procCwd(pid int) string {
	var info C.struct_proc_vnodepathinfo
	n := C.proc_pidinfo(C.int(pid), C.PROC_PIDVNODEPATHINFO, 0, unsafe.Pointer(&info), C.int(C.sizeof_struct_proc_vnodepathinfo))
	if n <= 0 {
		return ""
	}
	return C.GoString(&info.pvi_cdir.vip_path[0])
}
