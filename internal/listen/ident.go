package listen

import "errors"

// Ident is a process identity captured at list time. PID alone is not enough
// because the kernel can reuse PIDs.
type Ident struct {
	PID   int
	Start uint64
}

// Ident returns the kill identity captured for this row.
func (e Entry) Ident() Ident {
	return Ident{PID: e.PID, Start: e.Start}
}

var (
	errNoIdentity       = errors.New("missing process identity; refusing to kill")
	errIdentityMismatch = errors.New("process identity changed (pid reused?)")
)

// UniqueIdents returns distinct positive PIDs in first-seen order, with the
// start token captured for that row.
func UniqueIdents(entries []Entry) []Ident {
	seen := make(map[int]struct{}, len(entries))
	out := make([]Ident, 0, len(entries))
	for _, e := range entries {
		if e.PID <= 0 {
			continue
		}
		if _, ok := seen[e.PID]; ok {
			continue
		}
		seen[e.PID] = struct{}{}
		out = append(out, e.Ident())
	}
	return out
}
