package layout

import (
	"golang.org/x/net/html"

	"github.com/npillmayer/css-box-layout/text"
)

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
func buildBlockContainer(r *RenderNode, box BoxKind) (*LayoutNode, error) {
	if r == nil {
		return nil, nil
	}
	flow := make([]FlowItem, 0, len(r.Children()))
	for _, child := range r.Children() {
		items, err := buildInlineFlow(child)
		if err != nil {
			return nil, err
		}
		flow = append(flow, items...)
	}
	children, err := normalizeBlockChildren(flow)
	if err != nil {
		return nil, err
	}
	return &LayoutNode{
		ID:       r.ID,
		Box:      box,
		FC:       FCBlock,
		Children: children,
	}, nil
}

// Builds an inline-level subtree, but may return hoisted blocks as FlowBlock items:
func buildInlineFlow(r *RenderNode) ([]FlowItem, error) {
	if r == nil {
		return nil, nil
	}

	display := r.ComputedStyle("display")
	if display == "none" {
		return nil, nil
	}

	if isTextNode(r.HTMLNode()) {
		text := buildText(r)
		if text == nil {
			return nil, nil
		}
		return []FlowItem{InlineItem(text)}, nil
	}

	if display == "" {
		display = "inline"
	}

	switch display {
	case "inline":
		flow := make([]FlowItem, 0, len(r.Children()))
		for _, child := range r.Children() {
			items, err := buildInlineFlow(child)
			if err != nil {
				return nil, err
			}
			flow = append(flow, items...)
		}
		if containsBlockFlow(flow) {
			proto := &LayoutNode{ID: r.ID, Box: BoxInline, FC: FCInline}
			return wrapInlineRunsForElement(proto, flow), nil
		}
		return []FlowItem{InlineItem(&LayoutNode{
			ID:       r.ID,
			Box:      BoxInline,
			FC:       FCInline,
			Children: inlineChildren(flow),
		})}, nil
	case "inline-block":
		node, err := buildBlockContainer(r, BoxInlineBlock)
		if err != nil {
			return nil, err
		}
		return []FlowItem{InlineItem(node)}, nil
	case "block":
		node, err := buildBlockContainer(r, BoxBlock)
		if err != nil {
			return nil, err
		}
		return []FlowItem{BlockItem(node)}, nil
	default:
		return nil, errNotImplemented
	}
}

func buildText(r *RenderNode) *LayoutNode { // returns BoxText leaf
	if r == nil || r.HTMLNode() == nil {
		return nil
	}
	data := r.HTMLNode().Data
	if data == "" {
		return nil
	}
	return &LayoutNode{
		ID:  r.ID,
		Box: BoxText,
		Text: text.TextRef{
			Source: 0,
			Range:  text.TextRange{Start: 0, End: uint64(len(data))},
		},
	}
}

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
func wrapInlineRunsForElement(proto *LayoutNode, flow []FlowItem) []FlowItem {
	if proto == nil {
		return nil
	}
	out := make([]FlowItem, 0, len(flow))
	run := make([]*LayoutNode, 0, len(flow))

	flush := func() {
		if len(run) == 0 {
			return
		}
		out = append(out, InlineItem(&LayoutNode{
			ID:       proto.ID,
			Box:      proto.Box,
			FC:       proto.FC,
			Children: append([]*LayoutNode(nil), run...),
		}))
		run = run[:0]
	}

	for _, item := range flow {
		if item.Kind == FlowInline {
			run = append(run, item.Node)
			continue
		}
		flush()
		out = append(out, item)
	}
	flush()
	return out
}


func isTextNode(n *html.Node) bool {
	return n != nil && n.Type == html.TextNode
}

func containsBlockFlow(flow []FlowItem) bool {
	for _, item := range flow {
		if item.Kind == FlowBlock {
			return true
		}
	}
	return false
}

func inlineChildren(flow []FlowItem) []*LayoutNode {
	children := make([]*LayoutNode, 0, len(flow))
	for _, item := range flow {
		if item.Kind == FlowInline {
			children = append(children, item.Node)
		}
	}
	return children
}
