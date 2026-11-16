package layout

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// Every text-bearing test block consumes 10px; empty groups explicitly permit
// collapsing. Use the real builder so anonymous groups and owners are exercised.
func marginTestBlock(id NodeID, top, bottom float32, text bool, children ...*RenderNode) *RenderNode {
	if text {
		children = append(children, newRenderText(id+100, "x"))
	}
	n := newRenderElement(id, blockDisplay, children...)
	n.Style = &ComputedStyle{Width: lenPx(100), Margin: EdgeLengths{Top: lenPx(top), Bottom: lenPx(bottom)}}
	return n
}

func marginTestLayout(t *testing.T, input *RenderNode) (map[NodeID]LayoutGeometry, *atomicFake) {
	t.Helper()
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
	adapter := newAtomicFake(t)
	result, err := FlowLayout(root, used, adapter, nil, LayoutContext{}, LayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(root, original) || !reflect.DeepEqual(used, before) {
		t.Fatal("layout mutated inputs")
	}
	out := make(map[NodeID]LayoutGeometry)
	var visit func(*LayoutNode)
	visit = func(n *LayoutNode) {
		if g, ok := result.Geometry[n.BoxID]; ok {
			out[n.NodeID] = g
		}
		for _, c := range n.Children {
			visit(c)
		}
	}
	visit(root)
	return out, adapter
}

func TestCollapsedSiblingPlacement(t *testing.T) {
	for _, tc := range []struct {
		name             string
		bottom, top, gap float32
	}{
		{"positive", 13, 8, 13}, {"mixed", 13, -8, 5},
		{"negative", -13, -8, -13}, {"zero", 0, 0, 0},
		{"negative height clamped", -40, -30, -40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := marginTestBlock(1, 0, 0, false,
				marginTestBlock(2, 0, tc.bottom, true), marginTestBlock(3, tc.top, 0, true))
			g, adapter := marginTestLayout(t, root)
			if g[2].Frame.Y != 0 || g[3].Frame.Y != 10+tc.gap || g[1].Content.H != max(0, 20+tc.gap) {
				t.Fatalf("incorrect sibling placement: %+v", g)
			}
			if len(adapter.calls) != 2 {
				t.Fatal("inline layout repeated")
			}
		})
	}
}

func TestThroughBoxOriginsAndPendingGroups(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		trailing, nested     bool
		y2, y3, next, height float32
	}{
		// A: H10, bottom5; E: top7,bottom30; F: top-12,bottom3; B: top50,H10.
		{name: "consecutive empties", y2: 17, y3: 28, next: 48, height: 58},
		{name: "trailing empties", trailing: true, y2: 17, y3: 28, height: 28},
		// Nested E contains an empty child with top9,bottom11. Its hypothetical
		// bottom-border leading group is 11, while its completed group remains 30.
		{name: "nested empty", nested: true, y2: 21, y3: 28, next: 48, height: 58},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := marginTestBlock(3, 7, 30, false)
			if tc.nested {
				e = marginTestBlock(3, 7, 30, false, marginTestBlock(6, 9, 11, false))
			}
			children := []*RenderNode{marginTestBlock(2, 0, 5, true), e, marginTestBlock(4, -12, 3, false)}
			if !tc.trailing {
				children = append(children, marginTestBlock(5, 50, 0, true))
			}
			g, _ := marginTestLayout(t, marginTestBlock(1, 0, 0, false, children...))
			if g[3].Frame.Y != tc.y2 || g[4].Frame.Y != tc.y3 || g[1].Content.H != tc.height {
				t.Fatalf("incorrect through origins/height: %+v", g)
			}
			if !tc.trailing && g[5].Frame.Y != tc.next {
				t.Fatalf("pending group applied incorrectly: %+v", g[5])
			}
			if tc.nested && g[6].Frame.Y != 0 {
				t.Fatalf("empty descendant must share parent's border origin: %+v", g[6])
			}
		})
	}
}

func TestParentChildCollapsedHeight(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		top, bottom                      float32
		empty                            bool
		parentY, childY, contentH, rootH float32
	}{
		{name: "open", parentY: 20, childY: 0, contentH: 10, rootH: 37},
		{name: "top padding", top: 1, parentY: 13, childY: 20, contentH: 30, rootH: 51},
		{name: "bottom padding", bottom: 1, parentY: 20, childY: 0, contentH: 17, rootH: 43},
		{name: "both padding", top: 1, bottom: 1, parentY: 13, childY: 20, contentH: 37, rootH: 57},
		{name: "empty open", empty: true, parentY: 20, childY: 0, contentH: 0, rootH: 20},
		{name: "empty top padding", empty: true, top: 1, parentY: 13, childY: 20, contentH: 0, rootH: 34},
		{name: "empty bottom padding", empty: true, bottom: 1, parentY: 20, childY: 0, contentH: 0, rootH: 26},
		{name: "empty both padding", empty: true, top: 1, bottom: 1, parentY: 13, childY: 20, contentH: 20, rootH: 40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := marginTestBlock(2, 13, 5, false, marginTestBlock(3, 20, 7, !tc.empty))
			parent.Style.Padding = EdgeLengths{Top: lenPx(tc.top), Bottom: lenPx(tc.bottom)}
			g, _ := marginTestLayout(t, marginTestBlock(1, 99, 99, false, parent))
			if g[1].Frame.Y != 0 || g[2].Frame.Y != tc.parentY || g[3].Frame.Y != tc.childY || g[2].Content.H != tc.contentH || g[1].Content.H != tc.rootH {
				t.Fatalf("incorrect parent/child geometry: %+v", g)
			}
			if g[2].Frame.H != tc.contentH+tc.top+tc.bottom {
				t.Fatal("edges missing from border height")
			}
		})
	}
}

func TestCollapsedInlineBlockPreparation(t *testing.T) {
	atomic := marginTestBlock(2, 3, 4, false,
		marginTestBlock(3, 6, 13, true), marginTestBlock(4, 8, 7, true))
	atomic.Display = inlineBlockDisplay
	g, adapter := marginTestLayout(t, marginTestBlock(1, 0, 0, false, atomic))
	// Atomic interior: 6+10+max(13,8)+10+7 = 46; outside margins stay separate.
	if g[2].Content.H != 46 || g[3].Frame.Y != 6 || g[4].Frame.Y != 29 || g[1].Content.H != 53 {
		t.Fatalf("incorrect atomic containment/preparation: %+v", g)
	}
	if len(adapter.calls) != 3 {
		t.Fatal("atomic or paragraph laid out more than once")
	}
}

func TestCollapsedCoordinateOverflow(t *testing.T) {
	input := marginTestBlock(1, 0, 0, false,
		marginTestBlock(2, math.MaxFloat32, 0, true), marginTestBlock(3, math.MaxFloat32, 0, true))
	root, err := BuildLayoutTree(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 100}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := FlowLayout(root, used, newAtomicFake(t), nil, LayoutContext{}, LayoutOptions{})
	if result != nil || err == nil || !strings.Contains(err.Error(), "coordinate overflow") {
		t.Fatalf("result=%v err=%v", result, err)
	}
}
