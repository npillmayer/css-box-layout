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
// valid). Used and Atomic are scoped to this layout invocation; input tables must
// not be mutated while it runs. The adapter supplies its own source/style lookup.
type InlineRequest struct {
	Group        *LayoutNode // BoxAnonymousInline; Group.BoxID is the retention key
	OwnerID      BoxID
	ContentWidth float32
	Used         UsedValuesView
	Atomic       AtomicSizer
}

// InlineLayouter returns only extent. The adapter retains typed paragraph results
// by group BoxID with OwnerID in invocation-local storage, published with document
// geometry only after successful layout. Lines, glyphs and baselines stay there.
// Repeated/speculative requests need an acceptance policy before being supported.
type InlineLayouter interface {
	LayoutInline(InlineRequest) (InlineExtent, error)
}

type AtomicSizer interface {
	SizeInlineBlock(node *LayoutNode, maxWidth float32) (w, h float32, err error)
}

type IntrinsicMeasurer interface {
	MaxContentWidth(node *LayoutNode) (float32, error)
}

type InlineIntrinsic interface {
	MaxContentWidth(inlineRoot *LayoutNode) (float32, error)
}
