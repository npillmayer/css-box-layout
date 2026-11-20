package layout

import (
	"reflect"
	"testing"
)

func TestFixedLayoutRootGeometry(t *testing.T) {
	for _, tc := range []struct {
		name    string
		edit    func(*ComputedStyle)
		natural float32
		frame   Rect
		content Rect
	}{
		{"left top prescribed overflow", func(s *ComputedStyle) {
			s.Fixed.Insets.Left, s.Fixed.Insets.Top = lenPx(10), lenPx(20)
		}, 120, Rect{15, 22, 114, 64}, Rect{36, 30, 80, 40}},
		{"right bottom natural height", func(s *ComputedStyle) {
			s.Fixed.Insets = EdgeLengths{lenAuto(), lenPx(10), lenPx(20), lenAuto()}
			s.Fixed.Height = lenAuto()
		}, 30, Rect{173, 122, 114, 54}, Rect{194, 130, 80, 30}},
		{"opposing insets stretch", func(s *ComputedStyle) {
			s.Fixed.Insets = EdgeLengths{lenPx(5), lenPx(20), lenPx(15), lenPx(10)}
			s.Width, s.Fixed.Height = lenAuto(), lenAuto()
		}, 180, Rect{15, 7, 262, 174}, Rect{36, 15, 228, 150}},
		{"auto margins center", func(s *ComputedStyle) {
			s.Fixed.Insets = EdgeLengths{}
			s.Margin, s.Padding, s.Border = edges(lenAuto()), EdgeLengths{}, EdgeLengths{}
		}, 30, Rect{110, 80, 80, 40}, Rect{110, 80, 80, 40}},
		{"percentage bases", func(s *ComputedStyle) {
			s.Width, s.Fixed.Height = lenPct(.5), lenPct(.25)
			s.Fixed.Insets.Left, s.Fixed.Insets.Top = lenPct(.1), lenPct(.1)
			s.Margin, s.Padding, s.Border = edges(lenPct(.02)), edges(lenPct(.01)), edges(lenPx(1))
		}, 30, Rect{36, 26, 158, 58}, Rect{40, 30, 150, 50}},
		{"fallback font", func(s *ComputedStyle) {
			s.Width, s.Fixed.Height = lenEm(4), lenEm(3)
			s.Fixed.Insets.Left, s.Fixed.Insets.Top = lenEm(1), lenEm(2)
			s.Margin, s.Padding, s.Border = EdgeLengths{}, edges(lenEm(.5)), EdgeLengths{}
		}, 30, Rect{10, 20, 50, 40}, Rect{15, 25, 40, 30}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := anchoredFixedStyle()
			s.Width, s.Fixed.Height = lenPx(80), lenPx(40)
			s.Margin = EdgeLengths{lenPx(2), lenPx(3), lenPx(4), lenPx(5)}
			s.Padding = EdgeLengths{lenPx(7), lenPx(11), lenPx(13), lenPx(17)}
			s.Border = EdgeLengths{lenPx(1), lenPx(2), lenPx(3), lenPx(4)}
			tc.edit(&s)
			root := fixedSizingRoot(t, s, newRenderText(2, "text"))
			before := copyTestTree(root)
			calls := 0
			viewport := Rect{1000, 2000, 300, 200}
			r, err := FixedLayout([]*LayoutNode{root}, viewport, resultFunc(func(req InlineRequest) (InlineResult, error) {
				calls++
				if req.OwnerID != root.BoxID || req.Group != root.Children[0] || req.ContentWidth != tc.content.W {
					t.Fatalf("unexpected inline request: %+v", req)
				}
				return InlineResult{Extent: InlineExtent{W: req.ContentWidth, H: tc.natural}}, nil
			}), nil, FixedLayoutOptions{FontSizePx: 10})
			if err != nil {
				t.Fatal(err)
			}
			want := LayoutGeometry{Frame: tc.frame, Content: tc.content}
			if r.Geometry[root.BoxID] != want || len(r.Geometry) != 1 || r.Viewport != viewport || calls != 1 {
				t.Fatalf("geometry=%+v, want %+v; calls=%d", r.Geometry, want, calls)
			}
			if !reflect.DeepEqual(root, before) {
				t.Fatal("fixed layout mutated input")
			}
		})
	}
}

func TestFixedLayoutNaturalFrameOverflow(t *testing.T) {
	viewport := Rect{W: 100, H: 3e38}
	const naturalHeight float32 = 2e38
	for _, tc := range []struct {
		name      string
		height    Length
		bottom    Length
		flow      bool
		wantError bool
		contentH  float32
		frameH    float32
	}{
		{"prescribed", lenPx(1), lenAuto(), false, false, 1, naturalHeight},
		{"stretch", lenAuto(), lenPx(0), false, false, viewport.H - naturalHeight, viewport.H},
		{"content height overflow", lenAuto(), lenAuto(), false, true, 0, 0},
		{"normal flow overflow", lenAuto(), lenAuto(), true, true, 0, 0},
	} {
		for _, interior := range []string{"inline", "block children"} {
			t.Run(tc.name+"/"+interior, func(t *testing.T) {
				s := anchoredFixedStyle()
				s.Width = lenPx(10)
				s.Padding.Bottom = lenPx(naturalHeight)
				s.Fixed.Height, s.Fixed.Insets.Bottom = tc.height, tc.bottom
				if tc.flow {
					s.Position, s.Fixed = PositionStatic, nil
				}
				child := newRenderText(3, "text")
				if interior == "block children" {
					child = newRenderElement(2, blockDisplay, child)
				}
				render := newRenderElement(1, blockDisplay, child)
				render.Style = &s
				trees, err := BuildLayoutTrees(render, BuildOptions{})
				if err != nil {
					t.Fatal(err)
				}
				calls := 0
				inline := resultFunc(func(req InlineRequest) (InlineResult, error) {
					calls++
					if req.ContentWidth != 10 {
						t.Fatalf("content width = %g, want 10", req.ContentWidth)
					}
					return InlineResult{Extent: InlineExtent{W: 10, H: naturalHeight}}, nil
				})
				if tc.flow {
					used, err := ResolveUsedValues(trees.Flow, ResolveContext{ContainingBlock: viewport})
					if err != nil {
						t.Fatal(err)
					}
					result, err := FlowLayout(trees.Flow, used, inline, nil, LayoutContext{}, LayoutOptions{})
					if err == nil || result != nil || calls != 1 {
						t.Fatalf("normal-flow overflow: result=%+v, error=%v, calls=%d", result, err, calls)
					}
					return
				}
				result, err := FixedLayout(trees.Fixed, viewport, inline, nil, FixedLayoutOptions{})
				if calls != 1 {
					t.Fatalf("final callbacks = %d, want 1", calls)
				}
				if tc.wantError {
					if err == nil || result != nil {
						t.Fatalf("actual fixed-frame overflow: result=%+v, error=%v", result, err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				root := trees.Fixed[0]
				want := LayoutGeometry{Frame: Rect{W: 10, H: tc.frameH}, Content: Rect{W: 10, H: tc.contentH}}
				if got := result.Geometry[root.BoxID]; got != want {
					t.Fatalf("root geometry=%+v, want %+v", got, want)
				}
				if interior == "block children" {
					want := LayoutGeometry{RelativeTo: root.BoxID, Frame: Rect{W: 10, H: naturalHeight}, Content: Rect{W: 10, H: naturalHeight}}
					if got := result.Geometry[root.Children[0].BoxID]; got != want {
						t.Fatalf("child geometry=%+v, want %+v", got, want)
					}
				}
			})
		}
	}
}

func TestFixedLayoutNestedAtomics(t *testing.T) {
	inner := newRenderElement(5, inlineBlockDisplay, newRenderText(6, "z"))
	inner.Style = &ComputedStyle{Width: lenPx(10), Padding: edges(lenPx(1))}
	auto := newRenderElement(3, inlineBlockDisplay, inner, newRenderText(4, "xx"))
	auto.Style = &ComputedStyle{Width: lenAuto(), Padding: edges(lenPx(2))}
	s := anchoredFixedStyle()
	s.Fixed.Insets = EdgeLengths{lenAuto(), lenPx(10), lenPx(20), lenAuto()}
	s.Padding, s.Border = edges(lenPx(5)), edges(lenPx(1))
	root := fixedSizingRoot(t, s, auto)
	a := root.Children[0].Children[0]
	b := a.Children[0].Children[0]
	before := copyTestTree(root)
	measured := []BoxID{}
	measure := measureFunc(func(req InlineMeasureRequest) (IntrinsicWidths, error) {
		measured = append(measured, req.OwnerID)
		switch req.OwnerID {
		case a.BoxID:
			atom, ok := req.Atomics.Lookup(b.BoxID)
			if !ok || atom.BorderWidths != (IntrinsicWidths{12, 12}) {
				t.Fatalf("inner intrinsic atomic: %+v", atom)
			}
			return IntrinsicWidths{12, 22}, nil
		case root.BoxID:
			atom, ok := req.Atomics.Lookup(a.BoxID)
			if !ok || atom.BorderWidths != (IntrinsicWidths{16, 26}) {
				t.Fatalf("auto intrinsic atomic: %+v", atom)
			}
			return atom.BorderWidths, nil
		default:
			t.Fatalf("unexpected measurement owner %d", req.OwnerID)
			return IntrinsicWidths{}, nil
		}
	})
	adapter := newAtomicFake(t)
	r, err := FixedLayout([]*LayoutNode{root}, Rect{1000, 2000, 300, 200}, adapter, measure, FixedLayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := LayoutGeometryTable{
		root.BoxID: {Frame: Rect{252, 152, 38, 28}, Content: Rect{258, 158, 26, 16}},
		a.BoxID:    {RelativeTo: root.BoxID, Frame: Rect{0, 0, 26, 16}, Content: Rect{2, 2, 22, 12}},
		b.BoxID:    {RelativeTo: a.BoxID, Frame: Rect{0, 0, 12, 12}, Content: Rect{1, 1, 10, 10}},
	}
	if !reflect.DeepEqual(r.Geometry, want) {
		t.Fatalf("got %+v, want %+v", r.Geometry, want)
	}
	if !reflect.DeepEqual(measured, []BoxID{a.BoxID, root.BoxID}) || !reflect.DeepEqual(adapter.calls, []BoxID{b.BoxID, a.BoxID, root.BoxID}) {
		t.Fatalf("measurements=%v, final calls=%v", measured, adapter.calls)
	}
	if len(adapter.retained) != 3 || len(adapter.texts[root.Children[0].BoxID]) != 0 || !reflect.DeepEqual(root, before) {
		t.Fatal("atomic ownership or immutable-input contract broken")
	}
}

func TestFixedLayoutBlockMarginsAndPercentDescendants(t *testing.T) {
	a := marginTestBlock(2, 13, 8, true)
	a.Style.Width = lenPct(.5)
	b := marginTestBlock(3, 20, 7, true)
	b.Style.Width = lenAuto()
	s := anchoredFixedStyle()
	s.Width = lenPx(100)
	s.Fixed.Insets = EdgeLengths{lenAuto(), lenAuto(), lenPx(20), lenPx(10)}
	s.Margin = EdgeLengths{Top: lenPx(11), Bottom: lenPx(7), Left: lenPx(9)}
	s.Padding = EdgeLengths{Left: lenPx(2), Right: lenPx(3)}
	root := fixedSizingRoot(t, s, a, b)
	r, err := FixedLayout([]*LayoutNode{root}, Rect{W: 300, H: 200}, newAtomicFake(t), nil, FixedLayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := LayoutGeometryTable{
		root.BoxID:             {Frame: Rect{19, 113, 105, 60}, Content: Rect{21, 113, 100, 60}},
		root.Children[0].BoxID: {RelativeTo: root.BoxID, Frame: Rect{0, 13, 50, 10}, Content: Rect{0, 13, 50, 10}},
		root.Children[1].BoxID: {RelativeTo: root.BoxID, Frame: Rect{0, 43, 100, 10}, Content: Rect{0, 43, 100, 10}},
	}
	if !reflect.DeepEqual(r.Geometry, want) {
		t.Fatalf("got %+v, want %+v", r.Geometry, want)
	}
}

func TestFixedLayoutResolvesDescendantsBelowAutoAtomic(t *testing.T) {
	leaf := newRenderElement(4, blockDisplay, newRenderText(5, "text"))
	leaf.Style = &ComputedStyle{Width: lenPct(.5)}
	block := newRenderElement(3, blockDisplay, leaf)
	block.Style = &ComputedStyle{Width: lenPx(40)}
	atomic := newRenderElement(2, inlineBlockDisplay, block)
	s := anchoredFixedStyle()
	s.Width = lenPx(100)
	root := fixedSizingRoot(t, s, atomic)
	a := root.Children[0].Children[0]
	b := a.Children[0]
	l := b.Children[0]
	adapter := newAtomicFake(t)
	// Explicit block width determines the atomic's intrinsic bounds, so no
	// external measurement is needed. Final resolution must still reach leaf.
	r, err := FixedLayout([]*LayoutNode{root}, Rect{W: 300, H: 200}, adapter, nil, FixedLayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := LayoutGeometryTable{
		root.BoxID: {Frame: Rect{0, 0, 100, 10}, Content: Rect{0, 0, 100, 10}},
		a.BoxID:    {RelativeTo: root.BoxID, Frame: Rect{0, 0, 40, 10}, Content: Rect{0, 0, 40, 10}},
		b.BoxID:    {RelativeTo: a.BoxID, Frame: Rect{0, 0, 40, 10}, Content: Rect{0, 0, 40, 10}},
		l.BoxID:    {RelativeTo: b.BoxID, Frame: Rect{0, 0, 20, 10}, Content: Rect{0, 0, 20, 10}},
	}
	if !reflect.DeepEqual(r.Geometry, want) || !reflect.DeepEqual(adapter.calls, []BoxID{l.BoxID, root.BoxID}) {
		t.Fatalf("geometry=%+v, calls=%v", r.Geometry, adapter.calls)
	}
}

func fixedForestFixture(t *testing.T) *LayoutTrees {
	t.Helper()
	child := newRenderElement(4, blockDisplay, newRenderText(5, "child"))
	child.Style = &ComputedStyle{Width: lenPct(.5)}
	nested := fixedElement(6, inlineDisplay, newRenderText(7, "nested"))
	ns := anchoredFixedStyle()
	ns.Width = lenPct(.5)
	ns.Fixed.Insets = EdgeLengths{lenAuto(), lenPx(7), lenPx(5), lenAuto()}
	nested.Style = &ns
	outer := fixedElement(2, blockDisplay, child, nested)
	os := anchoredFixedStyle()
	os.Width = lenPx(100)
	os.Fixed.Insets.Left, os.Fixed.Insets.Top = lenPx(10), lenPx(20)
	outer.Style = &os
	sibling := fixedElement(8, blockDisplay)
	ss := anchoredFixedStyle()
	ss.Width, ss.Fixed.Height = lenPx(30), lenPx(10)
	sibling.Style = &ss
	trees, err := BuildLayoutTrees(newRenderElement(1, blockDisplay, newRenderText(9, "flow"), outer, sibling), BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return trees
}

func TestFixedLayoutForestCoordinatesAndFlowIndependence(t *testing.T) {
	trees := fixedForestFixture(t)
	runFlow := func() *LayoutResult {
		u, err := ResolveUsedValues(trees.Flow, ResolveContext{ContainingBlock: Rect{W: 250}})
		if err != nil {
			t.Fatal(err)
		}
		r, err := FlowLayout(trees.Flow, u, newAtomicFake(t), nil, LayoutContext{}, LayoutOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	flow := runFlow()
	viewport := Rect{1000, 2000, 300, 200}
	r, err := FixedLayout(trees.Fixed, viewport, newAtomicFake(t), nil, FixedLayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Roots) != 3 || r.Roots[0] != trees.Fixed[0] || r.Roots[1].NodeID != 6 || r.Roots[2].NodeID != 8 {
		t.Fatal("fixed source order changed")
	}
	outer, nested, sibling := r.Roots[0], r.Roots[1], r.Roots[2]
	child := outer.Children[0]
	want := LayoutGeometryTable{
		outer.BoxID:   {Frame: Rect{10, 20, 100, 10}, Content: Rect{10, 20, 100, 10}},
		child.BoxID:   {RelativeTo: outer.BoxID, Frame: Rect{0, 0, 50, 10}, Content: Rect{0, 0, 50, 10}},
		nested.BoxID:  {Frame: Rect{143, 185, 150, 10}, Content: Rect{143, 185, 150, 10}},
		sibling.BoxID: {Frame: Rect{0, 0, 30, 10}, Content: Rect{0, 0, 30, 10}},
	}
	if !reflect.DeepEqual(r.Geometry, want) {
		t.Fatalf("got %+v, want %+v", r.Geometry, want)
	}
	// Follow content parents, then apply the viewport origin exactly once.
	global := func(id BoxID) (float32, float32) {
		g := r.Geometry[id]
		x, y := g.Frame.X, g.Frame.Y
		for parent := g.RelativeTo; parent != 0; {
			p := r.Geometry[parent]
			x, y, parent = x+p.Content.X, y+p.Content.Y, p.RelativeTo
		}
		return x + r.Viewport.X, y + r.Viewport.Y
	}
	if x, y := global(child.BoxID); x != 1010 || y != 2020 {
		t.Fatalf("child global origin=(%g,%g)", x, y)
	}
	if x, y := global(nested.BoxID); x != 1143 || y != 2185 {
		t.Fatalf("nested fixed global origin=(%g,%g)", x, y)
	}
	translated, err := FixedLayout(trees.Fixed, Rect{-300, -400, 300, 200}, newAtomicFake(t), nil, FixedLayoutOptions{})
	if err != nil || !reflect.DeepEqual(translated.Geometry, r.Geometry) {
		t.Fatalf("external viewport origin entered local layout: %v", err)
	}
	if !reflect.DeepEqual(runFlow(), flow) {
		t.Fatal("fixed layout changed normal-flow results")
	}
	trees.Fixed[0] = nil
	if r.Roots[0] != outer {
		t.Fatal("result root slice aliases caller slice")
	}
}
