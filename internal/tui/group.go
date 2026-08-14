package tui

import (
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
	default:
		return " "
	}
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
			for _, e := range g.sockets[1:] {
				out = append(out, viewRow{e: e, fold: foldChild})
			}
			continue
		}
		out = append(out, viewRow{e: g.sockets[0], fold: foldCollapsed, hidden: len(g.sockets) - 1})
	}
	return out
}

func sortGroups(groups []procBucket, key listen.SortKey, desc bool) {
	reps := make([]listen.Entry, len(groups))
	byKey := make(map[string]procBucket, len(groups))
	for i, g := range groups {
		reps[i] = g.sockets[0]
		byKey[g.sockets[0].Key()] = g
	}
	listen.SortBy(reps, key, desc)
	for i, e := range reps {
		groups[i] = byKey[e.Key()]
	}
}
