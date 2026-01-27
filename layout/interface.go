package layout

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
