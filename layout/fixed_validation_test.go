package layout

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestFixedSizingRejectsInvalidInputsBeforeMeasurement(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(**LayoutNode, *Rect, *float32)
	}{
		{"nil root", func(n **LayoutNode, _ *Rect, _ *float32) { *n = nil }},
		{"static root", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Style.Position = PositionStatic }},
		{"wrong root kind", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Box = BoxInlineBlock }},
		{"replaced root", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Replaced = &ReplacedContent{Height: 1} }},
		{"zero identity", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Children[0].BoxID = 0 }},
		{"duplicate identity", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Children[0].BoxID = (*n).BoxID }},
		{"cycle", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Children = []*LayoutNode{*n} }},
		{"nil child", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Children = []*LayoutNode{nil} }},
		{"unnormalized empty", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Children = nil }},
		{"block in inline", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Children[0].Children[0].Box = BoxBlock }},
		{"unknown box", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Children[0].Children[0].Box = BoxKind(99) }},
		{"empty text", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Children[0].Children[0].Text.Range.End = 0 }},
		{"text children", func(n **LayoutNode, _ *Rect, _ *float32) {
			(*n).Children[0].Children[0].Children = []*LayoutNode{{BoxID: 999, Box: BoxText}}
		}},
		{"nested fixed", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Children[0].Style.Position = PositionFixed }},
		{"invalid width", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Style.Width = lenPx(-1) }},
		{"invalid kind", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Style.Width.Kind = LengthKind(99) }},
		{"invalid padding", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Style.Padding.Left = lenAuto() }},
		{"invalid inset", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Style.Fixed.Insets.Left.Value = float32(math.NaN()) }},
		{"invalid height", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Style.Fixed.Height = lenPx(-1) }},
		{"bad computed font", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Style.FontSizePx = -1 }},
		{"bad white space", func(n **LayoutNode, _ *Rect, _ *float32) { (*n).Style.WhiteSpace = WhiteSpace(99) }},
		{"negative viewport width", func(_ **LayoutNode, v *Rect, _ *float32) { v.W = -1 }},
		{"negative viewport height", func(_ **LayoutNode, v *Rect, _ *float32) { v.H = -1 }},
		{"nan viewport origin", func(_ **LayoutNode, v *Rect, _ *float32) { v.X = float32(math.NaN()) }},
		{"infinite viewport", func(_ **LayoutNode, v *Rect, _ *float32) { v.H = float32(math.Inf(1)) }},
		{"bad fallback font", func(_ **LayoutNode, _ *Rect, f *float32) { *f = -1 }},
		{"nan fallback font", func(_ **LayoutNode, _ *Rect, f *float32) { *f = float32(math.NaN()) }},
		{"missing em font", func(n **LayoutNode, _ *Rect, f *float32) { *f = 0; (*n).Style.Fixed.Insets.Left = lenEm(1) }},
		{"missing descendant em font", func(n **LayoutNode, _ *Rect, f *float32) {
			*f = 0
			(*n).Children[0].Children[0].Style.Margin.Left = lenEm(1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixedSizingRoot(t, anchoredFixedStyle(), newRenderText(2, "x"))
			viewport, font := Rect{W: 200, H: 100}, float32(16)
			tc.edit(&root, &viewport, &font)
			calls := 0
			got, err := resolveFixedSizing(root, viewport, font, measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) { calls++; return IntrinsicWidths{10, 20}, nil }))
			if err == nil || got != nil || calls != 0 {
				t.Fatalf("got %+v, %v, calls=%d", got, err, calls)
			}
		})
	}
	for _, axis := range []string{"horizontal", "vertical", "nil constraints"} {
		s := anchoredFixedStyle()
		s.Width = lenPx(40)
		if axis == "horizontal" {
			s.Fixed.Insets.Left = lenAuto()
		}
		if axis == "vertical" {
			s.Fixed.Insets.Top = lenAuto()
		}
		if axis == "nil constraints" {
			s.Fixed = nil
		}
		got, err := resolveFixedSizing(fixedSizingRoot(t, s), Rect{W: 100, H: 100}, 16, nil)
		if got != nil || !errors.Is(err, ErrUnsupportedPosition) {
			t.Fatalf("static fallback was accepted: %v", err)
		}
	}
}

func TestFixedSizingNumericLimits(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*ComputedStyle)
	}{
		{"percentage overflow", func(s *ComputedStyle) { s.Width = lenPct(math.MaxFloat32) }},
		{"inset overflow", func(s *ComputedStyle) { s.Fixed.Insets.Top = lenPct(math.MaxFloat32) }},
		{"edge overflow", func(s *ComputedStyle) { s.Padding.Left = lenEm(math.MaxFloat32) }},
		{"border width overflow", func(s *ComputedStyle) { s.Width = lenPx(math.MaxFloat32); s.Padding.Right = lenPx(math.MaxFloat32) }},
		{"inset solution overflow", func(s *ComputedStyle) { s.Width = lenPx(math.MaxFloat32); s.Fixed.Insets.Left = lenPx(math.MaxFloat32) }},
		{"origin overflow", func(s *ComputedStyle) {
			s.Fixed.Insets.Left = lenPx(math.MaxFloat32)
			s.Margin.Left = lenPx(math.MaxFloat32)
		}},
		{"height overflow", func(s *ComputedStyle) {
			s.Fixed.Height = lenPx(math.MaxFloat32)
			s.Padding.Bottom = lenPx(math.MaxFloat32)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := anchoredFixedStyle()
			s.Width = lenPx(10)
			s.Fixed.Height = lenPx(10)
			tc.edit(&s)
			if got, err := resolveFixedSizing(fixedSizingRoot(t, s), Rect{W: 200, H: 100}, 16, nil); got != nil || err == nil {
				t.Fatalf("overflow accepted: %+v %v", got, err)
			}
		})
	}
	s := anchoredFixedStyle()
	s.Width = lenPx(10)
	s.Padding.Bottom = lenPx(math.MaxFloat32)
	pending, err := resolveFixedSizing(fixedSizingRoot(t, s), Rect{W: 200, H: 100}, 16, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := pending.withContentHeight(math.MaxFloat32); err == nil || got != nil {
		t.Fatal("completion overflow accepted")
	}
	// A signed intermediate exceeds float32, but every resulting value fits.
	s = anchoredFixedStyle()
	s.Width = lenPx(1e38)
	s.Fixed.Height = lenPx(0)
	s.Fixed.Insets.Left = lenPx(-1e38)
	s.Margin.Left = lenPx(1e38)
	got, err := resolveFixedSizing(fixedSizingRoot(t, s), Rect{W: 3e38, H: 0}, 0, nil)
	if err != nil || math.Abs(float64(got.Horizontal.End)/2e38-1) > 1e-6 {
		t.Fatalf("finite signed calculation failed: %+v %v", got, err)
	}
}

func TestFixedSizingIntrinsicContracts(t *testing.T) {
	for _, bounds := range []IntrinsicWidths{{-1, 2}, {3, 2}, {0, float32(math.Inf(1))}, {float32(math.NaN()), 2}} {
		root := fixedSizingRoot(t, anchoredFixedStyle(), newRenderText(2, "x"))
		got, err := resolveFixedSizing(root, Rect{W: 200, H: 100}, 16, measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) { return bounds, nil }))
		if got != nil || err == nil {
			t.Fatal("invalid intrinsic bounds accepted")
		}
	}
	root := fixedSizingRoot(t, anchoredFixedStyle(), newRenderText(2, "x"))
	if got, err := resolveFixedSizing(root, Rect{W: 200, H: 100}, 16, nil); got != nil || err == nil {
		t.Fatal("missing intrinsic service accepted")
	}
	// Unknown-width percentage dependencies retain the existing explicit rejection.
	child := newRenderElement(2, blockDisplay, newRenderText(3, "x"))
	child.Style = &ComputedStyle{Width: lenPct(.5)}
	if got, err := resolveFixedSizing(fixedSizingRoot(t, anchoredFixedStyle(), child), Rect{W: 200, H: 100}, 16, nil); got != nil || err == nil {
		t.Fatal("unknown percentage basis accepted")
	}
	// A definite child can supply intrinsic bounds without a text callback.
	child.Style.Width = lenPx(35)
	got, err := resolveFixedSizing(fixedSizingRoot(t, anchoredFixedStyle(), child), Rect{W: 200, H: 100}, 16, nil)
	if err != nil || got.Used.ContentWidth != 35 {
		t.Fatalf("definite child bounds: %+v %v", got, err)
	}
}

func TestFixedSizingNestedIntrinsicMeasurement(t *testing.T) {
	atomic := newRenderElement(2, inlineBlockDisplay, newRenderText(3, "nested"))
	atomic.Style = &ComputedStyle{Width: lenAuto(), Padding: edges(lenPx(2)), FontSizePx: 20}
	root := fixedSizingRoot(t, anchoredFixedStyle(), atomic, newRenderText(4, "outer"))
	inner := root.Children[0].Children[0]
	var calls []BoxID
	measure := measureFunc(func(req InlineMeasureRequest) (IntrinsicWidths, error) {
		calls = append(calls, req.OwnerID)
		if req.OwnerID == inner.BoxID {
			return IntrinsicWidths{20, 60}, nil
		}
		a, ok := req.Atomics.Lookup(inner.BoxID)
		if !ok || a.BorderWidths != (IntrinsicWidths{24, 64}) {
			t.Fatalf("nested bounds: %+v", a)
		}
		return IntrinsicWidths{34, 74}, nil
	})
	got, err := resolveFixedSizing(root, Rect{W: 50, H: 100}, 16, measure)
	if err != nil || got.Used.ContentWidth != 50 || !reflect.DeepEqual(calls, []BoxID{inner.BoxID, root.BoxID}) {
		t.Fatalf("nested sizing: %+v %v %v", got, err, calls)
	}
	// The descendant profile remains restricted even when the root has definite width.
	inner.Style.Width = lenPct(.5)
	root.Style.Width = lenPx(100)
	calls = nil
	if got, err := resolveFixedSizing(root, Rect{W: 200, H: 100}, 16, measure); got != nil || err == nil || len(calls) != 0 {
		t.Fatal("unsupported atomic width reached measurement")
	}
}
