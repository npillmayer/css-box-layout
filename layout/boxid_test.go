package layout

import (
	"reflect"
	"testing"
)

func TestBuildLayoutTree_BoxIDCollision(t *testing.T) {
	// With the original arithmetic allocator, the first child's empty wrapper
	// and the second child both receive 4611686018427387905.
	input := newRenderElement(5207740746832417946, "block",
		newRenderElement(2, "block"), newRenderElement(3, "block"))
	root, err := BuildLayoutTree(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assertUniqueBoxIDs(t, root)
}

func assertUniqueBoxIDs(t *testing.T, root *LayoutNode) {
	t.Helper()
	seen := make(map[BoxID]bool)
	var walk func(*LayoutNode)
	walk = func(n *LayoutNode) {
		if n.BoxID == 0 {
			t.Error("generated zero BoxID")
		}
		if seen[n.BoxID] {
			t.Errorf("duplicate BoxID %d", n.BoxID)
		}
		seen[n.BoxID] = true
		for _, child := range n.Children {
			walk(child)
		}
	}
	walk(root)
}

func TestBoxIDGen_ChildCounterBoundary(t *testing.T) {
	gen := newBoxIDGen()
	root := gen.newRoot(1)
	first := gen.newChild(root)
	gen.next[root] = 1<<32 - 1
	last := gen.newChild(root)
	after := gen.newChild(root)
	if first == last || first == after || last == after {
		t.Fatalf("duplicate IDs across counter boundary: %d, %d, %d", first, last, after)
	}
}

func TestBuildLayoutTree_BoxIDStability(t *testing.T) {
	for _, rootID := range []NodeID{1, 5207740746832417946, ^NodeID(0)} {
		makeInput := func(withEmpty bool) *RenderNode {
			children := []*RenderNode{
				newRenderElement(10, "inline", newRenderText(11, "before"),
					newRenderElement(12, "block", newRenderText(13, "inside")), newRenderText(14, "after")),
				newRenderElement(20, "inline-block", newRenderElement(21, "block")),
				newRenderElement(30, "block"),
			}
			if withEmpty {
				children = append([]*RenderNode{newRenderText(40, "")}, children...)
				children = append(children, newRenderText(41, ""))
				children[1].ChildrenNodes = append([]*RenderNode{newRenderText(42, "")}, children[1].ChildrenNodes...)
			}
			return newRenderElement(rootID, "block", children...)
		}
		build := func(empty bool) *LayoutNode {
			t.Helper()
			root, err := BuildLayoutTree(makeInput(empty), BuildOptions{})
			if err != nil {
				t.Fatal(err)
			}
			assertUniqueBoxIDs(t, root)
			return root
		}
		first, repeat, withEmpty := build(false), build(false), build(true)
		if !reflect.DeepEqual(first, repeat) || !reflect.DeepEqual(first, withEmpty) {
			t.Fatalf("root %d: rebuild or omitted empty text changed box identities/structure", rootID)
		}
		fragments := []BoxID{}
		var collect func(*LayoutNode)
		collect = func(n *LayoutNode) {
			if n.NodeID == 10 {
				fragments = append(fragments, n.BoxID)
			}
			for _, c := range n.Children {
				collect(c)
			}
		}
		collect(first)
		if len(fragments) != 2 || fragments[0] == fragments[1] {
			t.Fatalf("split fragments need distinct identities: %v", fragments)
		}
	}
}

func TestBoxIDGen_Wraparound(t *testing.T) {
	gen := newBoxIDGen()
	root := gen.newRoot(0)
	first := gen.newChild(root)
	gen.next[root] = ^uint64(0)
	before, after := gen.newChild(root), gen.newChild(root)
	if before == 0 || after == 0 || before == after || first == before || first == after {
		t.Fatalf("invalid IDs across uint64 rollover: %d, %d, %d", first, before, after)
	}
	// Exercise probing across the end of the ID space, skipping occupied 1.
	max := gen.newRoot(^NodeID(0) - 1)
	wrapped := gen.newRoot(^NodeID(0) - 1)
	if max != ^BoxID(0) || wrapped != 2 {
		t.Fatalf("probe wrap: got %d, %d; want max and 2", max, wrapped)
	}
}

func TestBuildLayoutTree_BoxIDsSurvivePasses(t *testing.T) {
	root, err := BuildLayoutTree(newRenderElement(1, "block",
		newRenderText(2, "before"), newRenderElement(3, "block"), newRenderText(4, "after")), BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ids := func() []BoxID {
		var result []BoxID
		var walk func(*LayoutNode)
		walk = func(n *LayoutNode) {
			result = append(result, n.BoxID)
			for _, c := range n.Children {
				walk(c)
			}
		}
		walk(root)
		return result
	}
	before := ids()
	used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 200}})
	if err != nil {
		t.Fatal(err)
	}
	if len(used) != len(before) {
		t.Fatal("used-value entries lost box identities")
	}
	result, err := FlowLayout(root, used, fakeInlineLayouter{}, nil, LayoutContext{}, LayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, ids()) {
		t.Fatal("passes changed box identities")
	}
	for id := range result.Geometry {
		if _, ok := used[id]; !ok {
			t.Fatalf("geometry has unknown BoxID %d", id)
		}
	}
}
