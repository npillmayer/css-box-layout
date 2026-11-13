package layout

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type measureFunc func(InlineMeasureRequest) (IntrinsicWidths, error)

func (f measureFunc) MeasureInline(r InlineMeasureRequest) (IntrinsicWidths, error) { return f(r) }

func autoFixture(t *testing.T, children ...*RenderNode) (*LayoutNode, *LayoutNode) {
	t.Helper()
	a := newRenderElement(2, "inline-block", children...)
	a.Style = &ComputedStyle{Width: lenAuto(), Margin: EdgeLengths{Left: lenPx(2), Right: lenPx(3)}, Padding: edges(lenPx(3)), Border: edges(lenPx(1))}
	root, err := BuildLayoutTree(newRenderElement(1, "block", a), BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return root, root.Children[0].Children[0]
}

func TestAutoShrinkToFitAndSnapshots(t *testing.T) {
	root, a := autoFixture(t, newRenderText(3, "text"))
	original := copyTestTree(root)
	var previous *LayoutResult
	var saved LayoutGeometryTable
	for _, tc := range []struct{ width, w float32 }{{200, 90}, {73, 60}, {20, 30}, {0, 30}} {
		used, _ := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: tc.width}, FontSizePx: 16})
		before := make(UsedValuesTable)
		for id, u := range used {
			before[id] = u
		}
		if !used[a.BoxID].WidthPending {
			t.Fatal("auto treated as resolved zero")
		}
		events := []string{}
		measure := measureFunc(func(r InlineMeasureRequest) (IntrinsicWidths, error) {
			events = append(events, "measure")
			if r.OwnerID != a.BoxID || r.Group != a.Children[0] {
				t.Fatal("wrong measure identity")
			}
			e, ok := r.Edges.Lookup(a.BoxID)
			if !ok || e.Padding.Left != 3 {
				t.Fatal("missing resolved owner edges")
			}
			e.Padding.Left = 999
			again, _ := r.Edges.Lookup(a.BoxID)
			if again.Padding.Left != 3 {
				t.Fatal("mutable edges")
			}
			if _, ok := r.Edges.Lookup(0); ok {
				t.Fatal("unknown edge")
			}
			if _, ok := r.Atomics.Lookup(a.BoxID); ok {
				t.Fatal("view contains itself")
			}
			return IntrinsicWidths{30, 90}, nil
		})
		inline := resultFunc(func(r InlineRequest) (InlineResult, error) {
			events = append(events, "layout")
			for id := range used {
				u, ok := r.Used.Lookup(id)
				if !ok || u.WidthPending || u.EdgesPending {
					t.Fatal("pending value leaked into layout")
				}
			}
			if r.OwnerID == a.BoxID {
				if r.ContentWidth != tc.w {
					t.Fatalf("width %g want %g", r.ContentWidth, tc.w)
				}
				return InlineResult{Extent: InlineExtent{W: tc.w, H: 180 / tc.w}}, nil
			}
			item, ok := r.Atomics.Lookup(a.BoxID)
			if !ok || item.BorderWidth != tc.w+8 {
				t.Fatal("border dimensions")
			}
			return InlineResult{Extent: InlineExtent{W: item.BorderWidth, H: item.BorderHeight}, Atomics: []AtomicPlacement{{BoxID: a.BoxID, X: 2, Y: 0}}}, nil
		})
		r, err := FlowLayout(root, used, inline, measure, LayoutContext{}, LayoutOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(events, []string{"measure", "layout", "layout"}) {
			t.Fatalf("phase order %v", events)
		}
		if !reflect.DeepEqual(root, original) || !reflect.DeepEqual(used, before) {
			t.Fatal("input mutation")
		}
		if previous != nil && !reflect.DeepEqual(previous.Geometry, saved) {
			t.Fatal("old snapshot changed")
		}
		previous = r
		saved = copyTestGeometry(r.Geometry)
	}
}

func TestNestedIntrinsicBoundsAndOneFinalLayout(t *testing.T) {
	b := newRenderElement(3, "inline-block", newRenderText(4, "b"))
	b.Style = &ComputedStyle{Width: lenAuto(), Padding: edges(lenPx(1)), Border: edges(lenPx(2)), Margin: EdgeLengths{Left: lenPx(2), Right: lenPx(2)}}
	a := newRenderElement(2, "inline-block", newRenderText(5, "aa"), newRenderElement(6, "inline", b))
	a.Style = &ComputedStyle{Width: lenAuto(), Padding: edges(lenPx(2)), Border: edges(lenPx(1))}
	root, _ := BuildLayoutTree(newRenderElement(1, "block", a), BuildOptions{})
	an := root.Children[0].Children[0]
	bn := an.Children[0].Children[1].Children[0]
	used, _ := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 80}})
	var measured []BoxID
	m := measureFunc(func(r InlineMeasureRequest) (IntrinsicWidths, error) {
		measured = append(measured, r.OwnerID)
		if r.OwnerID == bn.BoxID {
			if _, ok := r.Atomics.Lookup(an.BoxID); ok {
				t.Fatal("foreign item")
			}
			return IntrinsicWidths{30, 70}, nil
		}
		v, ok := r.Atomics.Lookup(bn.BoxID)
		if !ok || v.BorderWidths != (IntrinsicWidths{36, 76}) {
			t.Fatalf("nested border bounds %+v", v)
		}
		v.BorderWidths.MinContent = 999
		v, _ = r.Atomics.Lookup(bn.BoxID)
		if v.BorderWidths.MinContent != 36 {
			t.Fatal("mutable intrinsic view")
		}
		e, _ := r.Edges.Lookup(bn.BoxID)
		if horizontal(e.Margin) != 4 {
			t.Fatal("atomic margins unavailable")
		}
		return IntrinsicWidths{50, 90}, nil // text 10 + atomic 36..76 + margins 4
	})
	f := newAtomicFake(t)
	r, err := FlowLayout(root, used, f, m, LayoutContext{}, LayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(measured, []BoxID{bn.BoxID, an.BoxID}) {
		t.Fatalf("measurement order/duplicates %v", measured)
	}
	if !reflect.DeepEqual(f.calls, []BoxID{bn.BoxID, an.BoxID, root.BoxID}) {
		t.Fatalf("layout order %v", f.calls)
	}
	if r.Geometry[an.BoxID].Content.W != 74 || r.Geometry[bn.BoxID].Content.W != 64 {
		t.Fatal("wrong containing width or double edges")
	}
	if r.Geometry[bn.BoxID].RelativeTo != an.BoxID {
		t.Fatal("wrong coordinate owner")
	}
}

func TestIntrinsicBlocksAndDefinitePercentageBasis(t *testing.T) {
	nested := newRenderElement(5, "inline-block", newRenderText(6, "x"))
	nested.Style = &ComputedStyle{Width: lenAuto(), Padding: EdgeLengths{Left: lenPct(.1)}}
	fixed := newRenderElement(3, "block", nested)
	fixed.Style = &ComputedStyle{Width: lenPx(40), Padding: edges(lenPx(2)), Margin: EdgeLengths{Left: lenPx(2), Right: lenPx(2)}}
	flowing := newRenderElement(4, "block", newRenderText(7, "y"))
	flowing.Style = &ComputedStyle{Width: lenAuto(), Padding: edges(lenPx(1)), Margin: EdgeLengths{Left: lenPx(1), Right: lenPx(1)}}
	root, a := autoFixture(t, fixed, flowing)
	used, _ := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 77}})
	var calls int
	m := measureFunc(func(r InlineMeasureRequest) (IntrinsicWidths, error) { calls++; return IntrinsicWidths{10, 80}, nil })
	result, err := FlowLayout(root, used, newAtomicFake(t), m, LayoutContext{}, LayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Geometry[a.BoxID].Content.W != 64 {
		t.Fatal("block contributions summed instead of maximized")
	}
	fixedNode := a.Children[0]
	autoNode := fixedNode.Children[0].Children[0]
	if result.Geometry[autoNode.BoxID].Content.W != 36 || result.Geometry[autoNode.BoxID].Frame.W != 40 {
		t.Fatal("definite percentage basis not resolved after parent selection")
	}
	if result.Geometry[a.Children[1].BoxID].Content.W != 60 {
		t.Fatal("flow block descendant not resolved")
	}
	if calls != 2 {
		t.Fatalf("unexpected measure count %d", calls)
	}
}

func TestIntrinsicFailuresAndPercentageCycles(t *testing.T) {
	failure := errors.New("measurement failed")
	for _, tc := range []struct {
		name    string
		bounds  IntrinsicWidths
		err     error
		missing bool
	}{
		{name: "missing", missing: true}, {name: "wrapped", err: failure}, {name: "negative", bounds: IntrinsicWidths{-1, 2}},
		{name: "reversed", bounds: IntrinsicWidths{2, 1}}, {name: "nan", bounds: IntrinsicWidths{0, float32(math.NaN())}},
		{name: "infinite", bounds: IntrinsicWidths{0, float32(math.Inf(1))}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := autoFixture(t, newRenderText(3, "x"))
			used, _ := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 100}})
			var m IntrinsicMeasurer = measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) { return tc.bounds, tc.err })
			if tc.missing {
				m = nil
			}
			calls := 0
			r, err := FlowLayout(root, used, resultFunc(func(InlineRequest) (InlineResult, error) { calls++; return InlineResult{}, nil }), m, LayoutContext{}, LayoutOptions{})
			if err == nil || r != nil || calls != 0 {
				t.Fatal("invalid measurement reached layout")
			}
			if tc.err != nil && !errors.Is(err, failure) {
				t.Fatal("lost error")
			}
		})
	}
	for _, edge := range []bool{false, true} {
		child := newRenderElement(3, "block", newRenderText(4, "x"))
		child.Style = &ComputedStyle{Width: lenPct(.5)}
		if edge {
			child.Style = &ComputedStyle{Width: lenPx(20), Padding: EdgeLengths{Left: lenPct(.1)}}
		}
		root, a := autoFixture(t, child)
		used, _ := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 100}})
		u := used[a.Children[0].BoxID]
		if edge && !u.EdgesPending || !edge && !u.WidthPending {
			t.Fatal("unresolved percentage treated as zero")
		}
		calls := 0
		m := measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) { calls++; return IntrinsicWidths{}, nil })
		r, err := FlowLayout(root, used, nil, m, LayoutContext{}, LayoutOptions{})
		if err == nil || r != nil || calls != 0 {
			t.Fatal("percentage cycle reached measurer")
		}
	}
}

func TestEmptyAutoAndConcurrentReadOnlyMeasurement(t *testing.T) {
	root, a := autoFixture(t)
	used, _ := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 0}})
	before := copyTestTree(root)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := measureFunc(func(r InlineMeasureRequest) (IntrinsicWidths, error) {
				if len(r.Group.Children) != 0 {
					t.Error("not empty")
				}
				return IntrinsicWidths{}, nil
			})
			r, err := FlowLayout(root, used, newAtomicFake(t), m, LayoutContext{}, LayoutOptions{})
			if err != nil {
				t.Error(err)
				return
			}
			if r.Geometry[a.BoxID].Content.W != 0 || r.Geometry[a.BoxID].Frame.W != 8 {
				t.Error("empty zero width")
			}
		}()
	}
	wg.Wait()
	if !reflect.DeepEqual(root, before) || !used[a.BoxID].WidthPending {
		t.Fatal("shared input changed")
	}
}

func TestFixedIntrinsicAtomicAndMeasurementRecovery(t *testing.T) {
	b := newRenderElement(3, "inline-block", newRenderText(4, "unmeasured"))
	b.Style = &ComputedStyle{Width: lenPx(10), Padding: edges(lenPx(1))}
	root, a := autoFixture(t, newRenderElement(5, "inline", b))
	bn := a.Children[0].Children[0].Children[0]
	used, _ := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 100}})
	failure := errors.New("measurement abort")
	fail := false
	calls := 0
	m := measureFunc(func(r InlineMeasureRequest) (IntrinsicWidths, error) {
		calls++
		if r.OwnerID != a.BoxID {
			t.Fatal("fixed atomic interior measured unnecessarily")
		}
		item, ok := r.Atomics.Lookup(bn.BoxID)
		if !ok || item.BorderWidths != (IntrinsicWidths{12, 12}) {
			t.Fatal("fixed bounds differ")
		}
		if fail {
			return IntrinsicWidths{}, failure
		}
		return item.BorderWidths, nil
	})
	first, err := FlowLayout(root, used, newAtomicFake(t), m, LayoutContext{}, LayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	saved := copyTestGeometry(first.Geometry)
	fail = true
	f := newAtomicFake(t)
	r, err := FlowLayout(root, used, f, m, LayoutContext{}, LayoutOptions{})
	if r != nil || !errors.Is(err, failure) || len(f.calls) != 0 {
		t.Fatal("measurement failure published layout")
	}
	fail = false
	again, err := FlowLayout(root, used, newAtomicFake(t), m, LayoutContext{}, LayoutOptions{})
	if err != nil || !reflect.DeepEqual(first.Geometry, saved) || !reflect.DeepEqual(again.Geometry, saved) || calls != 3 {
		t.Fatal("measurement recovery changed snapshots")
	}
}

func TestIntrinsicMandatoryBreakFixture(t *testing.T) {
	// Typography semantics belong to the measurer. This fixture models 5px glyphs,
	// spaces as optional breaks, and newline as a mandatory max-content break.
	root, a := autoFixture(t, newRenderText(3, "aa bbbbb\nc"))
	used, _ := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 100}})
	textByNode := map[NodeID]string{3: "aa bbbbb\nc"}
	m := measureFunc(func(r InlineMeasureRequest) (IntrinsicWidths, error) {
		var w IntrinsicWidths
		for _, node := range r.Group.Children {
			text := textByNode[node.NodeID]
			for _, line := range strings.Split(text, "\n") {
				w.MaxContent = max(w.MaxContent, float32(len(line))*5)
				for _, word := range strings.Fields(line) {
					w.MinContent = max(w.MinContent, float32(len(word))*5)
				}
			}
		}
		if w != (IntrinsicWidths{25, 40}) {
			t.Fatalf("breaking bounds %+v", w)
		}
		return w, nil
	})
	r, err := FlowLayout(root, used, newAtomicFake(t), m, LayoutContext{}, LayoutOptions{})
	if err != nil || r.Geometry[a.BoxID].Content.W != 40 {
		t.Fatalf("max-content mandatory break: %v", err)
	}
}
