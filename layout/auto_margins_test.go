package layout

import (
	"reflect"
	"testing"
)

func TestBlockHorizontalMargins(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		kind                           BoxKind
		width, left, right             Length
		containing, padding, border    float32
		wantWidth, wantLeft, wantRight float32
	}{
		{"center", BoxBlock, lenPx(40), lenAuto(), lenAuto(), 100, 0, 0, 40, 30, 30},
		{"anonymous", BoxAnonymousBlock, lenPx(40), lenAuto(), lenAuto(), 100, 0, 0, 40, 30, 30},
		{"left auto", BoxBlock, lenPx(40), lenAuto(), lenPx(10), 100, 0, 0, 40, 50, 10},
		{"right auto", BoxBlock, lenPx(40), lenPx(10), lenAuto(), 100, 0, 0, 40, 10, 50},
		{"border box centered", BoxBlock, lenPx(40), lenAuto(), lenAuto(), 100, 5, 2, 40, 23, 23},
		{"negative right", BoxBlock, lenPx(40), lenAuto(), lenPx(-10), 100, 0, 0, 40, 70, -10},
		{"negative left", BoxBlock, lenPx(40), lenPx(-10), lenAuto(), 100, 0, 0, 40, -10, 70},
		{"overflow both auto", BoxBlock, lenPx(120), lenAuto(), lenAuto(), 100, 0, 0, 120, 0, -20},
		{"overflow left auto", BoxBlock, lenPx(120), lenAuto(), lenPx(10), 100, 0, 0, 120, 0, -20},
		{"overflow right auto", BoxBlock, lenPx(120), lenPx(10), lenAuto(), 100, 0, 0, 120, 10, -30},
		{"fixed margins residual", BoxBlock, lenPx(40), lenPx(10), lenPx(20), 100, 0, 0, 40, 10, 50},
		{"fixed margins overflow", BoxBlock, lenPx(120), lenPx(10), lenPx(20), 100, 0, 0, 120, 10, -30},
		{"large right margin with left auto", BoxBlock, lenPx(40), lenAuto(), lenPx(1e30), 100, 0, 0, 40, 0, 60},
		{"large fixed right margin", BoxBlock, lenPx(40), lenPx(10), lenPx(1e30), 100, 0, 0, 40, 10, 50},
		{"zero containing width", BoxBlock, lenPx(40), lenAuto(), lenAuto(), 0, 0, 0, 40, 0, -40},
		{"zero box", BoxBlock, lenPx(0), lenAuto(), lenAuto(), 100, 0, 0, 0, 50, 50},
		{"fractional", BoxBlock, lenPx(40.5), lenAuto(), lenAuto(), 101, 0, 0, 40.5, 30.25, 30.25},
		{"percent width", BoxBlock, lenPct(.5), lenAuto(), lenAuto(), 100, 0, 0, 50, 25, 25},
		{"em width", BoxBlock, lenEm(2), lenAuto(), lenAuto(), 100, 0, 0, 20, 40, 40},
		{"percent opposite margin", BoxBlock, lenPx(40), lenPct(.1), lenAuto(), 100, 0, 0, 40, 10, 50},
		{"auto width", BoxBlock, lenAuto(), lenAuto(), lenAuto(), 100, 0, 0, 100, 0, 0},
		{"auto width rounding", BoxBlock, lenAuto(), lenAuto(), lenAuto(), 100, .1, 0, 99.8, 0, 0},
		{"auto width with edges", BoxBlock, lenAuto(), lenAuto(), lenPx(10), 100, 5, 0, 80, 0, 10},
		{"auto width excessive edges", BoxBlock, lenAuto(), lenAuto(), lenAuto(), 100, 60, 0, 0, 0, -20},
		{"inline block", BoxInlineBlock, lenPx(40), lenAuto(), lenAuto(), 100, 0, 0, 40, 0, 0},
		{"auto inline block", BoxInlineBlock, lenAuto(), lenAuto(), lenAuto(), 100, 0, 0, 0, 0, 0},
		{"ordinary inline", BoxInline, lenPx(40), lenAuto(), lenAuto(), 100, 0, 0, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := ComputedStyle{Width: tc.width, FontSizePx: 10,
				Margin:  EdgeLengths{Left: tc.left, Right: tc.right, Top: lenAuto(), Bottom: lenAuto()},
				Padding: edges(lenPx(tc.padding)), Border: edges(lenPx(tc.border))}
			before := s
			n := &LayoutNode{BoxID: 1, Box: tc.kind, Style: &s}
			values, err := ResolveUsedValues(n, ResolveContext{ContainingBlock: Rect{W: tc.containing}})
			if err != nil {
				t.Fatal(err)
			}
			u := values[n.BoxID]
			if u.ContentWidth != tc.wantWidth || u.Margin != (Edges{Left: tc.wantLeft, Right: tc.wantRight}) {
				t.Fatalf("width=%g margins=%+v; want width=%g left=%g right=%g", u.ContentWidth, u.Margin, tc.wantWidth, tc.wantLeft, tc.wantRight)
			}
			if u.Padding != (Edges{Top: tc.padding, Right: tc.padding, Bottom: tc.padding, Left: tc.padding}) || u.Border != (Edges{Top: tc.border, Right: tc.border, Bottom: tc.border, Left: tc.border}) || s != before {
				t.Fatal("edges or computed style changed")
			}
			if IsBlockLevel(tc.kind) && u.Margin.Left+u.Border.Left+u.Padding.Left+u.ContentWidth+u.Padding.Right+u.Border.Right+u.Margin.Right != tc.containing {
				t.Fatal("normal-flow width constraint not satisfied")
			}
		})
	}
}

func TestAutoMarginsAfterIntrinsicWidthSelection(t *testing.T) {
	center := newRenderElement(3, blockDisplay, newRenderText(30, "center"))
	center.Style = &ComputedStyle{Width: lenPx(40), Margin: EdgeLengths{Left: lenAuto(), Right: lenAuto()}, Padding: edges(lenPx(2))}
	flowing := newRenderElement(4, blockDisplay, newRenderText(40, "flow"))
	flowing.Style = &ComputedStyle{Width: lenAuto(), Margin: EdgeLengths{Left: lenAuto(), Right: lenAuto()}}
	atomic := newRenderElement(2, inlineBlockDisplay, center, flowing)
	atomic.Style = &ComputedStyle{Width: lenAuto(), Margin: edges(lenAuto()), Padding: edges(lenPx(3)), Border: edges(lenPx(1))}
	root, err := BuildLayoutTree(newRenderElement(1, blockDisplay, atomic), BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	a := root.Children[0].Children[0]
	c, f := a.Children[0], a.Children[1]
	original := copyTestTree(root)
	var previous *LayoutResult
	var snapshot LayoutGeometryTable
	for _, tc := range []struct{ containing, width, left float32 }{{180, 100, 28}, {80, 72, 14}, {40, 44, 0}, {180, 100, 28}} {
		used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: tc.containing}})
		if err != nil {
			t.Fatal(err)
		}
		if !used[a.BoxID].WidthPending || !used[c.BoxID].EdgesPending || used[c.BoxID].Margin != (Edges{}) {
			t.Fatal("block margins treated as final before containing width is known", used[c.BoxID])
		}
		before := make(UsedValuesTable)
		for id, u := range used {
			before[id] = u
		}
		measures := 0
		m := measureFunc(func(r InlineMeasureRequest) (IntrinsicWidths, error) {
			measures++
			if r.OwnerID != f.BoxID {
				t.Fatal("unexpected intrinsic callback", r.OwnerID)
			}
			e, ok := r.Edges.Lookup(f.BoxID)
			if !ok || e.Margin != (Edges{}) {
				t.Fatal("distributed margins leaked into intrinsic measurement")
			}
			return IntrinsicWidths{20, 100}, nil
		})
		fake := newAtomicFake(t)
		inline := resultFunc(func(r InlineRequest) (InlineResult, error) {
			u, ok := r.Used.Lookup(c.BoxID)
			if !ok || u.EdgesPending || u.Margin.Left != tc.left || u.Margin.Right != tc.left || u.ContentWidth != 40 {
				t.Fatal("late block margins", u)
			}
			au, _ := r.Used.Lookup(a.BoxID)
			if au.Margin != (Edges{}) {
				t.Fatal("inline-block auto margins distributed", au)
			}
			return fake.LayoutInline(r)
		})
		r, err := FlowLayout(root, used, inline, m, LayoutContext{}, LayoutOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if measures != 1 || r.Geometry[a.BoxID].Content.W != tc.width || r.Geometry[a.BoxID].Frame.X != 0 || r.Geometry[c.BoxID].Frame.X != tc.left || r.Geometry[c.BoxID].Content.X != tc.left+2 || r.Geometry[c.BoxID].RelativeTo != a.BoxID {
			t.Fatal("incorrect final width or placement", r.Geometry)
		}
		if !reflect.DeepEqual(used, before) || !reflect.DeepEqual(root, original) {
			t.Fatal("mutated input")
		}
		if previous != nil && !reflect.DeepEqual(previous.Geometry, snapshot) {
			t.Fatal("changed previous result")
		}
		previous, snapshot = r, copyTestGeometry(r.Geometry)
	}
}
