package layout

import "testing"

func TestNormalizeBlockChildren_BlockOnly1(t *testing.T) {
	a := &LayoutNode{ID: 1, Box: BoxBlock, FC: FCBlock}
	b := &LayoutNode{ID: 2, Box: BoxAnonymousBlock, FC: FCBlock}

	flow := []FlowItem{
		BlockItem(a),
		BlockItem(b),
	}

	children, err := normalizeBlockChildren(flow)
	if err != nil {
		t.Fatalf("normalizeBlockChildren returned error: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
	if children[0] != a || children[1] != b {
		t.Fatalf("expected block nodes returned in order")
	}
	if children[0].Box == BoxAnonymousInline || children[1].Box == BoxAnonymousInline {
		t.Fatalf("did not expect anonymous inline wrappers for block-only flow")
	}
}

func TestNormalizeBlockChildren_BlockOnly2(t *testing.T) {
	a := &LayoutNode{ID: 1, Box: BoxBlock, FC: FCBlock}
	b := &LayoutNode{ID: 2, Box: BoxAnonymousInline, FC: FCBlock}

	flow := []FlowItem{
		BlockItem(a),
		InlineItem(b),
	}

	children, err := normalizeBlockChildren(flow)
	if err != nil {
		t.Fatalf("normalizeBlockChildren returned error: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
	if children[0] != a {
		t.Fatalf("expected block nodes returned in order")
	}
	if children[1].Box != BoxAnonymousBlock {
		t.Fatalf("did expect anonymous block wrappers for block-only flow")
	}
}
