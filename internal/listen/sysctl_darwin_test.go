//go:build darwin

package listen

import "testing"

func TestMergeUnownedAddsPIDZeroRowsOnly(t *testing.T) {
	libproc := []Entry{
		{Proto: TCP, Port: 5000, Addr: "0.0.0.0", PID: 1098, Name: "ControlCenter", Start: 42},
		{Proto: TCP, Port: 5000, Addr: "::", PID: 1098, Name: "ControlCenter", Start: 42},
		{Proto: UDP, Port: 5353, Addr: "0.0.0.0", PID: 973, Name: "rapportd", Start: 7},
	}
	socks := []rawSocket{
		// Already attributed: must not gain a duplicate PID-0 row.
		{Proto: TCP, Port: 5000, Addr: "0.0.0.0"},
		{Proto: TCP, Port: 5000, Addr: "::"},
		{Proto: UDP, Port: 5353, Addr: "0.0.0.0"},
		// Owned by root, invisible to libproc.
		{Proto: TCP, Port: 22, Addr: "0.0.0.0"},
		{Proto: TCP, Port: 22, Addr: "::"},
		{Proto: UDP, Port: 137, Addr: "0.0.0.0"},
		// Two root processes on the same wildcard port: one row is enough.
		{Proto: UDP, Port: 137, Addr: "0.0.0.0"},
		// Same port, different proto and different addr: distinct rows.
		{Proto: UDP, Port: 22, Addr: "0.0.0.0"},
		{Proto: TCP, Port: 22, Addr: "127.0.0.1"},
	}

	got := mergeUnowned(libproc, socks)

	want := map[string]int{ // key -> expected PID
		"tcp/0.0.0.0/5000": 1098,
		"tcp/::/5000":      1098,
		"udp/0.0.0.0/5353": 973,
		"tcp/0.0.0.0/22":   0,
		"tcp/::/22":        0,
		"udp/0.0.0.0/137":  0,
		"udp/0.0.0.0/22":   0,
		"tcp/127.0.0.1/22": 0,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	seen := make(map[string]bool, len(got))
	for _, e := range got {
		k := socketKey(e.Proto, e.Addr, e.Port)
		pid, ok := want[k]
		if !ok {
			t.Errorf("unexpected entry %s", k)
			continue
		}
		if seen[k] {
			t.Errorf("duplicate entry %s", k)
		}
		seen[k] = true
		if e.PID != pid {
			t.Errorf("%s: PID = %d, want %d", k, e.PID, pid)
		}
		if e.PID == 0 {
			// PID 0 rows carry no process data, and Start 0 makes Kill refuse them.
			if e.Name != "" || e.Path != "" || e.Cmdline != "" || e.Cwd != "" || e.Project != "" {
				t.Errorf("%s: unattributed row has process details: %+v", k, e)
			}
			if e.Start != 0 {
				t.Errorf("%s: unattributed row has Start %d, want 0", k, e.Start)
			}
		}
	}
}

func TestMergeUnownedKeepsLibprocRowsWhenSysctlEmpty(t *testing.T) {
	libproc := []Entry{{Proto: TCP, Port: 8080, Addr: "127.0.0.1", PID: 5, Name: "x"}}
	got := mergeUnowned(libproc, nil)
	if len(got) != 1 || got[0].PID != 5 {
		t.Fatalf("got %+v, want the single libproc row untouched", got)
	}
}

// TestListSocketsWalksPCBLists guards the sysctl walk itself: it must terminate,
// and every socket it reports must be a usable proto/addr/port triple.
func TestListSocketsWalksPCBLists(t *testing.T) {
	socks, err := listSockets()
	if err != nil {
		t.Fatalf("listSockets: %v", err)
	}
	if len(socks) == 0 {
		t.Skip("no listening sockets on this host")
	}
	for _, s := range socks {
		if s.Port == 0 {
			t.Errorf("socket with port 0: %+v", s)
		}
		if s.Addr == "" {
			t.Errorf("socket with empty addr: %+v", s)
		}
		if s.Addr != normalizeAddr(s.Addr) {
			t.Errorf("addr %q is not normalized", s.Addr)
		}
	}
}

// TestListIncludesUnprivilegedSockets checks that List reports at least the
// sockets sysctl can see, including those owned by processes this user cannot
// inspect.
func TestListIncludesUnprivilegedSockets(t *testing.T) {
	before, err := listSockets()
	if err != nil {
		t.Skipf("listSockets: %v", err)
	}
	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	after, err := listSockets()
	if err != nil {
		t.Skipf("listSockets: %v", err)
	}

	// Sockets open and close while the test runs, so only require the ones
	// that existed both before and after the List call.
	stable := make(map[string]struct{}, len(before))
	seenBefore := make(map[string]struct{}, len(before))
	for _, s := range before {
		seenBefore[socketKey(s.Proto, s.Addr, s.Port)] = struct{}{}
	}
	for _, s := range after {
		k := socketKey(s.Proto, s.Addr, s.Port)
		if _, ok := seenBefore[k]; ok {
			stable[k] = struct{}{}
		}
	}
	if len(stable) == 0 {
		t.Skip("no stable listening sockets on this host")
	}

	have := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		have[socketKey(e.Proto, e.Addr, e.Port)] = struct{}{}
	}
	for k := range stable {
		if _, ok := have[k]; !ok {
			t.Errorf("List is missing socket %s", k)
		}
	}
}
