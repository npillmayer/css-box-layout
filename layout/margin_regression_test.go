package layout

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestCollapsedAnonymousBlockSeparators(t *testing.T) {
	for _, tc := range []struct {
		name                                            string
		allow                                           bool
		height, parentY, blockY, tailY, contentH, rootH float32
	}{
		{"ignorable empty runs", true, 0, 20, 0, 17, 10, 37},
		{"zero-height separating runs", false, 0, 13, 20, 37, 37, 55},
		{"reserved paragraph space", false, 4, 13, 24, 41, 45, 63},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := marginTestBlock(2, 13, 5, false, newRenderElement(3, "inline"),
				marginTestBlock(4, 20, 7, true), newRenderElement(5, "inline"))
			root, err := BuildLayoutTree(marginTestBlock(1, 0, 0, false, p), BuildOptions{})
			if err != nil {
				t.Fatal(err)
			}
			parent := root.Children[0]
			first, block, last := parent.Children[0], parent.Children[1], parent.Children[2]
			if first.Box != BoxAnonymousBlock || last.Box != BoxAnonymousBlock {
				t.Fatal("expected anonymous inline-run owners")
			}
			used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 100}})
			if err != nil {
				t.Fatal(err)
			}
			var calls []BoxID
			adapter := resultFunc(func(req InlineRequest) (InlineResult, error) {
				calls = append(calls, req.OwnerID)
				if req.OwnerID == block.BoxID {
					return InlineResult{Extent: InlineExtent{H: 10}}, nil
				}
				return InlineResult{Extent: InlineExtent{H: tc.height}, AllowsMarginCollapse: tc.allow}, nil
			})
			result, err := FlowLayout(root, used, adapter, nil, LayoutContext{}, LayoutOptions{})
			if err != nil {
				t.Fatal(err)
			}
			g := result.Geometry
			if g[parent.BoxID].Frame.Y != tc.parentY || g[first.BoxID].Frame.Y != 0 || g[block.BoxID].Frame.Y != tc.blockY || g[last.BoxID].Frame.Y != tc.tailY || g[parent.BoxID].Content.H != tc.contentH || g[root.BoxID].Content.H != tc.rootH {
				t.Fatalf("incorrect anonymous-run separation: %+v", g)
			}
			if !reflect.DeepEqual(calls, []BoxID{first.BoxID, block.BoxID, last.BoxID}) {
				t.Fatalf("callback order/count=%v", calls)
			}
			for _, n := range parent.Children {
				if g[n.BoxID].RelativeTo != parent.BoxID {
					t.Fatal("anonymous/block coordinate owner changed")
				}
			}
		})
	}
}

func TestCollapsedResolvedMarginsAndResize(t *testing.T) {
	a, e, b := marginTestBlock(2, 0, 0, true), marginTestBlock(3, 0, 0, false), marginTestBlock(4, 0, 0, true)
	for _, n := range []*RenderNode{a, e, b} {
		n.Style.Width = lenPx(40)
		n.Style.FontSizePx = 10
		n.Style.Margin.Left, n.Style.Margin.Right = lenAuto(), lenAuto()
	}
	a.Style.Margin.Top, a.Style.Margin.Bottom = lenAuto(), lenPct(.1)
	e.Style.Margin.Top, e.Style.Margin.Bottom = lenAuto(), lenEm(-.5)
	b.Style.Margin.Top, b.Style.Margin.Bottom = lenEm(1.5), lenAuto()
	input := marginTestBlock(1, 0, 0, false, a, e, b)
	input.Style.Width = lenAuto()
	root, err := BuildLayoutTree(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var previous *LayoutResult
	var snapshot LayoutGeometryTable
	for _, tc := range []struct{ width, x, emptyY, nextY, height float32 }{
		{200, 80, 30, 25, 35}, {100, 30, 20, 20, 30}, {200, 80, 30, 25, 35},
	} {
		used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: tc.width}})
		if err != nil {
			t.Fatal(err)
		}
		before := make(UsedValuesTable)
		for id, u := range used {
			before[id] = u
		}
		nodes := root.Children
		if used[nodes[0].BoxID].Margin.Top != 0 || used[nodes[2].BoxID].Margin.Bottom != 0 {
			t.Fatal("vertical auto margins did not resolve to zero")
		}
		result, err := FlowLayout(root, used, newAtomicFake(t), nil, LayoutContext{}, LayoutOptions{})
		if err != nil {
			t.Fatal(err)
		}
		g := result.Geometry
		if g[nodes[1].BoxID].Frame.Y != tc.emptyY || g[nodes[2].BoxID].Frame.Y != tc.nextY || g[root.BoxID].Content.H != tc.height {
			t.Fatalf("incorrect resolved collapse: %+v", g)
		}
		for _, n := range nodes {
			if g[n.BoxID].Frame.X != tc.x {
				t.Fatal("vertical collapse changed horizontal centering")
			}
		}
		if !reflect.DeepEqual(used, before) {
			t.Fatal("collapsed margins replaced resolved values")
		}
		if previous != nil && !reflect.DeepEqual(previous.Geometry, snapshot) {
			t.Fatal("resize mutated old result")
		}
		previous, snapshot = result, copyTestGeometry(g)
	}
}

func TestCollapsedMarginsAfterAutoAtomicWidth(t *testing.T) {
	a, b := marginTestBlock(3, 0, 13, true), marginTestBlock(4, 8, 0, true)
	a.Style.Width = lenPx(40)
	a.Style.Margin.Top = lenAuto()
	a.Style.Margin.Left, a.Style.Margin.Right = lenAuto(), lenAuto()
	b.Style.Width = lenAuto()
	b.Style.Margin.Bottom = lenAuto()
	atomic := marginTestBlock(2, 0, 0, false, a, b)
	atomic.Styles["display"] = "inline-block"
	atomic.Style.Width = lenAuto()
	input := marginTestBlock(1, 0, 0, false, atomic)
	input.Style.Width = lenAuto()
	root, err := BuildLayoutTree(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	owner := root.Children[0].Children[0]
	first, last := owner.Children[0], owner.Children[1]
	for _, tc := range []struct{ available, width, x float32 }{{200, 100, 30}, {60, 60, 10}, {30, 40, 0}} {
		used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: tc.available}})
		if err != nil {
			t.Fatal(err)
		}
		before := make(UsedValuesTable)
		for id, u := range used {
			before[id] = u
		}
		measures := 0
		measure := measureFunc(func(req InlineMeasureRequest) (IntrinsicWidths, error) {
			measures++
			if req.OwnerID != last.BoxID {
				t.Fatal("unexpected measurement owner")
			}
			return IntrinsicWidths{MinContent: 20, MaxContent: 100}, nil
		})
		fake := newAtomicFake(t)
		adapter := resultFunc(func(req InlineRequest) (InlineResult, error) {
			u, _ := req.Used.Lookup(first.BoxID)
			if u.EdgesPending || u.Margin.Left != tc.x || u.Margin.Bottom != 13 {
				t.Fatalf("callback received provisional edges: %+v", u)
			}
			return fake.LayoutInline(req)
		})
		result, err := FlowLayout(root, used, adapter, measure, LayoutContext{}, LayoutOptions{})
		if err != nil {
			t.Fatal(err)
		}
		g := result.Geometry
		if measures != 1 || len(fake.calls) != 3 || g[owner.BoxID].Content.W != tc.width || g[first.BoxID].Frame.X != tc.x || g[last.BoxID].Frame.Y != 23 || g[owner.BoxID].Content.H != 33 || g[root.BoxID].Content.H != 33 {
			t.Fatalf("incorrect final atomic sizing/placement: %+v", g)
		}
		if !reflect.DeepEqual(used, before) {
			t.Fatal("final width resolution mutated caller values")
		}
	}
}

func TestCollapsedBoundaryCombinations(t *testing.T) {
	// Border and padding barriers have identical collapse eligibility but distinct
	// content origins. Exercise every top/bottom combination, including horizontal
	// edges that must never obstruct vertical collapsing.
	for _, top := range []string{"open", "padding", "border"} {
		for _, bottom := range []string{"open", "padding", "border"} {
			t.Run(top+"/"+bottom, func(t *testing.T) {
				parent := marginTestBlock(2, 13, 5, false, marginTestBlock(3, 20, 7, true))
				parent.Style.Padding.Left = lenPx(2)
				parent.Style.Border.Right = lenPx(3)
				switch top {
				case "padding":
					parent.Style.Padding.Top = lenPx(1)
				case "border":
					parent.Style.Border.Top = lenPx(1)
				}
				switch bottom {
				case "padding":
					parent.Style.Padding.Bottom = lenPx(1)
				case "border":
					parent.Style.Border.Bottom = lenPx(1)
				}
				g, _ := marginTestLayout(t, marginTestBlock(1, 0, 0, false, parent))
				parentY, childY, contentH, frameH, bottomGap := float32(20), float32(0), float32(10), float32(10), float32(7)
				var contentOffset float32
				if top != "open" {
					parentY = 13
					contentOffset = 1
					childY = 20
					contentH += 20
					frameH += 21
				}
				if bottom != "open" {
					contentH += 7
					frameH += 8
					bottomGap = 5
				}
				if g[2].Frame.Y != parentY || g[3].Frame.Y != childY || g[2].Content.Y != parentY+contentOffset || g[2].Content.H != contentH || g[2].Frame.H != frameH || g[1].Content.H != parentY+frameH+bottomGap {
					t.Fatalf("incorrect boundary geometry: %+v", g)
				}
			})
		}
	}
}

func TestCollapsedFailureReplayAndConcurrentInputs(t *testing.T) {
	input := marginTestBlock(1, 0, 0, false, marginTestBlock(2, 0, 5, true),
		marginTestBlock(3, 7, 30, false), marginTestBlock(4, -12, 3, false), marginTestBlock(5, 50, 0, true))
	root, err := BuildLayoutTree(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 100}})
	if err != nil {
		t.Fatal(err)
	}
	original := copyTestTree(root)
	before := make(UsedValuesTable)
	for id, u := range used {
		before[id] = u
	}
	first, err := FlowLayout(root, used, newAtomicFake(t), nil, LayoutContext{}, LayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	saved := copyTestGeometry(first.Geometry)
	failure := errors.New("late failure with an unresolved margin group")
	failed := newAtomicFake(t)
	failed.failOwner, failed.failure = root.Children[3].BoxID, failure
	result, err := FlowLayout(root, used, failed, nil, LayoutContext{}, LayoutOptions{})
	if result != nil || !errors.Is(err, failure) || len(failed.calls) != 4 || len(failed.retained) != 3 {
		t.Fatalf("unexpected failed invocation: result=%v err=%v calls=%v", result, err, failed.calls)
	}
	// Independent adapters prevent retained paragraph state from being shared.
	// Exercise replay on the exact same inputs concurrently under the race suite.
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := FlowLayout(root, used, newAtomicFake(t), nil, LayoutContext{}, LayoutOptions{})
			if e != nil {
				t.Error(e)
				return
			}
			if !reflect.DeepEqual(r.Geometry, saved) {
				t.Error("failure/replay changed collapsed geometry")
			}
		}()
	}
	wg.Wait()
	if !reflect.DeepEqual(first.Geometry, saved) || !reflect.DeepEqual(used, before) || !reflect.DeepEqual(root, original) {
		t.Fatal("failure or replay mutated a prior snapshot/input")
	}
	if first.Geometry[root.BoxID].Content.H != 58 || first.Geometry[root.Children[3].BoxID].Frame.Y != 48 {
		t.Fatal("unexpected collapsed geometry oracle")
	}
}
