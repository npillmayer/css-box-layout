package layout

import "errors"

var errBoxIDExhausted = errors.New("layout: BoxID space exhausted")

// IDs are unique within a build and deterministic for the same allocation order.
// Structural edits may change IDs; NodeID is the source identity across edits.
type boxIDGen struct {
	next      map[BoxID]uint64
	allocated map[BoxID]struct{}
	err       error
}

func newBoxIDGen() *boxIDGen {
	return &boxIDGen{
		next:      make(map[BoxID]uint64),
		allocated: make(map[BoxID]struct{}),
	}
}

func (g *boxIDGen) newRoot(nodeID NodeID) BoxID {
	return g.makeBoxID(0, uint64(nodeID))
}

func (g *boxIDGen) newChild(parent BoxID) BoxID {
	idx := g.next[parent]
	g.next[parent] = idx + 1
	return g.makeBoxID(parent, idx)
}

func (g *boxIDGen) makeBoxID(parent BoxID, salt uint64) BoxID {
	if g.err != nil {
		return 0
	}
	candidate := parent*1103515245 + BoxID(salt+1)
	if candidate == 0 {
		candidate = 1
	}
	start := candidate
	for {
		if _, exists := g.allocated[candidate]; !exists {
			g.allocated[candidate] = struct{}{}
			return candidate
		}
		// Arithmetic and ordinal wraparound are safe: every candidate is checked.
		// Reserve zero for invalid/unassigned identities.
		candidate++
		if candidate == 0 {
			candidate = 1
		}
		if candidate == start {
			g.err = errBoxIDExhausted
			return 0
		}
	}
}
