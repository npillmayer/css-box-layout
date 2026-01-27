package layout

type FlowKind uint8

const (
	FlowInline FlowKind = iota
	FlowBlock
)

type FlowItem struct {
	Kind FlowKind
	Node *LayoutNode
}

func InlineItem(n *LayoutNode) FlowItem { return FlowItem{Kind: FlowInline, Node: n} }
func BlockItem(n *LayoutNode) FlowItem  { return FlowItem{Kind: FlowBlock, Node: n} }

// Entry point for a block container (BoxBlock / BoxAnonymousBlock / BoxInlineBlock):
func buildBlockContainer(r *RenderNode, box BoxKind) (*LayoutNode, error)

// Builds an inline-level subtree, but may return hoisted blocks as FlowBlock items:
func buildInlineFlow(r *RenderNode) ([]FlowItem, error)

func buildText(r *RenderNode) *LayoutNode // returns BoxText leaf

/*
* Behavior of normalizeBlockChildren

A. If flow contains only FlowBlock items:
  - return their nodes directly

B. If it contains only FlowInline items:
  - return exactly one BoxAnonymousInline containing all inline nodes

C. If it contains both:
 1. scan runs; for each maximal run of inline items:
    1.1 create BoxAnonymousBlock
    1.1.1 create BoxAnonymousInline containing the inline nodes
    1.2 -append that anonymous block as a child
 2. append block items as-is in order

This single function enforces the core invariant across:
- normal blocks (BoxBlock)
- anonymous blocks (BoxAnonymousBlock)
- inline-block containers (BoxInlineBlock) internally
*/
func normalizeBlockChildren(flow []FlowItem) ([]*LayoutNode, error) {
	if len(flow) == 0 {
		return nil, nil
	}

	hasBlock := false
	hasInline := false
	for _, item := range flow {
		if item.Kind == FlowBlock {
			hasBlock = true
		} else {
			hasInline = true
		}
	}

	if hasBlock && !hasInline {
		children := make([]*LayoutNode, 0, len(flow))
		for _, item := range flow {
			children = append(children, item.Node)
		}
		return children, nil
	}

	if hasInline && !hasBlock {
		inlines := make([]*LayoutNode, 0, len(flow))
		for _, item := range flow {
			inlines = append(inlines, item.Node)
		}
		return []*LayoutNode{wrapInAnonymousInline(inlines)}, nil
	}

	children := make([]*LayoutNode, 0, len(flow))
	inlineRun := make([]*LayoutNode, 0, len(flow))
	flushInlineRun := func() {
		if len(inlineRun) == 0 {
			return
		}
		children = append(children, wrapInlineRunAsAnonymousBlock(inlineRun))
		inlineRun = inlineRun[:0]
	}

	for _, item := range flow {
		if item.Kind == FlowInline {
			inlineRun = append(inlineRun, item.Node)
			continue
		}
		flushInlineRun()
		children = append(children, item.Node)
	}
	flushInlineRun()

	return children, nil
}

type ContainingBlock struct {
	Width  Constraint
	Height Constraint
}

type ConstraintKind uint8

const (
	ConstraintIndefinite ConstraintKind = iota
	ConstraintDefinite
)

type Constraint struct {
	Kind  ConstraintKind
	Value float32
}

type ResolveCtx struct {
	AvailableWidth float32
	FontSizePx     float32
}

func resolveLength(l Length, ctx ResolveCtx) (px float32, isAuto bool) {
	switch l.Kind {
	case LenPx:
		return l.Value, false
	case LenPercent:
		return ctx.AvailableWidth * l.Value, false // assume l.Value is 0..1
	case LenEm:
		return ctx.FontSizePx * l.Value, false
	case LenAuto:
		return 0, true
	default:
		return 0, true
	}
}

// === Helpers ==========================================================

func wrapInAnonymousInline(inlines []*LayoutNode) *LayoutNode {
	return &LayoutNode{
		ID: 0, Box: BoxAnonymousInline, FC: FCInline,
		Children: inlines,
	}
}

func wrapInlineRunAsAnonymousBlock(inlines []*LayoutNode) *LayoutNode {
	ai := wrapInAnonymousInline(inlines)
	return &LayoutNode{
		ID: 0, Box: BoxAnonymousBlock, FC: FCBlock,
		Children: []*LayoutNode{ai},
	}
}

// For split+hoist: take mixed flow returned from building an inline element’s children
// and wrap each inline run inside a BoxInline for that element (same ID).
func wrapInlineRunsForElement(proto *LayoutNode, flow []FlowItem) []FlowItem
