package layout

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

type resultFunc func(InlineRequest) (InlineResult, error)

func (f resultFunc) LayoutInline(r InlineRequest) (InlineResult, error) { return f(r) }

// This fake P breaks only between input items. Whatsits are indivisible, may
// overflow an empty line, and align their bottom margin edge to its baseline.
type atomicFake struct {
	t         *testing.T
	active    bool
	calls     []BoxID
	retained  map[BoxID]InlineResult
	texts     map[BoxID][]BoxID
	failOwner BoxID
	failure   error
}

func (f *atomicFake) LayoutInline(r InlineRequest) (InlineResult, error) {
	if f.active {
		f.t.Fatal("reentrant external layout")
	}
	f.active = true
	defer func() { f.active = false }()
	f.calls = append(f.calls, r.OwnerID)
	if r.OwnerID == f.failOwner {
		return InlineResult{}, f.failure
	}
	type item struct {
		handle int
		w, h   float32
		margin Edges
	}
	var items []item
	handles := map[int]BoxID{}
	var walk func(*LayoutNode)
	walk = func(n *LayoutNode) {
		if n.Box == BoxInlineBlock {
			a, ok := r.Atomics.Lookup(n.BoxID)
			if !ok || a.BoxID != n.BoxID {
				f.t.Fatal("unprepared branch")
			}
			u, _ := r.Used.Lookup(n.BoxID)
			handle := len(items) + 100
			handles[handle] = n.BoxID
			items = append(items, item{handle, a.BorderWidth, a.BorderHeight, u.Margin})
			a.BorderWidth = 999
			again, _ := r.Atomics.Lookup(n.BoxID)
			if again.BorderWidth == 999 {
				f.t.Fatal("mutable prepared view")
			}
			return
		}
		if n.Box == BoxText {
			f.texts[r.Group.BoxID] = append(f.texts[r.Group.BoxID], n.BoxID)
			items = append(items, item{w: float32(n.Text.Range.End-n.Text.Range.Start) * 5, h: 10})
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(r.Group)
	if _, ok := r.Atomics.Lookup(r.OwnerID); ok {
		f.t.Fatal("view includes owner instead of group boundaries")
	}
	var result InlineResult
	var line []item
	var x, y, h, maxW float32
	flush := func() {
		var px float32
		for _, it := range line {
			if it.handle != 0 {
				result.Atomics = append(result.Atomics, AtomicPlacement{handles[it.handle], px + it.margin.Left, y + h - it.margin.Bottom - it.h})
			}
			px += it.w + it.margin.Left + it.margin.Right
		}
		if x > maxW {
			maxW = x
		}
		y += h
		line = nil
		x = 0
		h = 0
	}
	for _, it := range items {
		w := it.w + it.margin.Left + it.margin.Right
		if len(line) > 0 && x+w > r.ContentWidth {
			flush()
		}
		line = append(line, it)
		x += w
		if ih := it.h + it.margin.Top + it.margin.Bottom; ih > h {
			h = ih
		}
	}
	flush()
	result.Extent = InlineExtent{maxW, y}
	f.retained[r.Group.BoxID] = result
	return result, nil
}

func recursiveFixture(t *testing.T) (*LayoutNode, map[NodeID]*LayoutNode) {
	b := newRenderElement(5, "inline-block", newRenderText(6, "z"))
	b.Style = &ComputedStyle{Width: lenPx(10), Padding: edges(lenPx(1))}
	c := newRenderElement(4, "block", newRenderElement(7, "inline", b))
	c.Style = &ComputedStyle{Width: lenPct(.5), Padding: EdgeLengths{Left: lenPct(.1)}}
	a := newRenderElement(3, "inline-block", newRenderText(8, "a"), c, newRenderText(9, "b"))
	a.Style = &ComputedStyle{Width: lenPx(40), Padding: edges(lenPx(2)), Border: edges(lenPx(1))}
	input := newRenderElement(1, "block", newRenderText(2, "Hello "), newRenderElement(10, "inline", a), newRenderText(11, " world."), newRenderElement(12, "block", newRenderText(13, "tail")))
	root, err := BuildLayoutTree(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	nodes := map[NodeID]*LayoutNode{}
	var walk func(*LayoutNode)
	walk = func(n *LayoutNode) {
		if n.NodeID != 0 {
			nodes[n.NodeID] = n
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	return root, nodes
}
func newAtomicFake(t *testing.T) *atomicFake {
	return &atomicFake{t: t, retained: map[BoxID]InlineResult{}, texts: map[BoxID][]BoxID{}}
}

func TestRecursiveAtomicsPreparationPlacementAndSnapshots(t *testing.T) {
	root, n := recursiveFixture(t)
	original := copyTestTree(root)
	run := func(width float32, f *atomicFake) (*LayoutResult, error) {
		u, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: width}})
		if err != nil {
			t.Fatal(err)
		}
		before := make(UsedValuesTable)
		for id, v := range u {
			before[id] = v
		}
		if u[n[4].BoxID].ContentWidth != 20 || u[n[4].BoxID].Padding.Left != 4 {
			t.Fatal("wrong atomic descendant basis")
		}
		r, err := FlowLayout(root, u, f, nil, LayoutContext{}, LayoutOptions{})
		if !reflect.DeepEqual(before, u) || !reflect.DeepEqual(root, original) {
			t.Fatal("mutated inputs")
		}
		return r, err
	}
	firstAdapter := newAtomicFake(t)
	first, err := run(120, firstAdapter)
	if err != nil {
		t.Fatal(err)
	}
	outer := root.Children[0]
	wantOrder := []BoxID{n[3].Children[0].BoxID, n[5].BoxID, n[4].BoxID, n[3].Children[2].BoxID, outer.BoxID, n[12].BoxID}
	if !reflect.DeepEqual(firstAdapter.calls, wantOrder) {
		t.Fatalf("order %v want %v", firstAdapter.calls, wantOrder)
	}
	if got := firstAdapter.texts[outer.Children[0].BoxID]; !reflect.DeepEqual(got, []BoxID{n[2].BoxID, n[11].BoxID}) {
		t.Fatalf("inner text leaked: %v", got)
	}
	a := first.Geometry[n[3].BoxID]
	b := first.Geometry[n[5].BoxID]
	c := first.Geometry[n[4].BoxID]
	if a != (LayoutGeometry{RelativeTo: outer.BoxID, Frame: Rect{X: 30, W: 46, H: 38}, Content: Rect{X: 33, Y: 3, W: 40, H: 32}}) {
		t.Fatalf("A: %+v", a)
	}
	if c.RelativeTo != n[3].BoxID || c.Content != (Rect{X: 4, Y: 10, W: 20, H: 12}) {
		t.Fatalf("C: %+v", c)
	}
	if b.RelativeTo != n[4].BoxID || b.Content != (Rect{X: 1, Y: 1, W: 10, H: 10}) {
		t.Fatalf("B: %+v", b)
	}
	var wx, wy float32
	id := n[5].BoxID
	for id != 0 {
		g := first.Geometry[id]
		wx += g.Content.X
		wy += g.Content.Y
		id = g.RelativeTo
	}
	if wx != 38 || wy != 14 {
		t.Fatalf("composed origin %g,%g", wx, wy)
	}
	if first.Geometry[n[12].BoxID].Frame.Y != 38 {
		t.Fatal("following sibling height")
	}
	saved := copyTestGeometry(first.Geometry)
	secondAdapter := newAtomicFake(t)
	second, err := run(30, secondAdapter)
	if err != nil {
		t.Fatal(err)
	}
	if second.Geometry[n[3].BoxID].Frame.W != 46 || second.Geometry[n[12].BoxID].Frame.Y != 58 {
		t.Fatal("fixed overflow/resize propagation")
	}
	// Returned placement slices are adapter-owned, but geometry must not alias them.
	p := firstAdapter.retained[outer.Children[0].BoxID]
	p.Atomics[0].X = 999
	failed := newAtomicFake(t)
	failed.failOwner = n[4].BoxID
	failed.failure = errors.New("inner failure")
	r, err := run(60, failed)
	if r != nil || !errors.Is(err, failed.failure) {
		t.Fatalf("failure %v", err)
	}
	for _, id := range failed.calls {
		if id == outer.BoxID {
			t.Fatal("dependent outer called after failure")
		}
	}
	if len(failed.retained) != 2 {
		t.Fatalf("expected partial inner state: %d", len(failed.retained))
	}
	if !reflect.DeepEqual(first.Geometry, saved) {
		t.Fatal("old snapshot changed")
	}
	again, err := run(120, newAtomicFake(t))
	if err != nil || !reflect.DeepEqual(again.Geometry, saved) {
		t.Fatal("failure poisoned rerun")
	}
}

func TestAtomicPlacementValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    []AtomicPlacement
		bad  bool
	}{
		{"missing", nil, true}, {"unknown", []AtomicPlacement{{BoxID: 99}}, true},
		{"foreign", []AtomicPlacement{{BoxID: 1}}, true},
		{"nan", []AtomicPlacement{{BoxID: 3, X: float32(math.NaN())}}, true},
		{"infinity", []AtomicPlacement{{BoxID: 3, Y: float32(math.Inf(1))}}, true},
		{"extra", []AtomicPlacement{{BoxID: 3}, {BoxID: 3}}, true},
		{"negative", []AtomicPlacement{{BoxID: 3, X: -4, Y: -2}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := &LayoutNode{BoxID: 1, Box: BoxBlock, Children: []*LayoutNode{{BoxID: 2, Box: BoxAnonymousInline, Children: []*LayoutNode{{BoxID: 3, Box: BoxInlineBlock, Style: &ComputedStyle{Width: lenPx(0)}, Children: []*LayoutNode{{BoxID: 4, Box: BoxAnonymousInline}}}}}}}
			used, _ := ResolveUsedValues(root, ResolveContext{})
			r, err := FlowLayout(root, used, resultFunc(func(req InlineRequest) (InlineResult, error) {
				if req.OwnerID == 3 {
					return InlineResult{}, nil
				}
				a, ok := req.Atomics.Lookup(3)
				if !ok || a.BorderWidth != 0 || a.BorderHeight != 0 {
					t.Fatal("zero item lost")
				}
				return InlineResult{Atomics: tc.p}, nil
			}), nil, LayoutContext{}, LayoutOptions{})
			if tc.bad {
				if err == nil || r != nil {
					t.Fatal("accepted invalid placement")
				}
			} else if err != nil || r.Geometry[3].Frame.X != -4 {
				t.Fatalf("negative zero placement: %v", err)
			}
		})
	}
	items := map[BoxID]PreparedAtomic{3: {BoxID: 3}, 4: {BoxID: 4}}
	geom := LayoutGeometryTable{3: {Frame: Rect{W: 2}}, 4: {Frame: Rect{W: 3}}}
	before := copyTestGeometry(geom)
	if err := placeAtomics([]AtomicPlacement{{BoxID: 3}, {BoxID: 3}}, items, 1, geom); err == nil || !reflect.DeepEqual(geom, before) {
		t.Fatal("duplicate/transaction validation")
	}
}

func TestAtomicUnsupportedWidthFailsBeforeDelegation(t *testing.T) {
	for _, w := range []Length{lenPct(.5), {Kind: LenEm, Value: 2}} {
		a := newRenderElement(2, "inline-block")
		a.Style = &ComputedStyle{Width: w}
		root, _ := BuildLayoutTree(newRenderElement(1, "block", a), BuildOptions{})
		used, _ := ResolveUsedValues(root, ResolveContext{})
		calls := 0
		r, err := FlowLayout(root, used, resultFunc(func(InlineRequest) (InlineResult, error) { calls++; return InlineResult{}, nil }), nil, LayoutContext{}, LayoutOptions{})
		if err == nil || r != nil || calls != 0 {
			t.Fatal("unsupported width reached adapter")
		}
	}
}

func TestAtomicMarginsAndZeroDescendantBasis(t *testing.T) {
	for _, width := range []float32{0, 20} {
		child := newRenderElement(4, "block", newRenderText(5, "x"))
		child.Style = &ComputedStyle{Width: lenPct(.5), Padding: EdgeLengths{Left: lenPct(.1)}}
		a := newRenderElement(3, "inline-block", child)
		a.Style = &ComputedStyle{Width: lenPx(width), Margin: EdgeLengths{Left: lenPx(3), Right: lenPx(4), Top: lenPx(2), Bottom: lenPx(5)}, Padding: edges(lenPx(1))}
		root, err := BuildLayoutTree(newRenderElement(1, "block", newRenderText(2, "x"), a), BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		used, _ := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 100}})
		atomic := root.Children[0].Children[1]
		desc := atomic.Children[0]
		if used[desc.BoxID].ContentWidth != width/2 || used[desc.BoxID].Padding.Left != width/10 {
			t.Fatal("zero/percent descendant basis")
		}
		f := newAtomicFake(t)
		result, err := FlowLayout(root, used, f, nil, LayoutContext{}, LayoutOptions{})
		if err != nil {
			t.Fatal(err)
		}
		g := result.Geometry[atomic.BoxID]
		if g.Frame.X != 8 || g.Frame.Y != 2 || g.Frame.W != width+2 || g.Frame.H != 12 || g.Content.X != 9 || g.Content.Y != 3 {
			t.Fatalf("margin/alignment accounting %+v", g)
		}
		if result.Geometry[root.BoxID].Content.H != 19 {
			t.Fatal("margin extent lost")
		}
	}
}
