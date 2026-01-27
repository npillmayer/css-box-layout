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

type InlineLayouter interface {
	LayoutInline(
		inlineRoot *LayoutNode, // BoxAnonymousInline
		maxWidth float32,
		atomic AtomicSizer, // callback for atomic inline items (inline-block)
	) ([]LineBox, error)
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
