package tui

import (
	"sort"
	"strconv"

	"github.com/yutat23/lsoff/internal/listen"
)

type foldState int

const (
	foldNone foldState = iota
	foldCollapsed
	foldExpanded
	foldChild
)

type viewRow struct {
	e      listen.Entry
	fold   foldState
	hidden int
	last   bool
}

func (r viewRow) id() string {
	if r.fold == foldChild {
		return r.e.Key()
	}
	if r.e.PID > 0 && (r.fold == foldCollapsed || r.fold == foldExpanded) {
		return "p/" + strconv.Itoa(r.e.PID)
	}
	return r.e.Key()
}

func (r viewRow) mark() string {
	switch r.fold {
	case foldCollapsed:
		return "▸"
	case foldExpanded:
		return "▾"
	case foldChild:
		if r.last {
			return "└─"
		}
		return "├─"
	default:
		return " "
	}
}

// markCell is mark() padded to the width of the mark column, so a two-cell
// tree connector and a one-cell ▸ / ▾ leave the following columns on the same
// grid.
func (r viewRow) markCell() string {
	return padRight(r.mark(), markWidth)
}

type procBucket struct {
	pid     int
	sockets []listen.Entry
}

func flattenGroups(entries []listen.Entry, key listen.SortKey, desc bool, expanded map[int]bool) []viewRow {
	if len(entries) == 0 {
		return nil
	}
	order := make([]int, 0)
	byPID := make(map[int]*procBucket)
	var zeros []listen.Entry
	for _, e := range entries {
		if e.PID <= 0 {
			zeros = append(zeros, e)
			continue
		}
		b, ok := byPID[e.PID]
		if !ok {
			b = &procBucket{pid: e.PID}
			byPID[e.PID] = b
			order = append(order, e.PID)
		}
		b.sockets = append(b.sockets, e)
	}

	groups := make([]procBucket, 0, len(order)+len(zeros))
	for _, pid := range order {
		b := byPID[pid]
		listen.SortBy(b.sockets, key, desc)
		groups = append(groups, *b)
	}
	for _, e := range zeros {
		groups = append(groups, procBucket{sockets: []listen.Entry{e}})
	}
	sortGroups(groups, key, desc)

	out := make([]viewRow, 0, len(entries))
	for _, g := range groups {
		if len(g.sockets) == 1 {
			out = append(out, viewRow{e: g.sockets[0], fold: foldNone})
			continue
		}
		if expanded[g.pid] {
			out = append(out, viewRow{e: g.sockets[0], fold: foldExpanded})
			for i, e := range g.sockets {
				if i == 0 {
					continue
				}
				out = append(out, viewRow{e: e, fold: foldChild, last: i == len(g.sockets)-1})
			}
			continue
		}
		out = append(out, viewRow{e: g.sockets[0], fold: foldCollapsed, hidden: len(g.sockets) - 1})
	}
	return out
}

// sortGroups orders groups by their representative socket. It sorts the slice
// itself rather than sorting representatives and rebuilding the slice from a
// map keyed by Entry.Key(): two groups can share a key (PID 0 rows on the same
// proto/addr/port), and keying by it made one group appear twice while the
// other vanished.
func sortGroups(groups []procBucket, key listen.SortKey, desc bool) {
	sort.SliceStable(groups, func(i, j int) bool {
		return repLess(groups[i].sockets[0], groups[j].sockets[0], key, desc)
	})
}

// repLess reports whether a sorts strictly before b, deferring to
// listen.SortBy so group order always matches the row order inside a group.
// listen.SortBy is stable, so sorting the pair as [b, a] only puts a first when
// a is strictly smaller; equal-comparing entries keep their input order and
// repLess reports false, which keeps sort.SliceStable stable for ties.
func repLess(a, b listen.Entry, key listen.SortKey, desc bool) bool {
	if a == b {
		return false
	}
	pair := []listen.Entry{b, a}
	listen.SortBy(pair, key, desc)
	return pair[0] == a
}
