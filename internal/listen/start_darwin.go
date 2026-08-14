//go:build darwin

package listen

/*
#include <libproc.h>
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func procStartToken(pid int) (uint64, error) {
	var info C.struct_proc_bsdinfo
	n := C.proc_pidinfo(C.int(pid), C.PROC_PIDTBSDINFO, 0, unsafe.Pointer(&info), C.int(C.sizeof_struct_proc_bsdinfo))
	if n < C.int(C.sizeof_struct_proc_bsdinfo) {
		return 0, fmt.Errorf("proc_pidinfo bsdinfo: %d", int(n))
	}
	usec := uint64(info.pbi_start_tvusec)
	if usec >= 1_000_000 {
		return 0, fmt.Errorf("bad start usec %d", usec)
	}
	token := uint64(info.pbi_start_tvsec)*1_000_000 + usec
	if token == 0 {
		return 0, errNoIdentity
	}
	return token, nil
}
