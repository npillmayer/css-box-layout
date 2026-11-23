package layout

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestMeasureIntrinsicStandalone(t *testing.T) {
	root, atom := autoFixture(t, newRenderText(3, "text"))
	original := copyTestTree(root)
	var order []BoxID
	bounds := IntrinsicWidths{10, 30}
	failure := errors.New("late group failure")
	fail := false
	measure := measureFunc(func(r InlineMeasureRequest) (IntrinsicWidths, error) {
		order = append(order, r.OwnerID)
		if r.OwnerID == atom.BoxID {
			return bounds, nil
		}
		if fail {
			return IntrinsicWidths{999, 999}, failure
		}
		a, ok := r.Atomics.Lookup(atom.BoxID)
		e, found := r.Edges.Lookup(atom.BoxID)
		if !ok || !found || a.BorderWidths != (IntrinsicWidths{bounds.MinContent + 8, bounds.MaxContent + 8}) {
			t.Fatal("missing core border bounds", a, e)
		}
		return IntrinsicWidths{a.BorderWidths.MinContent + e.Margin.Left + e.Margin.Right, a.BorderWidths.MaxContent + e.Margin.Left + e.Margin.Right}, nil
	})
	ctx := ResolveContext{ContainingBlock: Rect{W: 100}, FontSizePx: 16}
	first, err := MeasureIntrinsic(root, ctx, measure)
	if err != nil || first != (IntrinsicWidths{23, 43}) || !reflect.DeepEqual(order, []BoxID{atom.BoxID, root.BoxID}) {
		t.Fatal(first, order, err)
	}
	fail = true
	if got, err := MeasureIntrinsic(root, ctx, measure); got != (IntrinsicWidths{}) || !errors.Is(err, failure) {
		t.Fatal("partial result or lost cause", got, err)
	}
	fail, bounds = false, IntrinsicWidths{20, 50}
	got, err := MeasureIntrinsic(root, ctx, measure)
	if err != nil || got != (IntrinsicWidths{33, 63}) || first != (IntrinsicWidths{23, 43}) || !reflect.DeepEqual(root, original) {
		t.Fatal("cached answer, poisoned retry or input mutation", got, err)
	}
}

func TestMeasureIntrinsicDefiniteRootAndFailures(t *testing.T) {
	root, _ := BuildLayoutTree(newRenderElement(1, blockDisplay, newRenderText(2, "x")), BuildOptions{})
	root.Style.Width = lenPct(.5)
	root.Style.Padding.Left = lenPct(.1)
	root.Style.Margin.Right = lenPx(99)
	for _, width := range []float32{100, 200, 0} {
		got, err := MeasureIntrinsic(root, ResolveContext{ContainingBlock: Rect{W: width}}, nil)
		if err != nil || got != (IntrinsicWidths{width/2 + width/10, width/2 + width/10}) {
			t.Fatal("percentage basis or root margin", got, err)
		}
	}
	if got, err := MeasureIntrinsic(nil, ResolveContext{}, nil); err != nil || got != (IntrinsicWidths{}) {
		t.Fatal(got, err)
	}
	for _, mutate := range []func(*LayoutNode, *ResolveContext){
		func(_ *LayoutNode, c *ResolveContext) { c.ContainingBlock.W = -1 },
		func(_ *LayoutNode, c *ResolveContext) { c.ContainingBlock.W = float32(math.NaN()) },
		func(_ *LayoutNode, c *ResolveContext) { c.FontSizePx = float32(math.Inf(1)) },
		func(n *LayoutNode, _ *ResolveContext) { n.Box = BoxInline },
		func(n *LayoutNode, _ *ResolveContext) { n.Style.Position = PositionFixed },
		func(n *LayoutNode, _ *ResolveContext) { n.Children[0].BoxID = n.BoxID },
		func(n *LayoutNode, _ *ResolveContext) { n.Children = append(n.Children, n) },
		func(n *LayoutNode, _ *ResolveContext) { n.Style.Padding.Left.Value = float32(math.Inf(1)) },
	} {
		n := copyTestTree(root)
		ctx := ResolveContext{}
		mutate(n, &ctx)
		calls := 0
		got, err := MeasureIntrinsic(n, ctx, measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) { calls++; return IntrinsicWidths{}, nil }))
		if got != (IntrinsicWidths{}) || err == nil || calls != 0 {
			t.Fatal("invalid input reached measurement", got, calls, err)
		}
	}
}

func TestMeasureIntrinsicAutoPercentageCycle(t *testing.T) {
	root, atom := autoFixture(t, newRenderText(3, "text"))
	atom.Style.Padding.Left = lenPct(.1)
	calls := 0
	got, err := MeasureIntrinsic(root, ResolveContext{ContainingBlock: Rect{W: 100}}, measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) { calls++; return IntrinsicWidths{}, nil }))
	if got != (IntrinsicWidths{}) || err == nil || calls != 0 {
		t.Fatal("auto descendant borrowed external percentage basis", got, calls, err)
	}
}
