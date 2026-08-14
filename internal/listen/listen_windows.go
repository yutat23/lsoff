//go:build windows

package listen

import (
	"encoding/binary"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	tcpTableOwnerPIDListener = 3
	udpTableOwnerPID         = 1
	tcpStateListen           = 2
)

var (
	modiphlpapi             = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable = modiphlpapi.NewProc("GetExtendedTcpTable")
	procGetExtendedUdpTable = modiphlpapi.NewProc("GetExtendedUdpTable")
)

type mibTCPRowOwnerPID struct {
	State      uint32
	LocalAddr  uint32
	LocalPort  uint32
	RemoteAddr uint32
	RemotePort uint32
	OwningPid  uint32
}

type mibTCP6RowOwnerPID struct {
	LocalAddr     [16]byte
	LocalScopeId  uint32
	LocalPort     uint32
	RemoteAddr    [16]byte
	RemoteScopeId uint32
	RemotePort    uint32
	State         uint32
	OwningPid     uint32
}

type mibUDPRowOwnerPID struct {
	LocalAddr uint32
	LocalPort uint32
	OwningPid uint32
}

type mibUDP6RowOwnerPID struct {
	LocalAddr    [16]byte
	LocalScopeId uint32
	LocalPort    uint32
	OwningPid    uint32
}

type procInfo struct {
	Name    string
	Path    string
	Cmdline string
	Cwd     string
	Start   uint64
}

// List returns LISTEN TCP sockets and bound UDP sockets via IP Helper.
func List() ([]Entry, error) {
	cache := make(map[uint32]procInfo)
	var out []Entry

	tcp4, err := getExtendedTable[mibTCPRowOwnerPID](procGetExtendedTcpTable, windows.AF_INET, tcpTableOwnerPIDListener)
	if err != nil {
		return nil, err
	}
	for _, row := range tcp4 {
		if row.State != tcpStateListen {
			continue
		}
		out = append(out, makeEntry(TCP, ipv4String(row.LocalAddr), portFromDWORD(row.LocalPort), row.OwningPid, cache))
	}

	tcp6, err := getExtendedTable[mibTCP6RowOwnerPID](procGetExtendedTcpTable, windows.AF_INET6, tcpTableOwnerPIDListener)
	if err != nil {
		return nil, err
	}
	for _, row := range tcp6 {
		if row.State != tcpStateListen {
			continue
		}
		out = append(out, makeEntry(TCP, ipv6String(row.LocalAddr), portFromDWORD(row.LocalPort), row.OwningPid, cache))
	}

	udp4, err := getExtendedTable[mibUDPRowOwnerPID](procGetExtendedUdpTable, windows.AF_INET, udpTableOwnerPID)
	if err != nil {
		return nil, err
	}
	for _, row := range udp4 {
		port := portFromDWORD(row.LocalPort)
		if port == 0 {
			continue
		}
		out = append(out, makeEntry(UDP, ipv4String(row.LocalAddr), port, row.OwningPid, cache))
	}

	udp6, err := getExtendedTable[mibUDP6RowOwnerPID](procGetExtendedUdpTable, windows.AF_INET6, udpTableOwnerPID)
	if err != nil {
		return nil, err
	}
	for _, row := range udp6 {
		port := portFromDWORD(row.LocalPort)
		if port == 0 {
			continue
		}
		out = append(out, makeEntry(UDP, ipv6String(row.LocalAddr), port, row.OwningPid, cache))
	}

	Sort(out)
	return out, nil
}

func makeEntry(proto Proto, addr string, port uint16, pid uint32, cache map[uint32]procInfo) Entry {
	info := lookupProc(pid, cache)
	return Entry{
		Proto:   proto,
		Port:    port,
		Addr:    addr,
		PID:     int(pid),
		Name:    info.Name,
		Path:    info.Path,
		Cmdline: info.Cmdline,
		Cwd:     info.Cwd,
		Project: ProjectName(info.Cwd),
		Start:   info.Start,
	}
}

func lookupProc(pid uint32, cache map[uint32]procInfo) procInfo {
	if pid == 0 {
		return procInfo{}
	}
	if p, ok := cache[pid]; ok {
		return p
	}
	p := queryProcess(pid)
	cache[pid] = p
	return p
}

func queryProcess(pid uint32) procInfo {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return procInfo{}
	}
	defer windows.CloseHandle(h)

	size := uint32(1024)
	buf := make([]uint16, size)
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return procInfo{}
	}
	path := windows.UTF16ToString(buf)
	start, _ := processStartToken(h)
	cmdline := queryCmdlineCaller(h)
	cwd, pebCmd := queryPebStrings(pid)
	if cmdline == "" {
		cmdline = pebCmd
	}
	return procInfo{
		Name:    filepath.Base(path),
		Path:    path,
		Cmdline: cmdline,
		Cwd:     cwd,
		Start:   start,
	}
}

const (
	processBasicInformation       = 0
	processWow64Information       = 26
	processCommandLineInformation = 60
)

type processBasicInfo struct {
	Reserved1       uintptr
	PebBaseAddress  uintptr
	Reserved2       [2]uintptr
	UniqueProcessID uintptr
	Reserved3       uintptr
}

func queryPebStrings(pid uint32) (cwd, cmdline string) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return "", ""
	}
	defer windows.CloseHandle(h)

	ptrSize, peb := processPeb(h)
	if peb == 0 || ptrSize == 0 {
		return "", ""
	}
	paramsOff, cwdOff, cmdOff := pebOffsets(ptrSize)
	params := readRemotePtrSize(h, peb+paramsOff, ptrSize)
	if params == 0 {
		return "", ""
	}
	cwd = strings.TrimRight(readRemoteUnicode(h, params+cwdOff, ptrSize), `\`)
	cmdline = strings.TrimSpace(readRemoteUnicode(h, params+cmdOff, ptrSize))
	return cwd, cmdline
}

// pebOffsets returns PEB.ProcessParameters, RTL_USER_PROCESS_PARAMETERS
// CurrentDirectory.DosPath, and CommandLine offsets for the target bitness.
func pebOffsets(ptrSize uintptr) (paramsOff, cwdOff, cmdOff uintptr) {
	if ptrSize == 8 {
		return 0x20, 0x38, 0x70
	}
	return 0x10, 0x24, 0x40
}

func processPeb(h windows.Handle) (ptrSize, peb uintptr) {
	if unsafe.Sizeof(uintptr(0)) == 8 {
		var wow uint64
		var retLen uint32
		err := windows.NtQueryInformationProcess(h, processWow64Information, unsafe.Pointer(&wow), 8, &retLen)
		if err == nil && wow != 0 {
			return 4, uintptr(wow)
		}
	}
	var pbi processBasicInfo
	var retLen uint32
	if err := windows.NtQueryInformationProcess(h, processBasicInformation, unsafe.Pointer(&pbi), uint32(unsafe.Sizeof(pbi)), &retLen); err != nil {
		return 0, 0
	}
	if pbi.PebBaseAddress == 0 {
		return 0, 0
	}
	return unsafe.Sizeof(uintptr(0)), pbi.PebBaseAddress
}

func readRemotePtrSize(h windows.Handle, addr, size uintptr) uintptr {
	buf := make([]byte, size)
	var n uintptr
	if err := windows.ReadProcessMemory(h, addr, &buf[0], size, &n); err != nil || n != size {
		return 0
	}
	if size == 8 {
		return uintptr(binary.LittleEndian.Uint64(buf))
	}
	return uintptr(binary.LittleEndian.Uint32(buf))
}

func readRemoteUnicode(h windows.Handle, addr, ptrSize uintptr) string {
	hdrSize := uintptr(8)
	if ptrSize == 8 {
		hdrSize = 16
	}
	hdr := make([]byte, hdrSize)
	var n uintptr
	if err := windows.ReadProcessMemory(h, addr, &hdr[0], hdrSize, &n); err != nil || n != hdrSize {
		return ""
	}
	length := binary.LittleEndian.Uint16(hdr[0:2])
	if length == 0 || length%2 != 0 || length > 32768 {
		return ""
	}
	var bufPtr uintptr
	if ptrSize == 8 {
		bufPtr = uintptr(binary.LittleEndian.Uint64(hdr[8:16]))
	} else {
		bufPtr = uintptr(binary.LittleEndian.Uint32(hdr[4:8]))
	}
	if bufPtr == 0 {
		return ""
	}
	u16 := make([]uint16, int(length/2))
	if err := windows.ReadProcessMemory(h, bufPtr, (*byte)(unsafe.Pointer(&u16[0])), uintptr(length), &n); err != nil || n != uintptr(length) {
		return ""
	}
	return windows.UTF16ToString(u16)
}

// queryCmdlineCaller uses ProcessCommandLineInformation. That buffer is a
// UNICODE_STRING in the *calling* process's pointer size, not the target's.
// A 64-bit lsoff therefore always parses 64-bit layout, even when the target
// is WOW64. Target-bitness cmdline is read from the PEB in queryPebStrings.
func queryCmdlineCaller(h windows.Handle) string {
	ptrSize := int(unsafe.Sizeof(uintptr(0)))
	hdrMin := uint32(8)
	if ptrSize == 8 {
		hdrMin = 16
	}
	var retLen uint32
	_ = windows.NtQueryInformationProcess(h, processCommandLineInformation, nil, 0, &retLen)
	if retLen < hdrMin {
		return ""
	}
	buf := make([]byte, retLen)
	if err := windows.NtQueryInformationProcess(h, processCommandLineInformation, unsafe.Pointer(&buf[0]), retLen, &retLen); err != nil {
		return ""
	}
	if retLen < hdrMin || int(retLen) > len(buf) {
		return ""
	}
	return decodeUNICODEInBuffer(buf[:retLen], ptrSize)
}

func decodeUNICODEInBuffer(buf []byte, ptrSize int) string {
	hdrSize := 8
	if ptrSize == 8 {
		hdrSize = 16
	}
	if len(buf) < hdrSize {
		return ""
	}
	length := binary.LittleEndian.Uint16(buf[0:2])
	if length == 0 || length%2 != 0 {
		return ""
	}
	n := int(length / 2)
	var ptr uint64
	if ptrSize == 8 {
		if len(buf) < 16 {
			return ""
		}
		ptr = binary.LittleEndian.Uint64(buf[8:16])
	} else {
		ptr = uint64(binary.LittleEndian.Uint32(buf[4:8]))
	}
	base := uint64(uintptr(unsafe.Pointer(&buf[0])))
	end := uint64(length)
	if ptr >= base && ptr+end <= base+uint64(len(buf)) {
		off := int(ptr - base)
		u16 := unsafe.Slice((*uint16)(unsafe.Pointer(&buf[off])), n)
		return strings.TrimSpace(windows.UTF16ToString(u16))
	}
	if hdrSize+int(length) <= len(buf) {
		u16 := unsafe.Slice((*uint16)(unsafe.Pointer(&buf[hdrSize])), n)
		return strings.TrimSpace(windows.UTF16ToString(u16))
	}
	return ""
}

func getExtendedTable[T any](proc *windows.LazyProc, family, tableClass uint32) ([]T, error) {
	var size uint32
	r1, _, _ := proc.Call(0, uintptr(unsafe.Pointer(&size)), 0, uintptr(family), uintptr(tableClass), 0)
	if size == 0 {
		if r1 == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("iphlpapi table size query failed: %d", r1)
	}
	buf := make([]byte, size)
	r1, _, _ = proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0, uintptr(family), uintptr(tableClass), 0)
	if r1 != 0 {
		return nil, fmt.Errorf("iphlpapi table query failed: %d", r1)
	}
	if len(buf) < 4 {
		return nil, nil
	}
	count := binary.LittleEndian.Uint32(buf[:4])
	if count == 0 {
		return nil, nil
	}
	return unsafe.Slice((*T)(unsafe.Pointer(&buf[4])), int(count)), nil
}

func portFromDWORD(v uint32) uint16 {
	return binary.BigEndian.Uint16([]byte{byte(v), byte(v >> 8)})
}

func ipv4String(v uint32) string {
	return normalizeAddr(net.IPv4(byte(v), byte(v>>8), byte(v>>16), byte(v>>24)).String())
}

func ipv6String(b [16]byte) string {
	return normalizeAddr(net.IP(b[:]).String())
}
