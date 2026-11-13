package layout

import (
	"golang.org/x/net/html"
)

// StyNodeView is the minimal interface needed by core passes.
// Order is defined by Children() slice order (Rank must remain in sync).
type StyNodeView interface {
	Children() []StyNodeView
	HTMLNode() *html.Node
	ComputedStyle(string) string
}

// InlineExtent is the paragraph's overall layout extent in CSS pixels, not ink
// bounds. H includes paragraph-owned trailing space but excludes owner box edges
// and margins. W may exceed ContentWidth; it does not resize the containing block.
// Both dimensions must be finite and nonnegative. Empty groups may reserve space.
type InlineExtent struct{ W, H float32 }

// UsedValuesView exposes copies of resolved values without exposing the map.
type UsedValuesView interface {
	Lookup(BoxID) (UsedValues, bool)
}

type usedValuesView struct{ table UsedValuesTable }

func (v usedValuesView) Lookup(id BoxID) (UsedValues, bool) {
	u, ok := v.table[id]
	return u, ok
}

// InlineRequest identifies an existing group and its owning block. Group and its
// descendants are read-only. ContentWidth is definite, in CSS pixels (zero is
// valid). Used and Atomics are scoped to this layout invocation; input tables must
// not be mutated while it runs. The adapter supplies its own source/style lookup.
type InlineRequest struct {
	Group        *LayoutNode // BoxAnonymousInline; Group.BoxID is the retention key
	OwnerID      BoxID
	ContentWidth float32
	Used         UsedValuesView
	Atomics      PreparedAtomicsView
}

// InlineLayouter returns extent and placements of core-owned atomic boxes.
// The adapter retains typed paragraph results
// by group BoxID with OwnerID in invocation-local storage, published with document
// geometry only after successful layout. Lines, glyphs and baselines stay there.
// Repeated/speculative requests need an acceptance policy before being supported.
type InlineLayouter interface {
	LayoutInline(InlineRequest) (InlineResult, error)
}

// PreparedAtomic is a completed border box, excluding margins.
type PreparedAtomic struct {
	BoxID                     BoxID
	BorderWidth, BorderHeight float32
}

// PreparedAtomicsView contains this group's atomic boundaries, not their interiors.
// Lookup returns copies. The view is read-only and scoped to this invocation.
type PreparedAtomicsView interface {
	Lookup(BoxID) (PreparedAtomic, bool)
}

type preparedAtomicsView struct{ items map[BoxID]PreparedAtomic }

func (v preparedAtomicsView) Lookup(id BoxID) (PreparedAtomic, bool) {
	item, ok := v.items[id]
	return item, ok
}

// AtomicPlacement is a border origin relative to the requesting owner's content.
// Margin and alignment effects are already included. Negative positions are valid.
type AtomicPlacement struct {
	BoxID BoxID
	X, Y  float32
}

// InlineResult contains no paragraph payload. Every prepared item must be placed
// exactly once, including zero-sized items. The adapter retains typography data.
type InlineResult struct {
	Extent  InlineExtent
	Atomics []AtomicPlacement
}

// IntrinsicWidths are finite CSS-pixel content bounds with 0 <= Min <= Max.
type IntrinsicWidths struct{ MinContent, MaxContent float32 }

// IntrinsicAtomic excludes margins, but includes its own padding and borders.
type IntrinsicAtomic struct {
	BoxID        BoxID
	BorderWidths IntrinsicWidths
}
type IntrinsicAtomicsView interface {
	Lookup(BoxID) (IntrinsicAtomic, bool)
}
type BoxEdges struct{ Margin, Padding, Border Edges }

// ResolvedEdgesView never exposes provisional content widths.
type ResolvedEdgesView interface{ Lookup(BoxID) (BoxEdges, bool) }
type InlineMeasureRequest struct {
	Group   *LayoutNode
	OwnerID BoxID
	Edges   ResolvedEdgesView
	Atomics IntrinsicAtomicsView
}

// IntrinsicMeasurer measures one group without publishing paragraph results or
// geometry. Core supplies nested atomic bounds; no layout callbacks are provided.
type IntrinsicMeasurer interface {
	MeasureInline(InlineMeasureRequest) (IntrinsicWidths, error)
}
