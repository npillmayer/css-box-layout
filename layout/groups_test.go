package layout

import (
	"reflect"
	"slices"
	"sync"
	"testing"
)

func buildGroupFixture(t *testing.T, input *RenderNode) *LayoutNode {
	t.Helper()
	root, err := BuildLayoutTree(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func nestedGroupFixture(t *testing.T) *LayoutNode {
	t.Helper()
	leaf := newRenderElement(7, inlineDisplay)
	leaf.Style = &ComputedStyle{Width: lenPx(10)}
	leaf.Replaced = &ReplacedContent{Height: 20}
	root := newRenderElement(1, blockDisplay,
		newRenderText(2, "before "),
		newRenderElement(3, inlineDisplay,
			newRenderElement(4, inlineBlockDisplay,
				newRenderElement(5, blockDisplay,
					newRenderElement(6, inlineBlockDisplay))),
			leaf),
		newRenderElement(8, inlineBlockDisplay))
	return buildGroupFixture(t, root)
}

func TestInlineGroupsNestedAtomics(t *testing.T) {
	root := nestedGroupFixture(t)
	before := copyTestTree(root)
	groups := slices.Collect(InlineGroups(root))
	if len(groups) != 4 {
		t.Fatalf("groups = %d, want 4", len(groups))
	}
	for i, want := range []struct {
		owner, parent NodeID
		atoms         []NodeID
	}{
		{1, 0, []NodeID{4, 7, 8}},
		{5, 4, []NodeID{6}},
		{6, 6, nil},
		{8, 8, nil},
	} {
		g := groups[i]
		if g.Group.Box != BoxAnonymousInline || g.Group.NodeID != 0 || g.Owner.NodeID != want.owner || !IsBlockContainer(g.Owner.Box) {
			t.Fatalf("group %d owner: %+v", i, g)
		}
		if want.parent == 0 {
			if g.ParentAtomic != nil {
				t.Fatal("unexpected parent atomic")
			}
		} else if g.ParentAtomic == nil || g.ParentAtomic.NodeID != want.parent {
			t.Fatalf("group %d parent atomic", i)
		}
		var ids []NodeID
		for _, atom := range g.Atomics {
			ids = append(ids, atom.NodeID)
		}
		if !slices.Equal(ids, want.atoms) {
			t.Fatalf("group %d atoms: %v", i, ids)
		}
	}
	if groups[0].Owner != root || groups[0].Atomics[1].Replaced == nil {
		t.Fatal("descriptors must borrow the actual nodes, including replaced leaves")
	}
	if groups[1].ParentAtomic != groups[0].Atomics[0] || groups[2].Owner != groups[1].Atomics[0] || groups[3].Owner != groups[0].Atomics[2] {
		t.Fatal("lost node identity")
	}
	if len(groups[2].Group.Children) != 0 || len(groups[3].Group.Children) != 0 {
		t.Fatal("empty interiors lost")
	}
	if !reflect.DeepEqual(before, root) {
		t.Fatal("enumeration mutated tree/style/text")
	}

	// A block subtree cannot infer ancestors outside its input boundary.
	subtree := slices.Collect(InlineGroups(groups[1].Owner))
	if len(subtree) != 2 || subtree[0].ParentAtomic != nil || subtree[1].ParentAtomic != groups[2].Owner {
		t.Fatal("subtree boundary")
	}
	atomicRoot := slices.Collect(InlineGroups(groups[0].Atomics[0]))
	if len(atomicRoot) != 2 || atomicRoot[0].ParentAtomic != groups[0].Atomics[0] {
		t.Fatal("atomic root must enclose its interior")
	}
}

func TestInlineGroupsSplitAnonymousOwners(t *testing.T) {
	root := buildGroupFixture(t, newRenderElement(1, blockDisplay,
		newRenderElement(2, inlineDisplay,
			newRenderText(3, "before"),
			newRenderElement(4, blockDisplay, newRenderText(5, "block")),
			newRenderText(6, "after")),
		newRenderElement(7, flowRootDisplay)))
	groups := slices.Collect(InlineGroups(root))
	if len(groups) != 4 {
		t.Fatalf("groups = %d", len(groups))
	}
	for _, i := range []int{0, 2} {
		g := groups[i]
		if g.Owner.Box != BoxAnonymousBlock || g.Owner.NodeID != 0 || g.Owner.Children[0] != g.Group || g.Group.Children[0].NodeID != 2 {
			t.Fatalf("split group %d did not use actual anonymous owner", i)
		}
	}
	if groups[0].Owner == groups[2].Owner || groups[0].Group == groups[2].Group || groups[1].Owner.NodeID != 4 || groups[3].Owner.NodeID != 7 || !groups[3].Owner.EstablishesBFC {
		t.Fatal("source IDs or formatting contexts replaced structural ownership")
	}
	if len(groups[3].Group.Children) != 0 {
		t.Fatal("empty flow-root group missing")
	}
}

func TestInlineGroupsEmptyAndSuppressed(t *testing.T) {
	for _, input := range []*RenderNode{nil, newRenderElement(1, noneDisplay)} {
		if got := slices.Collect(InlineGroups(buildGroupFixture(t, input))); len(got) != 0 {
			t.Fatal("nil tree has groups")
		}
	}
	for _, input := range []*RenderNode{
		newRenderElement(1, blockDisplay),
		newRenderElement(1, blockDisplay, newRenderText(2, " \n\t")),
		newRenderElement(1, blockDisplay, newRenderElement(2, inlineDisplay)),
	} {
		root := buildGroupFixture(t, input)
		groups := slices.Collect(InlineGroups(root))
		if len(groups) != 1 || groups[0].Owner != root || len(groups[0].Atomics) != 0 {
			t.Fatal("empty/whitespace/inline-only owner")
		}
	}
	// Construction omits collapsible-only runs between blocks. Enumeration must
	// not reconstruct them as paragraphs, or count ordinary inline elements.
	root := buildGroupFixture(t, newRenderElement(1, blockDisplay,
		newRenderText(2, " \n"), newRenderElement(3, blockDisplay), newRenderText(4, " \n"),
		newRenderElement(5, blockDisplay), newRenderText(6, " \n")))
	groups := slices.Collect(InlineGroups(root))
	if len(groups) != 2 || groups[0].Owner.NodeID != 3 || groups[1].Owner.NodeID != 5 {
		t.Fatal("invented suppressed groups")
	}
}

func TestInlineGroupsIteratorLifetime(t *testing.T) {
	root := nestedGroupFixture(t)
	before := copyTestTree(root)
	seq := InlineGroups(root)
	want := slices.Collect(seq)
	seen := 0
	seq(func(g InlineGroup) bool { seen++; return false })
	if seen != 1 {
		t.Fatal("iterator ignored yield=false")
	}
	for g := range seq {
		if g.Group != want[0].Group {
			t.Fatal("iterator did not restart")
		}
		break
	}
	// Mutating the returned slice while suspended must not redirect later walks.
	var got []*LayoutNode
	for g := range seq {
		got = append(got, g.Group)
		if len(g.Atomics) > 0 {
			g.Atomics[0] = nil
		}
	}
	if len(got) != len(want) {
		t.Fatal("caller slice mutation affected traversal")
	}
	for i := range got {
		if got[i] != want[i].Group {
			t.Fatal("changed traversal after descriptor mutation")
		}
	}
	if !reflect.DeepEqual(slices.Collect(seq), want) || !reflect.DeepEqual(root, before) {
		t.Fatal("iterator retained mutable descriptor state")
	}
	// Retained descriptors from different runs have independent slice storage.
	again := slices.Collect(seq)
	again[0].Atomics[0] = nil
	if want[0].Atomics[0] == nil {
		t.Fatal("enumerations share descriptor slices")
	}
}

func TestInlineGroupsIndependentTreesAndConcurrentReads(t *testing.T) {
	a, b := nestedGroupFixture(t), nestedGroupFixture(t)
	first, second := slices.Collect(InlineGroups(a)), slices.Collect(InlineGroups(b))
	for i := range first {
		if first[i].Group.BoxID != second[i].Group.BoxID || first[i].Group == second[i].Group || first[i].Owner == second[i].Owner {
			t.Fatal("independent build identity")
		}
	}
	seq := InlineGroups(a)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 20 {
				got := slices.Collect(seq)
				if !reflect.DeepEqual(got, first) {
					t.Error("concurrent enumeration changed")
				}
				got[0].Atomics[0] = nil
			}
		})
	}
	wg.Wait()
}
