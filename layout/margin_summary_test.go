package layout

import (
	"math"
	"reflect"
	"testing"
)

func TestMarginStrut(t *testing.T) {
	for _, tc := range []struct {
		name    string
		margins []float32
		want    float32
	}{
		{"empty", nil, 0},
		{"positive", []float32{13, 8}, 13},
		{"mixed", []float32{13, -8}, 5},
		{"negative", []float32{-13, -8}, -13},
		{"preserve extrema", []float32{13, -8, 10}, 5},
		{"zero", []float32{0, 0}, 0},
		{"finite extremes", []float32{math.MaxFloat32, -math.MaxFloat32}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got marginStrut
			for _, m := range tc.margins {
				got = got.merge(strutFromMargin(m))
			}
			if got.value() != tc.want {
				t.Fatalf("strut=%+v, value=%v, want %v", got, got.value(), tc.want)
			}
		})
	}
	// Grouping and visitation order must not change an adjoining group's result.
	for _, a := range []float32{-13, 0, 10, 13} {
		for _, b := range []float32{-8, 0, 8} {
			for _, c := range []float32{-20, 0, 10} {
				x, y, z := strutFromMargin(a), strutFromMargin(b), strutFromMargin(c)
				before := [3]marginStrut{x, y, z}
				left, right := x.merge(y).merge(z), x.merge(y.merge(z))
				if left != right || left != z.merge(y).merge(x) || x.merge(x) != x || x.merge(marginStrut{}) != x {
					t.Fatalf("invalid merge algebra for %v", before)
				}
				if before != [3]marginStrut{x, y, z} {
					t.Fatal("merge mutated operands")
				}
			}
		}
	}
}

func TestRecursiveMarginSummaries(t *testing.T) {
	for _, tc := range []struct {
		name         string
		root, atomic bool
		paddingTop   float32
		borderBottom float32
		leading      marginStrut
		trailing     marginStrut
	}{
		{name: "ordinary", leading: marginStrut{13, -8}, trailing: marginStrut{4, -6}},
		{name: "root boundary", root: true, leading: marginStrut{13, 0}, trailing: marginStrut{0, -3}},
		{name: "atomic boundary", atomic: true, leading: marginStrut{13, 0}, trailing: marginStrut{0, -3}},
		{name: "top barrier", paddingTop: 1, leading: marginStrut{13, 0}, trailing: marginStrut{4, -6}},
		{name: "bottom barrier", borderBottom: 1, leading: marginStrut{13, -8}, trailing: marginStrut{0, -3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			empty := newRenderElement(2, "block")
			empty.Style = &ComputedStyle{Margin: EdgeLengths{Top: lenPx(-8), Bottom: lenPx(10)}}
			solid := newRenderElement(3, "block", newRenderText(30, "x"))
			solid.Style = &ComputedStyle{Margin: EdgeLengths{Top: lenPx(7), Bottom: lenPx(600)}}
			lastSolid := newRenderElement(5, "block", newRenderText(50, "y"))
			lastSolid.Style = &ComputedStyle{Margin: EdgeLengths{Top: lenPx(900), Bottom: lenPx(-6)}}
			tail := newRenderElement(4, "block")
			tail.Style = &ComputedStyle{Margin: EdgeLengths{Top: lenPx(4), Bottom: lenPx(-2)}}
			// The large internal margins must not leak into either outside group.
			input := newRenderElement(1, "block", empty, solid, lastSolid, tail)
			input.Style = &ComputedStyle{Width: lenPx(100), Margin: EdgeLengths{Top: lenPx(13), Bottom: lenPx(-3)},
				Padding: EdgeLengths{Top: lenPx(tc.paddingTop)}, Border: EdgeLengths{Bottom: lenPx(tc.borderBottom)}}
			n, err := BuildLayoutTree(input, BuildOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if tc.atomic {
				n.Box = BoxInlineBlock
			}
			u, err := ResolveUsedValues(n, ResolveContext{ContainingBlock: Rect{W: 100}})
			if err != nil {
				t.Fatal(err)
			}
			original := copyTestTree(n)
			before := make(UsedValuesTable)
			for id, v := range u {
				before[id] = v
			}
			g := make(LayoutGeometryTable)
			adapter := newAtomicFake(t)
			summary, err := layoutBlockContainer(n, u, g, adapter, nil, tc.root)
			if err != nil {
				t.Fatal(err)
			}
			if summary.Leading != tc.leading || summary.Trailing != tc.trailing || summary.Through {
				t.Fatalf("summary=%+v, want leading=%+v trailing=%+v, non-through", summary, tc.leading, tc.trailing)
			}
			// Two 10px lines plus the internal max(600,900) gap.
			// Closed edges retain leading +2 and trailing -2 respectively.
			wantContent := float32(920)
			if tc.root || tc.atomic || tc.paddingTop > 0 {
				wantContent += 2
			}
			if tc.root || tc.atomic || tc.borderBottom > 0 {
				wantContent -= 2
			}
			if summary.BorderWidth != g[n.BoxID].Frame.W || summary.BorderHeight != wantContent+tc.paddingTop+tc.borderBottom || g[n.BoxID].Content.H != wantContent {
				t.Fatalf("summary dimensions disagree with collapsed layout: %+v, %+v", summary, g[n.BoxID])
			}
			if len(adapter.calls) != 4 || !reflect.DeepEqual(n, original) || !reflect.DeepEqual(u, before) {
				t.Fatal("summary changed callback count or inputs")
			}
			againGeometry := make(LayoutGeometryTable)
			again, err := layoutBlockContainer(n, u, againGeometry, newAtomicFake(t), nil, tc.root)
			if err != nil || again != summary || !reflect.DeepEqual(g, againGeometry) {
				t.Fatalf("repeated layout changed summary or geometry: %+v, %v", again, err)
			}
		})
	}
}

func TestThroughMarginSummaryAndZeroHeightBarrier(t *testing.T) {
	leaf := newRenderElement(3, "block")
	leaf.Style = &ComputedStyle{Margin: EdgeLengths{Top: lenPx(-8), Bottom: lenPx(10)}}
	middle := newRenderElement(2, "block", leaf)
	middle.Style = &ComputedStyle{Margin: EdgeLengths{Top: lenPx(13), Bottom: lenPx(-3)}}
	input := newRenderElement(1, "block", middle)
	input.Style = &ComputedStyle{Margin: EdgeLengths{Top: lenPx(2), Bottom: lenPx(4)}}
	n, err := BuildLayoutTree(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	u, err := ResolveUsedValues(n, ResolveContext{ContainingBlock: Rect{W: 100}})
	if err != nil {
		t.Fatal(err)
	}
	for _, allow := range []bool{true, false} {
		adapter := resultFunc(func(InlineRequest) (InlineResult, error) {
			return InlineResult{AllowsMarginCollapse: allow}, nil
		})
		s, err := layoutBlockContainer(n, u, make(LayoutGeometryTable), adapter, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		if s.Through != allow || s.Leading != (marginStrut{13, -8}) {
			t.Fatalf("allow=%v: summary=%+v", allow, s)
		}
		wantTrailing := marginStrut{10, -3}
		if allow {
			wantTrailing = s.Leading
		}
		if s.Trailing != wantTrailing {
			t.Fatalf("allow=%v: trailing=%+v, want %+v", allow, s.Trailing, wantTrailing)
		}
	}
}
