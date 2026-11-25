package layout

import (
	"fmt"
	"unicode/utf8"

	"golang.org/x/net/html"
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
func buildBlockContainer(gen *treeBuilder, r *RenderNode, box BoxKind, boxID BoxID) (*LayoutNode, error) {
	return buildBlockContainerWithFont(gen, r, box, boxID, 0, WhiteSpaceNormal)
}

func buildBlockContainerWithFont(gen *treeBuilder, r *RenderNode, box BoxKind, boxID BoxID, inheritedFont float32, inheritedWhitespace WhiteSpace) (*LayoutNode, error) {
	if r == nil {
		return nil, nil
	}
	if r.Replaced != nil {
		return nil, buildError(r.ID, fmt.Errorf("replaced leaf cannot be a block container"))
	}
	style, err := buildStyle(r, inheritedFont, inheritedWhitespace)
	if err != nil {
		return nil, err
	}
	return buildBlockContainerWithStyle(gen, r, box, boxID, style)
}

func buildBlockContainerWithStyle(gen *treeBuilder, r *RenderNode, box BoxKind, boxID BoxID, style *ComputedStyle) (*LayoutNode, error) {
	flow := make([]FlowItem, 0, len(r.Children()))
	for _, child := range r.Children() {
		items, err := buildInlineFlowWithFont(gen, child, boxID, style.FontSizePx, style.WhiteSpace)
		if err != nil {
			return nil, err
		}
		flow = append(flow, items...)
	}
	children, err := normalizeBlockChildren(gen.boxIDGen, flow, boxID)
	if err != nil {
		return nil, buildError(r.ID, err)
	}
	setAnonymousStyles(children, style.FontSizePx, style.WhiteSpace)
	return &LayoutNode{
		Style:          style,
		BoxID:          boxID,
		NodeID:         r.ID,
		Box:            box,
		FC:             FCBlock,
		EstablishesBFC: r.Display.Inner == InnerFlowRoot || style.Position == PositionFixed,
		Children:       children,
	}, nil
}

// Builds an inline-level subtree, but may return hoisted blocks as FlowBlock items:
func buildInlineFlow(gen *treeBuilder, r *RenderNode, parentBoxID BoxID) ([]FlowItem, error) {
	return buildInlineFlowWithFont(gen, r, parentBoxID, 0, WhiteSpaceNormal)
}

func buildInlineFlowWithFont(gen *treeBuilder, r *RenderNode, parentBoxID BoxID, inheritedFont float32, inheritedWhitespace WhiteSpace) (result []FlowItem, failure error) {
	defer func() {
		if gen.trace != nil && r != nil && failure == nil {
			for _, item := range result {
				gen.trace.record(r.ID, item.Node)
			}
		}
	}()

	if r == nil {
		return nil, nil
	}

	if err := validateRenderDisplay(r); err != nil {
		return nil, err
	}
	display := r.Display
	if display.Kind == DisplayNone {
		return nil, nil
	}

	style, err := buildStyle(r, inheritedFont, inheritedWhitespace)
	if err != nil {
		return nil, err
	}
	if isTextNode(r.HTMLNode()) {
		text, err := buildText(r, gen.options, style.WhiteSpace)
		if err != nil {
			return nil, err
		}
		if text == nil {
			return nil, nil
		}
		text.Style = style
		text.BoxID = gen.newChild(parentBoxID)
		text.NodeID = r.ID
		return []FlowItem{InlineItem(text)}, nil
	}

	if style.Position == PositionFixed {
		if err := gen.divertFixed(r, gen.newChild(parentBoxID), style); err != nil {
			return nil, err
		}
		return nil, nil
	}

	if r.Replaced != nil {
		if display.Outer != OuterInline || len(r.Children()) != 0 || style.Width.Kind != LenPx || !finite(style.Width.Value) || style.Width.Value < 0 || !finite(r.Replaced.Height) || r.Replaced.Height < 0 {
			return nil, buildError(r.ID, fmt.Errorf("replaced leaf requires inline display, no children and finite nonnegative fixed dimensions"))
		}
		size := *r.Replaced
		return []FlowItem{InlineItem(&LayoutNode{BoxID: gen.newChild(parentBoxID), NodeID: r.ID, Box: BoxInlineBlock, Style: style, Replaced: &size})}, nil
	}
	switch display {
	case Display{Kind: DisplayBox, Outer: OuterInline, Inner: InnerFlow}:
		flow := make([]FlowItem, 0, len(r.Children()))
		for _, child := range r.Children() {
			items, err := buildInlineFlowWithFont(gen, child, parentBoxID, style.FontSizePx, style.WhiteSpace)
			if err != nil {
				return nil, err
			}
			flow = append(flow, items...)
		}
		if containsBlockFlow(flow) {
			proto := &LayoutNode{Style: style, NodeID: r.ID, Box: BoxInline, FC: FCInline}
			return wrapInlineRunsForElement(gen.boxIDGen, proto, flow, parentBoxID), nil
		}
		boxID := gen.newChild(parentBoxID)
		return []FlowItem{InlineItem(&LayoutNode{
			Style:    style,
			BoxID:    boxID,
			NodeID:   r.ID,
			Box:      BoxInline,
			FC:       FCInline,
			Children: inlineChildren(flow),
		})}, nil
	case Display{Kind: DisplayBox, Outer: OuterInline, Inner: InnerFlowRoot}:
		boxID := gen.newChild(parentBoxID)
		node, err := buildBlockContainerWithStyle(gen, r, BoxInlineBlock, boxID, style)
		if err != nil {
			return nil, err
		}
		return []FlowItem{InlineItem(node)}, nil
	case Display{Kind: DisplayBox, Outer: OuterBlock, Inner: InnerFlow},
		Display{Kind: DisplayBox, Outer: OuterBlock, Inner: InnerFlowRoot}:
		boxID := gen.newChild(parentBoxID)
		node, err := buildBlockContainerWithStyle(gen, r, BoxBlock, boxID, style)
		if err != nil {
			return nil, err
		}
		return []FlowItem{BlockItem(node)}, nil
	default:
		return nil, buildError(r.ID, fmt.Errorf("%w: %s", ErrUnsupportedDisplay, display))
	}
}

func buildText(r *RenderNode, opts BuildOptions, whitespace WhiteSpace) (*LayoutNode, error) {
	if r == nil || r.HTMLNode() == nil {
		return nil, nil
	}
	if r.Text == nil {
		if r.HTMLNode().Data == "" {
			return nil, nil
		}
		return nil, buildError(r.ID, fmt.Errorf("nonempty text requires a source TextRef"))
	}
	ref := *r.Text
	if ref.Range.End < ref.Range.Start {
		return nil, buildError(r.ID, fmt.Errorf("reversed text range"))
	}
	if ref.Range.End == ref.Range.Start {
		return nil, nil
	}
	content := r.HTMLNode().Data
	if opts.ReadText != nil {
		var err error
		content, err = opts.ReadText(ref)
		if err != nil {
			return nil, buildError(r.ID, fmt.Errorf("read text: %w", err))
		}
	}
	if uint64(len(content)) != ref.Range.End-ref.Range.Start || !utf8.ValidString(content) {
		return nil, buildError(r.ID, fmt.Errorf("text reader must supply the referenced UTF-8 range"))
	}
	return &LayoutNode{Box: BoxText, Text: ref, collapsibleText: whitespace.collapsesEntirely(content)}, nil
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
func normalizeBlockChildren(gen *boxIDGen, flow []FlowItem, parentBoxID BoxID) ([]*LayoutNode, error) {
	if len(flow) == 0 {
		return []*LayoutNode{wrapInAnonymousInline(gen, parentBoxID, nil)}, nil
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
		return []*LayoutNode{wrapInAnonymousInline(gen, parentBoxID, inlines)}, nil
	}

	children := make([]*LayoutNode, 0, len(flow))
	inlineRun := make([]*LayoutNode, 0, len(flow))
	flushInlineRun := func() {
		if len(inlineRun) == 0 {
			return
		}
		if onlyCollapsibleInlineRun(inlineRun) {
			inlineRun = inlineRun[:0]
			return
		}
		// Retained runs must not share the buffer reused for subsequent children.
		run := append([]*LayoutNode(nil), inlineRun...)
		children = append(children, wrapInlineRunAsAnonymousBlock(gen, parentBoxID, run))
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

// === Helpers ==========================================================

func wrapInAnonymousInline(gen *boxIDGen, parentBoxID BoxID, inlines []*LayoutNode) *LayoutNode {
	return &LayoutNode{
		BoxID:    gen.newChild(parentBoxID),
		NodeID:   0,
		Box:      BoxAnonymousInline,
		FC:       FCInline,
		Children: inlines,
	}
}

func wrapInlineRunAsAnonymousBlock(gen *boxIDGen, parentBoxID BoxID, inlines []*LayoutNode) *LayoutNode {
	anonBlockID := gen.newChild(parentBoxID)
	ai := wrapInAnonymousInline(gen, anonBlockID, inlines)
	return &LayoutNode{
		BoxID:    anonBlockID,
		NodeID:   0,
		Box:      BoxAnonymousBlock,
		FC:       FCBlock,
		Children: []*LayoutNode{ai},
	}
}

// For split+hoist: take mixed flow returned from building an inline element’s children
// and wrap each inline run inside a BoxInline for that element (same ID).
func wrapInlineRunsForElement(gen *boxIDGen, proto *LayoutNode, flow []FlowItem, parentBoxID BoxID) []FlowItem {
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
			BoxID:    gen.newChild(parentBoxID),
			NodeID:   proto.NodeID,
			Style:    proto.Style,
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

// Suppress only complete inline runs in mixed block/inline flow. Inline-only
// containers (including explicitly empty paragraphs) keep their normal group.
func onlyCollapsibleInlineRun(nodes []*LayoutNode) bool {
	for _, n := range nodes {
		switch n.Box {
		case BoxText:
			if !n.collapsibleText {
				return false
			}
		case BoxInline:
			if hasInlineEdges(n.Style) || !onlyCollapsibleInlineRun(n.Children) {
				return false
			}
		default:
			return false // Atomics remain significant even at zero size.
		}
	}
	return true
}
