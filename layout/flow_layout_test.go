package layout

import "testing"

type fakeInlineLayouter struct {
	lines []LineBox
}

func (f fakeInlineLayouter) LayoutInline(inlineRoot *LayoutNode, maxWidth float32, atomic AtomicSizer) ([]LineBox, error) {
	return append([]LineBox(nil), f.lines...), nil
}

type fakeIntrinsic struct {
	maxContent float32
}

func (f fakeIntrinsic) MaxContentWidth(node *LayoutNode) (float32, error) {
	return f.maxContent, nil
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

	lines := []LineBox{
		{Frame: Rect{Y: 0, H: 10}},
		{Frame: Rect{Y: 10, H: 12}},
	}

	res, err := ComputeLayoutWithConstraints(
		root,
		used,
		fakeInlineLayouter{lines: lines},
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
	if got := len(res.Lines[root.BoxID]); got != 2 {
		t.Fatalf("stored lines = %d, want 2", got)
	}
	if _, ok := res.Lines[inlineRoot.BoxID]; ok {
		t.Fatalf("did not expect lines stored for anonymous inline root")
	}
}

func TestAtomicSizer_InlineBlockWidth(t *testing.T) {
	inlineBlock := &LayoutNode{BoxID: 1, Box: BoxInlineBlock}

	a := atomicSizer{
		inline:    fakeInlineLayouter{},
		intrinsic: fakeIntrinsic{maxContent: 200},
		used: UsedValuesTable{
			inlineBlock.BoxID: {ContentWidth: 120},
		},
		geom:  make(LayoutGeometryTable),
		lines: make(LinesByBlock),
	}

	w, _, err := a.SizeInlineBlock(inlineBlock, 150)
	if err != nil {
		t.Fatalf("SizeInlineBlock error: %v", err)
	}
	if w != 120 {
		t.Fatalf("width = %v, want 120", w)
	}

	a.used[inlineBlock.BoxID] = UsedValues{ContentWidth: 0}
	w, _, err = a.SizeInlineBlock(inlineBlock, 150)
	if err != nil {
		t.Fatalf("SizeInlineBlock error: %v", err)
	}
	if w != 150 {
		t.Fatalf("width = %v, want 150", w)
	}
}

func TestFlowLayout_LinesNotStoredForAnonymousBlock(t *testing.T) {
	inlineRoot := &LayoutNode{BoxID: 2, Box: BoxAnonymousInline}
	anonBlock := &LayoutNode{BoxID: 1, Box: BoxAnonymousBlock, Children: []*LayoutNode{inlineRoot}}

	used := UsedValuesTable{
		anonBlock.BoxID: {ContentWidth: 100},
		inlineRoot.BoxID: {},
	}

	lines := []LineBox{{Frame: Rect{Y: 0, H: 10}}}

	res, err := ComputeLayoutWithConstraints(
		anonBlock,
		used,
		fakeInlineLayouter{lines: lines},
		fakeIntrinsic{},
		LayoutContext{ContainingBlock: Rect{W: 100}},
		LayoutOptions{},
	)
	if err != nil {
		t.Fatalf("ComputeLayoutWithConstraints error: %v", err)
	}
	if _, ok := res.Lines[anonBlock.BoxID]; ok {
		t.Fatalf("did not expect lines stored for anonymous block owner")
	}
}
