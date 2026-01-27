package layout

import (
	"testing"

	"golang.org/x/net/html"
)

func newRenderElement(id NodeID, display string, children ...*RenderNode) *RenderNode {
	return &RenderNode{
		ID:     id,
		HTML:   &html.Node{Type: html.ElementNode, Data: "div"},
		Styles: map[string]string{"display": display},
		ChildrenNodes: children,
	}
}

func newRenderText(id NodeID, data string) *RenderNode {
	return &RenderNode{
		ID:   id,
		HTML: &html.Node{Type: html.TextNode, Data: data},
		Styles: map[string]string{"display": "inline"},
	}
}

func TestBuildInlineFlow_SplitAndHoist(t *testing.T) {
	childInline1 := newRenderElement(2, "inline")
	childBlock := newRenderElement(3, "block")
	childInline2 := newRenderElement(4, "inline")
	parent := newRenderElement(1, "inline", childInline1, childBlock, childInline2)

	flow, err := buildInlineFlow(parent)
	if err != nil {
		t.Fatalf("buildInlineFlow returned error: %v", err)
	}
	if len(flow) != 3 {
		t.Fatalf("expected 3 flow items, got %d", len(flow))
	}
	if flow[0].Kind != FlowInline || flow[1].Kind != FlowBlock || flow[2].Kind != FlowInline {
		t.Fatalf("unexpected flow kinds: %v, %v, %v", flow[0].Kind, flow[1].Kind, flow[2].Kind)
	}
	if flow[0].Node.ID != parent.ID || flow[2].Node.ID != parent.ID {
		t.Fatalf("expected inline fragments to keep parent ID")
	}
	if len(flow[0].Node.Children) != 1 || flow[0].Node.Children[0].ID != childInline1.ID {
		t.Fatalf("expected first inline run to contain childInline1")
	}
	if flow[1].Node.ID != childBlock.ID || flow[1].Node.Box != BoxBlock {
		t.Fatalf("expected middle flow item to be block child")
	}
	if len(flow[2].Node.Children) != 1 || flow[2].Node.Children[0].ID != childInline2.ID {
		t.Fatalf("expected second inline run to contain childInline2")
	}
}

func TestBuildInlineFlow_TextNode(t *testing.T) {
	textNode := newRenderText(2, "hello")

	flow, err := buildInlineFlow(textNode)
	if err != nil {
		t.Fatalf("buildInlineFlow returned error: %v", err)
	}
	if len(flow) != 1 {
		t.Fatalf("expected 1 flow item, got %d", len(flow))
	}
	if flow[0].Kind != FlowInline || flow[0].Node.Box != BoxText {
		t.Fatalf("expected inline BoxText flow item")
	}
	if flow[0].Node.Text.Range.End == 0 {
		t.Fatalf("expected non-empty text range")
	}
}

func TestBuildInlineFlow_TextNodeEmpty(t *testing.T) {
	textNode := newRenderText(2, "")

	flow, err := buildInlineFlow(textNode)
	if err != nil {
		t.Fatalf("buildInlineFlow returned error: %v", err)
	}
	if len(flow) != 0 {
		t.Fatalf("expected no flow items for empty text node")
	}
}

func TestBuildInlineFlow_DisplayNone(t *testing.T) {
	child := newRenderElement(2, "inline")
	parent := newRenderElement(1, "none", child)

	flow, err := buildInlineFlow(parent)
	if err != nil {
		t.Fatalf("buildInlineFlow returned error: %v", err)
	}
	if len(flow) != 0 {
		t.Fatalf("expected no flow items for display:none")
	}
}

func TestBuildInlineFlow_InlineBlockIsAtomic(t *testing.T) {
	child := newRenderElement(2, "inline")
	parent := newRenderElement(1, "inline-block", child)

	flow, err := buildInlineFlow(parent)
	if err != nil {
		t.Fatalf("buildInlineFlow returned error: %v", err)
	}
	if len(flow) != 1 {
		t.Fatalf("expected 1 flow item, got %d", len(flow))
	}
	if flow[0].Kind != FlowInline || flow[0].Node.Box != BoxInlineBlock {
		t.Fatalf("expected inline-block to be atomic inline")
	}
}

func TestBuildInlineFlow_NestedInlineSplit(t *testing.T) {
	innerInline1 := newRenderElement(3, "inline")
	innerBlock := newRenderElement(4, "block")
	innerInline2 := newRenderElement(5, "inline")
	childInline := newRenderElement(2, "inline", innerInline1, innerBlock, innerInline2)
	parent := newRenderElement(1, "inline", childInline)

	flow, err := buildInlineFlow(parent)
	if err != nil {
		t.Fatalf("buildInlineFlow returned error: %v", err)
	}
	if len(flow) != 3 {
		t.Fatalf("expected 3 flow items, got %d", len(flow))
	}
	if flow[0].Kind != FlowInline || flow[1].Kind != FlowBlock || flow[2].Kind != FlowInline {
		t.Fatalf("unexpected flow kinds in nested split")
	}
	if flow[0].Node.ID != parent.ID || flow[2].Node.ID != parent.ID {
		t.Fatalf("expected parent inline fragments to keep parent ID")
	}
	if len(flow[0].Node.Children) != 1 || flow[0].Node.Children[0].ID != childInline.ID {
		t.Fatalf("expected parent inline fragment to wrap child inline fragment")
	}
}

func TestBuildBlockContainer_InlineOnly(t *testing.T) {
	childInline1 := newRenderElement(2, "inline")
	childInline2 := newRenderElement(3, "inline")
	parent := newRenderElement(1, "block", childInline1, childInline2)

	node, err := buildBlockContainer(parent, BoxBlock)
	if err != nil {
		t.Fatalf("buildBlockContainer returned error: %v", err)
	}
	if node == nil {
		t.Fatalf("expected layout node, got nil")
	}
	if len(node.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(node.Children))
	}
	if node.Children[0].Box != BoxAnonymousInline {
		t.Fatalf("expected anonymous inline child for inline-only container")
	}
	if len(node.Children[0].Children) != 2 {
		t.Fatalf("expected 2 inline children, got %d", len(node.Children[0].Children))
	}
}

func TestBuildBlockContainer_InlineOnlySingleChild(t *testing.T) {
	childInline := newRenderElement(2, "inline")
	parent := newRenderElement(1, "block", childInline)

	node, err := buildBlockContainer(parent, BoxBlock)
	if err != nil {
		t.Fatalf("buildBlockContainer returned error: %v", err)
	}
	if node == nil {
		t.Fatalf("expected layout node, got nil")
	}
	if len(node.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(node.Children))
	}
	if node.Children[0].Box != BoxAnonymousInline {
		t.Fatalf("expected anonymous inline child for inline-only container")
	}
	if len(node.Children[0].Children) != 1 {
		t.Fatalf("expected 1 inline child, got %d", len(node.Children[0].Children))
	}
}

func TestBuildBlockContainer_MixedInlineBlock(t *testing.T) {
	childInline := newRenderElement(2, "inline")
	childBlock := newRenderElement(3, "block")
	parent := newRenderElement(1, "block", childInline, childBlock)

	node, err := buildBlockContainer(parent, BoxBlock)
	if err != nil {
		t.Fatalf("buildBlockContainer returned error: %v", err)
	}
	if node == nil {
		t.Fatalf("expected layout node, got nil")
	}
	if len(node.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(node.Children))
	}
	if node.Children[0].Box != BoxAnonymousBlock {
		t.Fatalf("expected inline run to be wrapped in anonymous block")
	}
	if node.Children[1].Box != BoxBlock {
		t.Fatalf("expected block child preserved")
	}
}
