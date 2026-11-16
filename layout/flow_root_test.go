package layout

import (
	"errors"
	"reflect"
	"testing"
)

// These numeric expectations use CSS Display's flow-root BFC and CSS 2.2
// 8.3.1 adjacency rules. The fake paragraph consumes 10px for text and explicitly
// allows collapse for empty output; it does not calculate block margins.
func TestFlowRootContainsChildMargins(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		display                Display
		top, bottom            float32
		parentY, childY, h, rh float32
		calls                  int
	}{
		{"ordinary", blockDisplay, 20, 7, 20, 0, 10, 37, 1},
		{"flow-root", flowRootDisplay, 20, 7, 13, 20, 37, 55, 1},
		{"inline-block control", inlineBlockDisplay, 20, 7, 13, 20, 37, 55, 2},
		{"ordinary negative children", blockDisplay, -8, -6, 5, 0, 10, 14, 1},
		{"flow-root negative children", flowRootDisplay, -8, -6, 13, -8, 0, 18, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := marginTestBlock(2, 13, 5, false, marginTestBlock(3, tc.top, tc.bottom, true))
			parent.Display = tc.display
			g, adapter := marginTestLayout(t, marginTestBlock(1, 0, 0, false, parent))
			assertFlowRootGeometry(t, g, map[NodeID][2]float32{
				1: {0, tc.rh}, 2: {tc.parentY, tc.h}, 3: {tc.childY, 10},
			})
			if len(adapter.calls) != tc.calls {
				t.Fatalf("paragraph calls = %d, want %d", len(adapter.calls), tc.calls)
			}
		})
	}
}

func TestFlowRootPreservesSiblingCollapseInsideAndOutside(t *testing.T) {
	for _, tc := range []struct {
		display                 Display
		firstY, lastY, h, nextY float32
	}{
		{blockDisplay, 0, 23, 33, 64},
		{flowRootDisplay, 6, 29, 46, 77},
	} {
		t.Run(tc.display.String(), func(t *testing.T) {
			container := marginTestBlock(3, 3, 4, false,
				marginTestBlock(4, 6, 13, true), marginTestBlock(5, 8, 7, true))
			container.Display = tc.display
			root := marginTestBlock(1, 0, 0, false,
				marginTestBlock(2, 0, 12, true), container, marginTestBlock(6, 9, 0, true))
			g, adapter := marginTestLayout(t, root)
			// Inside gap = max(13,8) = 13. Outside gaps use only the BFC
			// container's own margins: max(12,3) and max(4,9).
			assertFlowRootGeometry(t, g, map[NodeID][2]float32{
				1: {0, tc.nextY + 10}, 2: {0, 10}, 3: {22, tc.h},
				4: {tc.firstY, 10}, 5: {tc.lastY, 10}, 6: {tc.nextY, 10},
			})
			if len(adapter.calls) != 4 {
				t.Fatal("a block flow-root must not become an atomic paragraph item")
			}
		})
	}
}

func TestFlowRootDoesNotCollapseThrough(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		display                       Display
		before, top, bottom, after    float32
		nested                        bool
		containerY, containerH, nextY float32
	}{
		{"ordinary empty", blockDisplay, 4, 7, 11, 13, false, 17, 0, 23},
		{"empty flow-root", flowRootDisplay, 4, 7, 11, 13, false, 17, 0, 30},
		{"ordinary signed", blockDisplay, 13, -8, 10, -6, false, 15, 0, 15},
		{"flow-root signed", flowRootDisplay, 13, -8, 10, -6, false, 15, 0, 19},
		{"flow-root empty descendants", flowRootDisplay, 4, 7, 11, 13, true, 17, 5, 35},
	} {
		t.Run(tc.name, func(t *testing.T) {
			empty := marginTestBlock(3, tc.top, tc.bottom, false)
			empty.Display = tc.display
			if tc.nested {
				// This internal adjoining group is max(13,10)-max(8,3)=5.
				// It stays inside the flow-root even though both children collapse through.
				empty.ChildrenNodes = []*RenderNode{marginTestBlock(5, 13, -8, false,
					marginTestBlock(6, 10, -3, false))}
			}
			g, _ := marginTestLayout(t, marginTestBlock(1, 0, 0, false,
				marginTestBlock(2, 0, tc.before, true), empty, marginTestBlock(4, tc.after, 0, true)))
			assertFlowRootGeometry(t, g, map[NodeID][2]float32{
				1: {0, tc.nextY + 10}, 2: {0, 10}, 3: {tc.containerY, tc.containerH}, 4: {tc.nextY, 10},
			})
			if tc.nested {
				assertFlowRootGeometry(t, g, map[NodeID][2]float32{5: {10, 0}, 6: {0, 0}})
			}
		})
	}
}

func TestFlowRootOutsideMarginsCanCollapseWithParent(t *testing.T) {
	for _, outer := range []Display{blockDisplay, flowRootDisplay} {
		t.Run(outer.String(), func(t *testing.T) {
			inner := marginTestBlock(3, 8, 7, false, marginTestBlock(4, 20, 17, true))
			inner.Display = flowRootDisplay
			parent := marginTestBlock(2, 13, 5, false, inner)
			parent.Display = outer
			g, _ := marginTestLayout(t, marginTestBlock(1, 0, 0, false, parent))
			// An ordinary parent absorbs only inner's own 8/7px margins;
			// its descendants' 20/17px margins remain inside inner's 47px height.
			innerY, parentH, rootH := float32(0), float32(47), float32(67)
			if outer == flowRootDisplay {
				innerY, parentH, rootH = 8, 62, 80
			}
			assertFlowRootGeometry(t, g, map[NodeID][2]float32{
				1: {0, rootH}, 2: {13, parentH}, 3: {innerY, 47}, 4: {20, 10},
			})
		})
	}
}

func TestFlowRootSuppliedRootContainment(t *testing.T) {
	for _, display := range []Display{blockDisplay, flowRootDisplay} {
		root := marginTestBlock(1, 99, 99, false, marginTestBlock(2, 20, 7, true))
		root.Display = display
		g, _ := marginTestLayout(t, root)
		assertFlowRootGeometry(t, g, map[NodeID][2]float32{1: {0, 37}, 2: {20, 10}})
	}
}

func TestEmptyFlowRootStopsAncestorThroughCollapse(t *testing.T) {
	empty := marginTestBlock(3, 8, 7, false)
	empty.Display = flowRootDisplay
	parent := marginTestBlock(2, 13, 5, false, empty)
	g, _ := marginTestLayout(t, marginTestBlock(1, 0, 0, false, parent))
	// The parent's top group max(13,8) stays separate from its bottom
	// group max(5,7), even though neither parent nor child consumes height.
	assertFlowRootGeometry(t, g, map[NodeID][2]float32{1: {0, 20}, 2: {13, 0}, 3: {0, 0}})
}

func TestFlowRootSplitHoistAndAnonymousBoundaries(t *testing.T) {
	flowRoot := newRenderElement(4, flowRootDisplay,
		newRenderText(5, "inner"), newRenderElement(6, blockDisplay, newRenderText(7, "block")))
	span := newRenderElement(2, inlineDisplay, newRenderText(3, "before"), flowRoot, newRenderText(8, "after"))
	root, err := BuildLayoutTree(newRenderElement(1, blockDisplay, span), BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 3 || root.Children[0].Box != BoxAnonymousBlock ||
		root.Children[1].NodeID != 4 || root.Children[2].Box != BoxAnonymousBlock {
		t.Fatalf("block flow-root was not hoisted between inline fragments: %+v", root.Children)
	}
	box := root.Children[1]
	if !IsBlockLevel(box.Box) || !IsBlockContainer(box.Box) || box.Box != BoxBlock || box.FC != FCBlock ||
		len(box.Children) != 2 || box.Children[0].Box != BoxAnonymousBlock || box.Children[1].NodeID != 6 {
		t.Fatalf("incorrect flow-root normalization: %+v", box)
	}
	var sources []NodeID
	fragments := 0
	var walk func(*LayoutNode)
	walk = func(n *LayoutNode) {
		if n.EstablishesBFC != (n.NodeID == 4) {
			t.Fatalf("containment was lost or inherited by another box: %+v", n)
		}
		if n.NodeID == 2 {
			fragments++
		}
		if n.Box == BoxText {
			sources = append(sources, n.NodeID)
		}
		for _, child := range n.Children {
			walk(child)
		}
	}
	walk(root)
	if fragments != 2 || !reflect.DeepEqual(sources, []NodeID{3, 5, 7, 8}) {
		t.Fatalf("split lost source order or fragments: %v, %d", sources, fragments)
	}
	assertUniqueBoxIDs(t, root)
	flowRoot.Display = blockDisplay
	if !box.EstablishesBFC {
		t.Fatal("built containment aliases the source projection")
	}
}

func TestFlowRootUsesBlockWidths(t *testing.T) {
	for _, tc := range []struct {
		name     string
		width    Length
		wantW, x float32
	}{
		{"auto fills", lenAuto(), 100, 0},
		{"fixed centered", lenPx(40), 40, 30},
		{"percentage", lenPct(.5), 50, 25},
		{"em", lenEm(2), 20, 40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			child := newRenderElement(2, flowRootDisplay, newRenderText(3, "x"))
			child.Style = &ComputedStyle{Width: tc.width, FontSizePx: 10,
				Margin: EdgeLengths{Left: lenAuto(), Right: lenAuto()}}
			root, err := BuildLayoutTree(newRenderElement(1, blockDisplay, child), BuildOptions{})
			if err != nil {
				t.Fatal(err)
			}
			used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 100}})
			if err != nil {
				t.Fatal(err)
			}
			// Auto block flow-root fills its containing width without a shrink-to-fit
			// measurement service; percent/em widths aren't atomic-width errors.
			result, err := FlowLayout(root, used, newAtomicFake(t), nil, LayoutContext{}, LayoutOptions{})
			if err != nil {
				t.Fatal(err)
			}
			g := result.Geometry[root.Children[0].BoxID]
			if g.Content.W != tc.wantW || g.Frame.X != tc.x || g.Content.H != 10 || g.RelativeTo != root.BoxID {
				t.Fatalf("flow-root lost normal block width/placement: %+v", g)
			}
		})
	}
}

func TestFlowRootInsideFixedAndAutoInlineBlock(t *testing.T) {
	for _, width := range []Length{lenPx(80), lenAuto()} {
		container := marginTestBlock(3, 3, 4, false,
			marginTestBlock(4, 6, 13, true), marginTestBlock(5, 8, 7, true))
		container.Display = flowRootDisplay
		container.Style.Width = lenAuto()
		for _, child := range container.ChildrenNodes {
			child.Style.Width = lenAuto()
		}
		atomic := newRenderElement(2, inlineBlockDisplay, container)
		atomic.Style = &ComputedStyle{Width: width}
		root, err := BuildLayoutTree(newRenderElement(1, blockDisplay, atomic), BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		a := root.Children[0].Children[0]
		f := a.Children[0]
		first, last := f.Children[0], f.Children[1]
		original := copyTestTree(root)
		var previous *LayoutResult
		var saved LayoutGeometryTable
		for _, tc := range []struct{ available, autoW float32 }{{100, 90}, {50, 50}, {20, 30}} {
			used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: tc.available}})
			if err != nil {
				t.Fatal(err)
			}
			before := make(UsedValuesTable)
			for id, u := range used {
				before[id] = u
			}
			var measured []BoxID
			measure := measureFunc(func(req InlineMeasureRequest) (IntrinsicWidths, error) {
				measured = append(measured, req.OwnerID)
				return IntrinsicWidths{30, 90}, nil
			})
			adapter := newAtomicFake(t)
			result, err := FlowLayout(root, used, adapter, measure, LayoutContext{}, LayoutOptions{})
			if err != nil {
				t.Fatal(err)
			}
			wantW := float32(80)
			if width.Kind == LenAuto {
				wantW = tc.autoW
				if !reflect.DeepEqual(measured, []BoxID{first.BoxID, last.BoxID}) {
					t.Fatalf("incorrect intrinsic groups: %v", measured)
				}
			} else if len(measured) != 0 {
				t.Fatal("fixed atomic width caused intrinsic measurement")
			}
			g := result.Geometry
			if g[a.BoxID].Content.W != wantW || g[f.BoxID].Content.W != wantW ||
				g[a.BoxID].Content.H != 53 || g[f.BoxID].Content.H != 46 || g[f.BoxID].Frame.Y != 3 ||
				g[first.BoxID].Frame.Y != 6 || g[last.BoxID].Frame.Y != 29 || g[root.BoxID].Content.H != 53 {
				t.Fatalf("incorrect nested flow-root/atomic dimensions: %+v", g)
			}
			if g[f.BoxID].RelativeTo != a.BoxID || g[first.BoxID].RelativeTo != f.BoxID ||
				!reflect.DeepEqual(adapter.calls, []BoxID{first.BoxID, last.BoxID, root.BoxID}) {
				t.Fatal("incorrect coordinate owners or repeated final layout")
			}
			failure := errors.New("last paragraph failed")
			current := copyTestGeometry(result.Geometry)
			failed := newAtomicFake(t)
			failed.failOwner, failed.failure = last.BoxID, failure
			partial, err := FlowLayout(root, used, failed, measure, LayoutContext{}, LayoutOptions{})
			if partial != nil || !errors.Is(err, failure) {
				t.Fatalf("failed layout published a result: %v, %v", partial, err)
			}
			if !reflect.DeepEqual(root, original) || !reflect.DeepEqual(used, before) || !reflect.DeepEqual(result.Geometry, current) ||
				(previous != nil && !reflect.DeepEqual(previous.Geometry, saved)) {
				t.Fatal("layout mutated input or a previous result")
			}
			previous, saved = result, copyTestGeometry(result.Geometry)
		}
	}
}

func assertFlowRootGeometry(t *testing.T, g map[NodeID]LayoutGeometry, want map[NodeID][2]float32) {
	t.Helper()
	for id, yh := range want {
		got, ok := g[id]
		if !ok || got.Frame.Y != yh[0] || got.Content.H != yh[1] {
			t.Errorf("node %d: geometry=%+v, want border Y=%v, content H=%v", id, got, yh[0], yh[1])
		}
	}
}
