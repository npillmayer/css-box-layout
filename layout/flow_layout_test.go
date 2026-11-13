package layout

import "testing"

type fakeInlineLayouter struct {
	extent InlineExtent
}

func (f fakeInlineLayouter) LayoutInline(InlineRequest) (InlineResult, error) {
	return InlineResult{Extent: f.extent}, nil
}

type fakeIntrinsic struct {
	maxContent float32
}

func (f fakeIntrinsic) MeasureInline(InlineMeasureRequest) (IntrinsicWidths, error) {
	return IntrinsicWidths{MaxContent: f.maxContent}, nil
}

func TestFlowLayout_BlockStacking(t *testing.T) {
	root := &LayoutNode{BoxID: 1, Box: BoxBlock}
	c1 := &LayoutNode{BoxID: 2, Box: BoxBlock}
	c2 := &LayoutNode{BoxID: 3, Box: BoxBlock}
	root.Children = []*LayoutNode{c1, c2}

	used := UsedValuesTable{
		root.BoxID: {ContentWidth: 100},
		c1.BoxID: {
			ContentWidth: 50,
			Margin:       Edges{Top: 10, Bottom: 5, Left: 3},
		},
		c2.BoxID: {
			ContentWidth: 60,
			Margin:       Edges{Top: 7, Bottom: 4, Left: 2},
		},
	}

	res, err := ComputeLayoutWithConstraints(
		root,
		used,
		fakeInlineLayouter{},
		fakeIntrinsic{},
		LayoutContext{ContainingBlock: Rect{W: 100}},
		LayoutOptions{},
	)
	if err != nil {
		t.Fatalf("ComputeLayoutWithConstraints error: %v", err)
	}

	c1Geom := res.Geometry[c1.BoxID]
	if c1Geom.Frame.Y != 10 {
		t.Fatalf("child1 Frame.Y = %v, want 10", c1Geom.Frame.Y)
	}
	if c1Geom.Frame.X != 3 {
		t.Fatalf("child1 Frame.X = %v, want 3", c1Geom.Frame.X)
	}

	c2Geom := res.Geometry[c2.BoxID]
	if c2Geom.Frame.Y != 22 {
		t.Fatalf("child2 Frame.Y = %v, want 22", c2Geom.Frame.Y)
	}
	if c2Geom.Frame.X != 2 {
		t.Fatalf("child2 Frame.X = %v, want 2", c2Geom.Frame.X)
	}

	rootGeom := res.Geometry[root.BoxID]
	if rootGeom.Content.H != 26 {
		t.Fatalf("root Content.H = %v, want 26", rootGeom.Content.H)
	}
}

func TestFlowLayout_InlineOnlyDelegation(t *testing.T) {
	inlineRoot := &LayoutNode{BoxID: 2, Box: BoxAnonymousInline}
	root := &LayoutNode{BoxID: 1, Box: BoxBlock, Children: []*LayoutNode{inlineRoot}}

	used := UsedValuesTable{
		root.BoxID: {
			ContentWidth: 100,
			Padding:      Edges{Top: 2, Bottom: 2},
			Border:       Edges{Top: 1, Bottom: 1},
		},
		inlineRoot.BoxID: {},
	}

	extent := InlineExtent{W: 100, H: 22}

	res, err := ComputeLayoutWithConstraints(
		root,
		used,
		fakeInlineLayouter{extent: extent},
		fakeIntrinsic{},
		LayoutContext{ContainingBlock: Rect{W: 100}},
		LayoutOptions{},
	)
	if err != nil {
		t.Fatalf("ComputeLayoutWithConstraints error: %v", err)
	}
	rootGeom := res.Geometry[root.BoxID]
	if rootGeom.Content.H != 22 {
		t.Fatalf("content height = %v, want 22", rootGeom.Content.H)
	}
	if rootGeom.Frame.H != 28 {
		t.Fatalf("frame height = %v, want 28", rootGeom.Frame.H)
	}
}

func TestFlowLayout_AnonymousBlockExtent(t *testing.T) {
	inlineRoot := &LayoutNode{BoxID: 2, Box: BoxAnonymousInline}
	anonBlock := &LayoutNode{BoxID: 1, Box: BoxAnonymousBlock, Children: []*LayoutNode{inlineRoot}}

	used := UsedValuesTable{
		anonBlock.BoxID:  {ContentWidth: 100},
		inlineRoot.BoxID: {},
	}

	extent := InlineExtent{W: 100, H: 10}

	res, err := ComputeLayoutWithConstraints(
		anonBlock,
		used,
		fakeInlineLayouter{extent: extent},
		fakeIntrinsic{},
		LayoutContext{ContainingBlock: Rect{W: 100}},
		LayoutOptions{},
	)
	if err != nil {
		t.Fatalf("ComputeLayoutWithConstraints error: %v", err)
	}
	if got := res.Geometry[anonBlock.BoxID].Content.H; got != 10 {
		t.Fatalf("anonymous block height=%v, want 10", got)
	}
}
