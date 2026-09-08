package tui

import (
	"fmt"
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
	e       listen.Entry
	group   groupKey
	grouped bool
	fold    foldState
	hidden  int
	last    bool
}

func (r viewRow) id() string {
	if r.fold == foldChild {
		return r.e.Key()
	}
	if r.grouped {
		return "g/" + r.group.String()
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

type ownerKind uint8

const (
	ownerProcess ownerKind = iota
	ownerDocker
	ownerAnonymous
)

type logicalOwner struct {
	kind ownerKind
	id   string
}

type groupKey struct {
	owner logicalOwner
	proto listen.Proto
	port  uint16
}

func (k groupKey) String() string {
	return fmt.Sprintf("%d/%s/%s/%d", k.owner.kind, k.owner.id, k.proto, k.port)
}

type listenerGroup struct {
	key     groupKey
	sockets []listen.Entry
}

func flattenGroups(entries []listen.Entry, key listen.SortKey, desc bool, expanded map[groupKey]bool) []viewRow {
	if len(entries) == 0 {
		return nil
	}
	order := make([]groupKey, 0)
	byKey := make(map[groupKey]*listenerGroup)
	for i, e := range entries {
		group := groupKeyForEntry(e, i)
		b, ok := byKey[group]
		if !ok {
			b = &listenerGroup{key: group}
			byKey[group] = b
			order = append(order, group)
		}
		b.sockets = append(b.sockets, e)
	}

	groups := make([]listenerGroup, 0, len(order))
	for _, groupID := range order {
		b := byKey[groupID]
		listen.SortBy(b.sockets, key, desc)
		groups = append(groups, *b)
	}
	sortGroups(groups, key, desc)

	out := make([]viewRow, 0, len(entries))
	for _, g := range groups {
		if len(g.sockets) == 1 {
			out = append(out, viewRow{e: g.sockets[0], group: g.key, grouped: isGroupable(g.key), fold: foldNone})
			continue
		}
		if expanded[g.key] {
			out = append(out, viewRow{e: g.sockets[0], group: g.key, grouped: true, fold: foldExpanded})
			for i, e := range g.sockets {
				if i == 0 {
					continue
				}
				out = append(out, viewRow{e: e, group: g.key, grouped: true, fold: foldChild, last: i == len(g.sockets)-1})
			}
			continue
		}
		out = append(out, viewRow{e: g.sockets[0], group: g.key, grouped: true, fold: foldCollapsed, hidden: len(g.sockets) - 1})
	}
	return out
}

// sortGroups orders logical groups by their representative socket. It sorts
// the slice itself rather than rebuilding it from a map, so distinct owners
// with otherwise similar entries remain distinct.
func sortGroups(groups []listenerGroup, key listen.SortKey, desc bool) {
	sort.SliceStable(groups, func(i, j int) bool {
		return repLess(groups[i].sockets[0], groups[j].sockets[0], key, desc)
	})
}

func groupKeyForEntry(e listen.Entry, index int) groupKey {
	owner := logicalOwner{kind: ownerAnonymous, id: strconv.Itoa(index)}
	if e.PID > 0 {
		owner = logicalOwner{kind: ownerProcess, id: strconv.Itoa(e.PID)}
	} else if e.Source == listen.SourceDocker && e.ContainerID != "" {
		owner = logicalOwner{kind: ownerDocker, id: e.ContainerID}
	}
	return groupKey{owner: owner, proto: e.Proto, port: e.Port}
}

func isGroupable(key groupKey) bool {
	return key.owner.kind != ownerAnonymous
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
