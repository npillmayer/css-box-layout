package layout

type boxIDGen struct {
	next map[BoxID]uint32
}

func newBoxIDGen() *boxIDGen {
	return &boxIDGen{next: make(map[BoxID]uint32)}
}

func (g *boxIDGen) newRoot(nodeID NodeID) BoxID {
	return g.makeBoxID(0, uint64(nodeID))
}

func (g *boxIDGen) newChild(parent BoxID) BoxID {
	idx := g.next[parent]
	g.next[parent] = idx + 1
	return g.makeBoxID(parent, uint64(idx))
}

func (g *boxIDGen) makeBoxID(parent BoxID, salt uint64) BoxID {
	return parent*1103515245 + BoxID(salt+1)
}
