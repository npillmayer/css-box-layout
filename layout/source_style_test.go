package layout

import (
	"math"
	"reflect"
	"testing"

	"github.com/npillmayer/css-box-layout/text"
)

func TestBuildResolve_DocumentStylesAndSources(t *testing.T) {
	before, office, dot := newRenderText(2, "Before "), newRenderText(4, "office"), newRenderText(5, ".")
	inside, after := newRenderText(7, "Inside"), newRenderText(8, "After")
	refs := map[NodeID]text.TextRef{}
	for i, n := range []*RenderNode{before, office, dot, inside, after} {
		// Several nodes can share a source, with distinct absolute byte ranges.
		ref := text.TextRef{Source: 99, Range: text.TextRange{Start: uint64(10*i + 100), End: uint64(10*i + 100 + len(n.HTML.Data))}}
		n.Text = &ref
		refs[n.ID] = ref
	}
	span := newRenderElement(3, "inline", office)
	note := newRenderElement(6, "block", inside)
	note.Style = &ComputedStyle{Width: lenPx(120)}
	input := newRenderElement(1, "block", before, span, dot, note, after)
	input.Style = &ComputedStyle{Width: lenPx(240), FontSizePx: 20, Padding: edges(lenPx(5)), Border: edges(lenPx(1)), Margin: edges(lenPx(3))}
	root, err := BuildLayoutTree(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 800}, FontSizePx: 16})
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 3 {
		t.Fatalf("got %d groups", len(root.Children))
	}
	for i, want := range []float32{240, 120, 240} {
		owner := root.Children[i]
		if got := used[owner.BoxID].ContentWidth; got != want {
			t.Errorf("group %d width=%v want %v", i, got, want)
		}
		if len(owner.Children) != 1 || owner.Children[0].Box != BoxAnonymousInline {
			t.Fatal("missing inline group")
		}
		if owner.Style == nil || owner.Style.FontSizePx != 20 {
			t.Fatal("lost inherited font size")
		}
		if i != 1 && (used[owner.BoxID].Padding != (Edges{}) || used[owner.BoxID].Border != (Edges{}) || used[owner.BoxID].Margin != (Edges{})) {
			t.Fatal("anonymous owner copied non-inherited edges")
		}
	}
	if used[root.BoxID].Padding.Left != 5 || used[root.BoxID].Border.Left != 1 || used[root.BoxID].Margin.Left != 3 {
		t.Fatal("root edges lost")
	}
	seen := map[NodeID]text.TextRef{}
	var walk func(*LayoutNode)
	walk = func(n *LayoutNode) {
		if n.Box == BoxText {
			seen[n.NodeID] = n.Text
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	if !reflect.DeepEqual(seen, refs) {
		t.Fatalf("text references changed: %v", seen)
	}
	// Mutating adapter inputs after construction cannot alter the box tree.
	input.Style.Width = lenPx(1)
	before.Text.Range.End = 999
	if root.Style.Width.Value != 240 || seen[2] != refs[2] {
		t.Fatal("input values were not snapshotted")
	}
	walk(root)
	if seen[2] != refs[2] {
		t.Fatal("text reference aliases input")
	}
}

func TestBuildResolve_SplitStyleAndFontInheritance(t *testing.T) {
	block := newRenderElement(4, "block", newRenderText(5, "inside"))
	block.Style = &ComputedStyle{Width: lenEm(2), Padding: EdgeLengths{Left: lenPct(.1)}}
	span := newRenderElement(2, "inline", newRenderText(3, "before"), block, newRenderText(6, "after"))
	span.Style = &ComputedStyle{Width: lenAuto(), FontSizePx: 24, Padding: EdgeLengths{Left: lenEm(1)}}
	input := newRenderElement(1, "block", span)
	input.Style = &ComputedStyle{Width: lenPx(200), FontSizePx: 16}
	root, err := BuildLayoutTree(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 800}, FontSizePx: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Children) != 3 {
		t.Fatal("expected split inline / block / inline")
	}
	for _, idx := range []int{0, 2} {
		fragment := root.Children[idx].Children[0].Children[0]
		if fragment.NodeID != 2 || fragment.Style == nil || *fragment.Style != *span.Style || used[fragment.BoxID].Padding.Left != 24 {
			t.Fatalf("fragment lost style or local em basis: %+v", fragment)
		}
	}
	hoisted := root.Children[1]
	if used[hoisted.BoxID].ContentWidth != 48 || used[hoisted.BoxID].Padding.Left != 20 {
		t.Fatalf("hoisting lost source inheritance: %+v", used[hoisted.BoxID])
	}
	span.Style.FontSizePx = 99
	if hoisted.Style.FontSizePx != 24 {
		t.Fatal("hoisted style aliases input")
	}
}

func TestBuildLayoutTree_TextReferenceContract(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		ref        *text.TextRef
		bad        bool
		count      int
	}{
		{name: "missing", data: "text", bad: true},
		{name: "reversed", ref: &text.TextRef{Range: text.TextRange{Start: 4, End: 2}}, bad: true},
		{name: "empty_missing", count: 0},
		{name: "empty_authoritative", data: "ignored", ref: &text.TextRef{Source: 7, Range: text.TextRange{Start: 9, End: 9}}, count: 0},
		{name: "source_backed", ref: &text.TextRef{Source: 0, Range: text.TextRange{Start: 100, End: 110}}, count: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := newRenderText(2, tc.data)
			n.Text = tc.ref
			root, err := BuildLayoutTree(newRenderElement(1, "block", n), BuildOptions{})
			if tc.bad {
				if err == nil || root != nil {
					t.Fatal("expected error without partial tree")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got := root.Children[0].Children
			if len(got) != tc.count {
				t.Fatalf("got %d text boxes", len(got))
			}
			if tc.count > 0 && got[0].Text != *tc.ref {
				t.Fatal("range not preserved")
			}
		})
	}
}

func TestBuildLayoutTree_InvalidTypedStyles(t *testing.T) {
	for _, style := range []ComputedStyle{
		{Width: Length{Kind: LengthKind(99)}},
		{Width: lenPx(float32(math.NaN()))},
		{Width: lenPx(-1)},
		{Width: lenAuto(), FontSizePx: -1},
		{Width: lenAuto(), Padding: edges(lenAuto())},
	} {
		input := newRenderElement(1, "block")
		input.Style = &style
		if tree, err := BuildLayoutTree(input, BuildOptions{}); err == nil || tree != nil {
			t.Fatalf("accepted invalid style %+v", style)
		}
	}
}

func TestBuildLayoutTree_StyleDefaultsAndInlineBlock(t *testing.T) {
	auto := newRenderElement(2, "block")
	zero := newRenderElement(3, "block")
	zero.Style = &ComputedStyle{}
	atomic := newRenderElement(4, "inline-block")
	style := DefaultComputedStyle()
	style.Width = lenPx(80)
	style.Border = edges(lenPx(2))
	atomic.Style = &style
	root, err := BuildLayoutTree(newRenderElement(1, "block", auto, zero, atomic), BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 200}, FontSizePx: 16})
	if err != nil {
		t.Fatal(err)
	}
	if used[root.Children[0].BoxID].ContentWidth != 200 || used[root.Children[1].BoxID].ContentWidth != 0 {
		t.Fatal("nil style and explicit zero width were conflated")
	}
	box := root.Children[2].Children[0].Children[0]
	if box.Box != BoxInlineBlock || box.Style == nil || *box.Style != style || used[box.BoxID].Border.Left != 2 {
		t.Fatal("inline-block style lost")
	}
}

func TestBuildLayoutTree_RejectsStringBoxValues(t *testing.T) {
	root := newRenderElement(1, "block")
	root.Styles["width"] = "240px"
	if tree, err := BuildLayoutTree(root, BuildOptions{}); err == nil || tree != nil {
		t.Fatal("unconverted box values must not be silently ignored")
	}
}
