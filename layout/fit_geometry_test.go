package layout

import (
	"context"
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"
)

// One source byte per ten-pixel line, independently of receiving width.
func fitTenPixelLines(material map[BoxID]string) fitTestService {
	return func(_ context.Context, req ParagraphFitRequest, yield func(ParagraphAlternative) bool) (FitEnumeration, error) {
		body := material[req.Group]
		start := int(req.From.Offset)
		for end := min(start+1, len(body)); ; end++ {
			a := ParagraphAlternative{End: ParagraphPosition{Offset: uint64(end)}, Complete: end == len(body), Extent: InlineExtent{H: float32(end-start) * 10}, Output: &fitTestPayload{Bytes: []byte(body[start:end])}, AllowsMarginCollapse: len(body) == 0}
			if end > start {
				a.Extent.W = 1
			}
			if !a.Complete {
				a.Checkpoint = &fitTestPayload{End: uint64(end)}
			}
			if !yield(a) {
				return FitStopped, nil
			}
			if a.Complete {
				return FitExhausted, nil
			}
		}
	}
}

func TestStructuralFitSplitEdgesAndClosingCapacity(t *testing.T) {
	root := buildGroupFixture(t, newRenderElement(1, blockDisplay, newRenderText(2, "ABCDE")))
	s := DefaultComputedStyle()
	s.Margin = EdgeLengths{Top: lenPx(2), Bottom: lenPx(3)}
	s.Padding = EdgeLengths{Top: lenPx(4), Bottom: lenPx(6), Left: lenPx(2), Right: lenPx(2)}
	s.Border = EdgeLengths{Top: lenPx(2), Bottom: lenPx(3), Left: lenPx(1), Right: lenPx(1)}
	root.Style = &s
	before := copyTestTree(root)
	group := root.Children[0].BoxID
	f, err := NewStructuralFitter(root, fitTenPixelLines(map[BoxID]string{group: "ABCDE"}))
	if err != nil {
		t.Fatal(err)
	}
	var cursor FitContinuation
	for i, tc := range []struct {
		width, height, content, top, bottom float32
		text                                string
	}{
		{100, 28, 94, 8, 0, "AB"}, {80, 20, 74, 0, 0, "CD"}, {120, 22, 114, 0, 12, "E"},
	} {
		if i == 2 {
			if bad := collectFits(t, f, FitRequest{Width: 120, Height: 21, Continuation: cursor}); len(bad) != 0 {
				t.Fatal("closing edges deferred to an edge-only tail")
			}
		}
		all := collectFits(t, f, FitRequest{Width: tc.width, Height: tc.height, Continuation: cursor})
		if len(all) == 0 {
			t.Fatal("missing fit", i)
		}
		a := all[0]
		if fitText(a) != tc.text || a.Extent() != (InlineExtent{tc.width, tc.height}) || a.Complete() != (i == 2) {
			t.Fatalf("fragment %d: %q %+v", i, fitText(a), a.Extent())
		}
		u := a.UsedValues()[root.BoxID]
		if u.ContentWidth != tc.content || u.Margin.Top+u.Padding.Top+u.Border.Top != tc.top || u.Margin.Bottom+u.Padding.Bottom+u.Border.Bottom != tc.bottom {
			t.Fatalf("edges %d: %+v", i, u)
		}
		p := a.Paragraphs()[0]
		if p.X != 3 || p.Y != tc.top {
			t.Fatalf("paragraph origin %+v", p)
		}
		c := a.Containers()[0]
		if c.First != (i == 0) || c.Last != (i == 2) || c.Edges != (BoxEdges{u.Margin, u.Padding, u.Border}) {
			t.Fatalf("paint facts %+v", c)
		}
		// Detached inspection must not mutate retained geometry or future fitting.
		g := a.Geometry()
		delete(g, root.BoxID)
		values := a.UsedValues()
		values[root.BoxID] = UsedValues{}
		if a.Geometry()[root.BoxID] != c.Geometry || a.UsedValues()[root.BoxID] != u {
			t.Fatal("mutable geometry inspection")
		}
		cursor = a.Continuation()
	}
	if !reflect.DeepEqual(root, before) {
		t.Fatal("source styles changed")
	}
}

func TestStructuralFitNestedEdgesAndContainedChildren(t *testing.T) {
	first := newRenderElement(3, blockDisplay, newRenderText(4, "A"))
	style := DefaultComputedStyle()
	style.Padding.Top = lenPx(3)
	style.Border.Bottom = lenPx(3)
	first.Style = &style
	root := buildGroupFixture(t, newRenderElement(1, blockDisplay, newRenderElement(2, blockDisplay, first, newRenderElement(5, blockDisplay, newRenderText(6, "BCD")))))
	outer := DefaultComputedStyle()
	outer.Border.Top, outer.Border.Bottom = lenPx(2), lenPx(4)
	root.Style = &outer
	inner := DefaultComputedStyle()
	inner.Padding.Top, inner.Padding.Bottom = lenPx(3), lenPx(5)
	root.Children[0].Style = &inner
	groups := slices.Collect(InlineGroups(root))
	f, err := NewStructuralFitter(root, fitTenPixelLines(map[BoxID]string{groups[0].Group.BoxID: "A", groups[1].Group.BoxID: "BCD"}))
	if err != nil {
		t.Fatal(err)
	}
	a := collectFits(t, f, FitRequest{Width: 100, Height: 21})[0]
	if fitText(a) != "A" || a.Extent().H != 21 || a.Geometry()[groups[0].Owner.BoxID].Frame.H != 16 {
		t.Fatal("contained edges", fitText(a), a.Extent(), a.Geometry())
	}
	for _, c := range a.Containers() {
		if c.Box == groups[0].Owner.BoxID {
			if !c.First || !c.Last || c.Edges.Padding.Top != 3 || c.Edges.Border.Bottom != 3 {
				t.Fatal(c)
			}
		} else if !c.First || c.Last {
			t.Fatal(c)
		}
	}
	b := collectFits(t, f, FitRequest{Width: 80, Height: 20, Continuation: a.Continuation()})[0]
	if fitText(b) != "BC" || b.Extent().H != 20 {
		t.Fatal("middle", fitText(b), b.Extent())
	}
	c := collectFits(t, f, FitRequest{Width: 120, Height: 19, Continuation: b.Continuation()})[0]
	if !c.Complete() || fitText(c) != "D" || c.Extent().H != 19 {
		t.Fatal("last", fitText(c), c.Extent())
	}
	for _, box := range c.Containers() {
		if box.First || !box.Last {
			t.Fatal("last edges", box)
		}
	}
}

func TestStructuralFitPercentEmAndLocalContext(t *testing.T) {
	child := newRenderElement(2, blockDisplay, newRenderText(3, "A"))
	style := DefaultComputedStyle()
	style.Width = lenPct(.5)
	style.Padding.Left = lenPct(.1)
	style.Border.Right = Length{Kind: LenEm, Value: .5}
	style.FontSizePx = 10
	style.Margin.Left, style.Margin.Right = Length{Kind: LenAuto}, Length{Kind: LenAuto}
	child.Style = &style
	root := buildGroupFixture(t, newRenderElement(1, blockDisplay, child))
	rootStyle := DefaultComputedStyle()
	rootStyle.Padding = EdgeLengths{Left: lenPct(.1), Right: lenPct(.1)}
	root.Style = &rootStyle
	groups := slices.Collect(InlineGroups(root))
	f, err := NewStructuralFitter(root, fitTenPixelLines(map[BoxID]string{groups[0].Group.BoxID: "A"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ width, rootContent, childContent, padding, margin, x float32 }{{200, 160, 80, 16, 29.5, 65.5}, {100, 80, 40, 8, 13.5, 31.5}} {
		a := collectFits(t, f, FitRequest{Width: tc.width, Height: 10})[0]
		u := a.UsedValues()[groups[0].Owner.BoxID]
		if a.UsedValues()[root.BoxID].ContentWidth != tc.rootContent || u.ContentWidth != tc.childContent || u.Padding.Left != tc.padding || u.Border.Right != 5 || u.Margin.Left != tc.margin || a.Paragraphs()[0].X != tc.x {
			t.Fatalf("local context %+v %+v", u, a.Paragraphs()[0])
		}
	}
}

func TestStructuralFitSignedEmptyMarginsAndFrameBoundary(t *testing.T) {
	a := newRenderElement(2, blockDisplay, newRenderText(3, "A"))
	empty := newRenderElement(4, blockDisplay)
	b := newRenderElement(5, blockDisplay, newRenderText(6, "B"))
	styles := []ComputedStyle{DefaultComputedStyle(), DefaultComputedStyle(), DefaultComputedStyle()}
	styles[0].Margin.Bottom = lenPx(10)
	styles[1].Margin.Top = lenPx(20)
	styles[1].Margin.Bottom = lenPx(-5)
	styles[2].Margin.Top = lenPx(7)
	a.Style, empty.Style, b.Style = &styles[0], &styles[1], &styles[2]
	root := buildGroupFixture(t, newRenderElement(1, blockDisplay, a, empty, b))
	groups := slices.Collect(InlineGroups(root))
	f, err := NewStructuralFitter(root, fitTenPixelLines(map[BoxID]string{groups[0].Group.BoxID: "A", groups[2].Group.BoxID: "B"}))
	if err != nil {
		t.Fatal(err)
	}
	all := collectFits(t, f, FitRequest{Width: 100, Height: 35})
	full := all[0]
	if !full.Complete() || full.Extent().H != 35 || full.Paragraphs()[2].Y != 25 || full.Geometry()[groups[1].Owner.BoxID].Frame.Y != 30 {
		t.Fatal("collapsed signed strut", full.Extent(), full.Geometry(), full.Paragraphs())
	}
	stop, _ := f.Boundary(groups[0].Owner.BoxID, BoundaryAfter)
	prefix := collectFits(t, f, FitRequest{Width: 100, Height: 20, Stop: stop})[0]
	if prefix.Extent().H != 20 {
		t.Fatal("retained trailing margin", prefix.Extent())
	}
	suffix := collectFits(t, f, FitRequest{Width: 100, Height: 25, Continuation: prefix.Continuation()})[0]
	if !suffix.Complete() || suffix.Extent().H != 25 || suffix.Paragraphs()[1].Y != 15 {
		t.Fatal("cross-frame margin collapse", suffix.Extent(), suffix.Paragraphs())
	}
}

func TestStructuralFitInvalidGeometryAndSnapshot(t *testing.T) {
	root := buildGroupFixture(t, newRenderElement(1, blockDisplay, newRenderText(2, "A")))
	style := DefaultComputedStyle()
	style.Padding.Left = lenPct(math.MaxFloat32)
	root.Style = &style
	f, err := NewStructuralFitter(root, fitTenPixelLines(nil))
	if err != nil {
		t.Fatal(err)
	}
	status, err := f.Enumerate(context.Background(), FitRequest{Width: 100, Height: 100}, func(FitAlternative) bool { t.Fatal("invalid geometry published"); return true })
	if status != FitFailed || !errors.Is(err, ErrInvalidFit) {
		t.Fatal(status, err)
	}
	style = DefaultComputedStyle()
	style.Padding.Top = lenPx(2)
	f, err = NewStructuralFitter(root, fitTenPixelLines(map[BoxID]string{root.Children[0].BoxID: "A"}))
	if err != nil {
		t.Fatal(err)
	}
	style.Padding.Top = lenPx(1000)
	root.Children = nil
	a := collectFits(t, f, FitRequest{Width: 100, Height: 12})[0]
	if a.Extent().H != 12 || a.Paragraphs()[0].Y != 2 {
		t.Fatal("borrowed caller style")
	}
}

func TestStructuralFitEmRequiresEffectiveFont(t *testing.T) {
	root := buildGroupFixture(t, newRenderElement(1, blockDisplay, newRenderText(2, "A")))
	s := DefaultComputedStyle()
	s.Padding.Top = Length{Kind: LenEm, Value: 1}
	root.Style = &s
	if _, err := NewStructuralFitter(root, fitTenPixelLines(nil)); !errors.Is(err, ErrInvalidFit) {
		t.Fatal("missing font", err)
	}
	f, err := NewStructuralFitterWithOptions(root, fitTenPixelLines(map[BoxID]string{root.Children[0].BoxID: "A"}), StructuralFitOptions{FontSizePx: 12})
	if err != nil {
		t.Fatal(err)
	}
	a := collectFits(t, f, FitRequest{Width: 100, Height: 22})[0]
	if a.Paragraphs()[0].Y != 12 {
		t.Fatal("fallback font ignored")
	}
}
