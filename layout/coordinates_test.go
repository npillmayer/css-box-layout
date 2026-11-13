package layout

import (
	"reflect"
	"testing"
)

func TestFlowLayout_NestedContentRelativeCoordinates(t *testing.T) {
	leafA := newRenderElement(3, "block", newRenderText(30, "A"))
	leafA.Style = &ComputedStyle{Width: lenPx(80), Margin: EdgeLengths{Left: lenPx(2), Top: lenPx(6), Bottom: lenPx(4)}, Padding: EdgeLengths{Left: lenPx(3), Right: lenPx(1), Top: lenPx(2), Bottom: lenPx(3)}, Border: edges(lenPx(1))}
	leafB := newRenderElement(4, "block", newRenderText(40, "B"))
	leafB.Style = &ComputedStyle{Width: lenPx(90), Margin: EdgeLengths{Left: lenPx(-3), Top: lenPx(5), Bottom: lenPx(2)}}
	middle := newRenderElement(2, "block", leafA, leafB)
	middle.Style = &ComputedStyle{Width: lenPx(180), Margin: EdgeLengths{Left: lenPx(7), Top: lenPx(11), Bottom: lenPx(13)}, Padding: EdgeLengths{Left: lenPx(4), Right: lenPx(6), Top: lenPx(3), Bottom: lenPx(5)}, Border: EdgeLengths{Left: lenPx(1), Right: lenPx(2), Top: lenPx(2), Bottom: lenPx(1)}}
	tail := newRenderElement(5, "block", newRenderText(50, "tail"))
	tail.Style = &ComputedStyle{Width: lenPx(100), Margin: EdgeLengths{Left: lenPx(9), Top: lenPx(8), Bottom: lenPx(10)}, Padding: EdgeLengths{Left: lenPx(2), Top: lenPx(1), Bottom: lenPx(2)}}
	input := newRenderElement(1, "block", middle, tail)
	input.Style = &ComputedStyle{Width: lenPx(300), Padding: EdgeLengths{Left: lenPx(10), Right: lenPx(12), Top: lenPx(20), Bottom: lenPx(7)}, Border: EdgeLengths{Left: lenPx(5), Right: lenPx(3), Top: lenPx(2), Bottom: lenPx(4)}}
	root, err := BuildLayoutTree(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 800}})
	if err != nil {
		t.Fatal(err)
	}
	bySource := map[NodeID]*LayoutNode{}
	var index func(*LayoutNode)
	index = func(n *LayoutNode) {
		if n.Box == BoxBlock {
			bySource[n.NodeID] = n
		}
		for _, c := range n.Children {
			index(c)
		}
	}
	index(root)
	var calls []BoxID
	adapter := inlineFunc(func(req InlineRequest) (InlineExtent, error) {
		calls = append(calls, req.OwnerID)
		for _, tc := range []struct {
			id   NodeID
			w, h float32
		}{{3, 80, 10}, {4, 90, 12}, {5, 100, 20}} {
			if req.OwnerID == bySource[tc.id].BoxID {
				if req.ContentWidth != tc.w {
					t.Errorf("source %d: width %v, want %v", tc.id, req.ContentWidth, tc.w)
				}
				return InlineExtent{W: tc.w, H: tc.h}, nil
			}
		}
		t.Fatal("unexpected inline owner")
		return InlineExtent{}, nil
	})
	result, err := FlowLayout(root, used, adapter, nil, LayoutContext{}, LayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Independently calculated: middle content height is 6+17+4+5+12+2=46;
	// root content height is 11+57+13+8+23+10=122. Margins do not collapse.
	want := map[NodeID]LayoutGeometry{
		1: {Frame: Rect{W: 330, H: 155}, Content: Rect{X: 15, Y: 22, W: 300, H: 122}},
		2: {Frame: Rect{X: 7, Y: 11, W: 193, H: 57}, Content: Rect{X: 12, Y: 16, W: 180, H: 46}},
		3: {Frame: Rect{X: 2, Y: 6, W: 86, H: 17}, Content: Rect{X: 6, Y: 9, W: 80, H: 10}},
		4: {Frame: Rect{X: -3, Y: 32, W: 90, H: 12}, Content: Rect{X: -3, Y: 32, W: 90, H: 12}},
		5: {Frame: Rect{X: 9, Y: 89, W: 102, H: 23}, Content: Rect{X: 11, Y: 90, W: 100, H: 20}},
	}
	for id, expected := range want {
		n := bySource[id]
		got := result.Geometry[n.BoxID]
		if got != expected {
			t.Errorf("source %d: geometry=%+v, want %+v", id, got, expected)
		}
		if n.Frame != got.Frame || n.Content != got.Content {
			t.Errorf("source %d: node mirror disagrees", id)
		}
	}
	if !reflect.DeepEqual(calls, []BoxID{bySource[3].BoxID, bySource[4].BoxID, bySource[5].BoxID}) {
		t.Fatalf("unexpected paragraph order: %v", calls)
	}
	// A consumer composes content origins once; paragraph-local geometry is unchanged.
	rc := result.Geometry[root.BoxID].Content
	mc := result.Geometry[bySource[2].BoxID].Content
	ac := result.Geometry[bySource[3].BoxID].Content
	if x, y := rc.X+mc.X+ac.X, rc.Y+mc.Y+ac.Y; x != 33 || y != 47 {
		t.Errorf("paragraph A document origin=(%v,%v), want (33,47)", x, y)
	}
	// Changing root edges moves descendants in document space, not parent-local space.
	u := used[root.BoxID]
	u.Padding.Left += 100
	u.Padding.Top += 50
	used[root.BoxID] = u
	again, err := FlowLayout(root, used, adapter, nil, LayoutContext{}, LayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []NodeID{2, 3, 4, 5} {
		boxID := bySource[id].BoxID
		if again.Geometry[boxID] != result.Geometry[boxID] {
			t.Errorf("root edges changed source %d parent-local geometry", id)
		}
	}
}
