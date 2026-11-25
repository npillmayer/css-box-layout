package layout

import (
	"fmt"
	"slices"
)

// BoundaryEdge identifies a position immediately before or after a normalized
// box, including its descendants. These are structural positions, not legal
// fragmentation opportunities or text offsets.
type BoundaryEdge uint8

const (
	BoundaryBefore BoundaryEdge = iota
	BoundaryAfter
)

// StructuralBoundary is local to one build. Order is a one-based depth-first
// enter/exit ordinal within Root; compare Order only within the same Root.
// Group is the containing anonymous inline group (or the box itself for a
// group). Atomic interiors have their own groups. Zero denotes no position.
type StructuralBoundary struct {
	Root, Box, Group BoxID
	Edge             BoundaryEdge
	Order            uint64
}

type BoundaryDisposition string

const (
	BoundaryRetained   BoundaryDisposition = "retained"
	BoundaryOmitted    BoundaryDisposition = "omitted"
	BoundaryEmpty      BoundaryDisposition = "empty"
	BoundarySuppressed BoundaryDisposition = "suppressed"
)

// BoundaryMapping explicitly distinguishes missing material from a position.
// Omitted includes whitespace-only runs discarded during block normalization;
// Empty means the source emitted no boxes (for example a zero-length text).
// Neither chooses a nearby box or applies consumer-specific affinity.
type BoundaryMapping struct {
	Disposition BoundaryDisposition
	Position    StructuralBoundary
}

// SourceBoundaries describes an input source's outer boundaries. Occurrences
// lists retained boxes bearing this NodeID in structural order; split inlines
// may have several, and a hoisted-only inline may have none despite retained
// outer boundaries. Before/After enclose its emitted flow, including hoisted
// children. If an endpoint was discarded, that endpoint is explicitly omitted.
type SourceBoundaries struct {
	Source        NodeID
	Before, After BoundaryMapping
	Occurrences   []BoxID
}

// NormalizationProvenance owns detached build-local values, never node pointers.
// Consumers must qualify identities with their own build/tree lifetime.
type NormalizationProvenance struct {
	sources    []SourceBoundaries
	boundaries []StructuralBoundary
}

func (p *NormalizationProvenance) Sources() []SourceBoundaries {
	if p == nil {
		return nil
	}
	out := slices.Clone(p.sources)
	for i := range out {
		out[i].Occurrences = slices.Clone(out[i].Occurrences)
	}
	return out
}

// Boundaries returns structural traversal order: normal-flow root first, then
// fixed roots in source order. Each independent root restarts Order at one.
func (p *NormalizationProvenance) Boundaries() []StructuralBoundary {
	if p == nil {
		return nil
	}
	return slices.Clone(p.boundaries)
}

type normalizationTrace struct {
	sources    []*RenderNode
	suppressed map[NodeID]bool
	emitted    map[NodeID][]BoxID
}

// BuildLayoutTreesWithProvenance performs the same normalization as
// BuildLayoutTrees and also captures source boundaries out of band. Input source
// IDs must be unique, including suppressed descendants. Errors are
// BuildErrors; neither trees nor provenance are published on failure. Sources
// absent from the supplied projection cannot be described by this builder.
// TextRef ranges on retained text boxes remain the authority for byte positions;
// this API neither collapses text bytes nor maps them to prepared paragraphs.
func BuildLayoutTreesWithProvenance(root *RenderNode, opts BuildOptions) (*LayoutTrees, *NormalizationProvenance, error) {
	trace := &normalizationTrace{suppressed: map[NodeID]bool{}, emitted: map[NodeID][]BoxID{}}
	seen := map[NodeID]bool{}
	var visit func(*RenderNode, bool) error
	visit = func(n *RenderNode, hidden bool) error {
		if n == nil {
			return nil
		}
		if seen[n.ID] {
			return buildError(n.ID, fmt.Errorf("provenance requires unique source IDs"))
		}
		seen[n.ID] = true
		hidden = hidden || n.Display.Kind == DisplayNone
		trace.sources = append(trace.sources, n)
		trace.suppressed[n.ID] = hidden
		for _, c := range n.Children() {
			if err := visit(c, hidden); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(root, false); err != nil {
		return nil, nil, buildRootError(root.ID, err)
	}
	trees, err := buildLayoutTrees(root, opts, trace)
	if err != nil {
		return nil, nil, err
	}
	return trees, trace.finish(trees), nil
}

func (t *normalizationTrace) record(id NodeID, nodes ...*LayoutNode) {
	if t == nil {
		return
	}
	for _, n := range nodes {
		if n != nil {
			t.emitted[id] = append(t.emitted[id], n.BoxID)
		}
	}
}

func (t *normalizationTrace) finish(trees *LayoutTrees) *NormalizationProvenance {
	p := &NormalizationProvenance{}
	edges := map[BoxID][2]StructuralBoundary{}
	occurrences := map[NodeID][]BoxID{}
	for _, root := range append([]*LayoutNode{trees.Flow}, trees.Fixed...) {
		if root == nil {
			continue
		}
		var order uint64
		var walk func(*LayoutNode, BoxID)
		walk = func(n *LayoutNode, group BoxID) {
			if n.Box == BoxAnonymousInline {
				group = n.BoxID
			}
			order++
			before := StructuralBoundary{Root: root.BoxID, Box: n.BoxID, Group: group, Edge: BoundaryBefore, Order: order}
			p.boundaries = append(p.boundaries, before)
			if n.Box != BoxAnonymousInline && n.Box != BoxAnonymousBlock {
				occurrences[n.NodeID] = append(occurrences[n.NodeID], n.BoxID)
			}
			childGroup := group
			if n.Box == BoxInlineBlock {
				childGroup = 0
			}
			for _, c := range n.Children {
				walk(c, childGroup)
			}
			order++
			after := StructuralBoundary{Root: root.BoxID, Box: n.BoxID, Group: group, Edge: BoundaryAfter, Order: order}
			p.boundaries = append(p.boundaries, after)
			edges[n.BoxID] = [2]StructuralBoundary{before, after}
		}
		walk(root, 0)
	}
	for _, s := range t.sources {
		r := SourceBoundaries{Source: s.ID, Occurrences: occurrences[s.ID]}
		disposition := BoundaryEmpty
		if t.suppressed[s.ID] {
			disposition = BoundarySuppressed
		}
		r.Before.Disposition = disposition
		r.After.Disposition = disposition
		ids := t.emitted[s.ID]
		if len(ids) > 0 {
			for i, target := range []*BoundaryMapping{&r.Before, &r.After} {
				id := ids[0]
				if i == 1 {
					id = ids[len(ids)-1]
				}
				target.Disposition = BoundaryOmitted
				if pair, ok := edges[id]; ok {
					target.Disposition = BoundaryRetained
					target.Position = pair[i]
				}
			}
		}
		p.sources = append(p.sources, r)
	}
	return p
}
