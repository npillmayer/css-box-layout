package layout

import (
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
)

func fixedElement(id NodeID, display Display, children ...*RenderNode) *RenderNode {
	n := newRenderElement(id, display, children...)
	s := DefaultComputedStyle()
	s.Position = PositionFixed
	n.Style = &s
	return n
}

func walkPositionTree(n *LayoutNode, visit func(*LayoutNode)) {
	if n == nil {
		return
	}
	visit(n)
	for _, child := range n.Children {
		walkPositionTree(child, visit)
	}
}

func TestBuildLayoutTreesFixedBranches(t *testing.T) {
	inner := fixedElement(7, inlineBlockDisplay, newRenderText(8, "nested"))
	outer := fixedElement(4, inlineDisplay, newRenderText(5, "before"),
		newRenderElement(6, blockDisplay, inner), newRenderText(9, "after"))
	outer.Style.WhiteSpace = WhiteSpacePre
	constraints := DefaultFixedStyle()
	constraints.Insets.Left = lenPx(-5)
	constraints.Insets.Bottom = lenPct(.1)
	constraints.Height = lenEm(2)
	outer.Style.Fixed = &constraints
	span := newRenderElement(2, inlineDisplay, newRenderText(3, "a"), outer, newRenderText(10, "b"))
	root := newRenderElement(1, blockDisplay, span, fixedElement(11, blockDisplay))
	rootStyle := DefaultComputedStyle()
	rootStyle.FontSizePx = 24
	root.Style = &rootStyle
	trees, err := BuildLayoutTrees(root, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(trees.Fixed) != 3 {
		t.Fatalf("fixed roots = %d, want 3", len(trees.Fixed))
	}
	seen := map[BoxID]bool{}
	for i, n := range append([]*LayoutNode{trees.Flow}, trees.Fixed...) {
		walkPositionTree(n, func(box *LayoutNode) {
			if box.BoxID == 0 || seen[box.BoxID] {
				t.Fatalf("nonunique/zero BoxID %d", box.BoxID)
			}
			seen[box.BoxID] = true
			if box != n && box.Style.Position != PositionStatic {
				t.Fatal("nested fixed box was not diverted")
			}
			if i == 0 && box.Style.Position != PositionStatic {
				t.Fatal("fixed participant leaked into flow")
			}
			if box.Box == BoxAnonymousBlock || box.Box == BoxAnonymousInline {
				if box.Style.Fixed != nil || box.Style.Width.Kind != LenAuto {
					t.Fatal("anonymous box inherited fixed constraints")
				}
			}
		})
	}
	for i, id := range []NodeID{4, 7, 11} {
		n := trees.Fixed[i]
		if n.NodeID != id || n.Box != BoxBlock || n.FC != FCBlock || !n.EstablishesBFC || n.Style.Position != PositionFixed {
			t.Fatalf("fixed root %d = %+v", i, n)
		}
		if n.Style.FontSizePx != 24 {
			t.Fatal("source font inheritance lost during diversion")
		}
	}
	fixed := trees.Fixed[0]
	if len(fixed.Children) != 3 || fixed.Children[0].Box != BoxAnonymousBlock || fixed.Children[2].Box != BoxAnonymousBlock {
		t.Fatal("fixed interior was not independently normalized")
	}
	if fixed.Children[0].Style.WhiteSpace != WhiteSpacePre || fixed.Children[0].Children[0].Children[0].Style.WhiteSpace != WhiteSpacePre {
		t.Fatal("fixed interior lost inherited white-space")
	}
	if len(trees.Flow.Children) != 1 || len(trees.Flow.Children[0].Children) != 1 || len(trees.Flow.Children[0].Children[0].Children) != 2 {
		t.Fatal("fixed inline descendant split the surrounding paragraph/span")
	}
	if fixed.Style.Fixed == outer.Style.Fixed || *fixed.Style.Fixed != constraints {
		t.Fatal("fixed constraints were not copied")
	}
	if outer.Display != inlineDisplay || inner.Display != inlineBlockDisplay || len(span.ChildrenNodes) != 3 {
		t.Fatal("builder mutated render structure/display")
	}
	outer.Style.Fixed.Height = lenPx(999)
	outer.Style.FontSizePx = 99
	if fixed.Style.Fixed.Height != lenEm(2) || fixed.Style.FontSizePx != 24 {
		t.Fatal("built style aliases adapter input")
	}
}

func TestBuildLayoutTreesRootAndDefaults(t *testing.T) {
	for _, display := range []Display{blockDisplay, flowRootDisplay, inlineDisplay, inlineBlockDisplay} {
		for _, fixedStyle := range []*FixedStyle{nil, {}, func() *FixedStyle { s := DefaultFixedStyle(); return &s }()} {
			input := fixedElement(1, display)
			input.Style.Fixed = fixedStyle
			trees, err := BuildLayoutTrees(input, BuildOptions{})
			if err != nil || trees.Flow != nil || len(trees.Fixed) != 1 {
				t.Fatalf("fixed-only build: %+v, %v", trees, err)
			}
			if !reflect.DeepEqual(trees.Fixed[0].Style.Fixed, fixedStyle) {
				t.Fatal("auto/explicit zero constraints changed")
			}
			if _, err := BuildLayoutTree(input, BuildOptions{}); !errors.Is(err, ErrFixedLayoutRequired) {
				t.Fatalf("legacy builder discarded fixed root: %v", err)
			}
		}
	}
	d := DefaultFixedStyle()
	for _, l := range []Length{d.Insets.Top, d.Insets.Right, d.Insets.Bottom, d.Insets.Left, d.Height} {
		if l.Kind != LenAuto {
			t.Fatal("fixed default must be auto")
		}
	}
	for _, style := range []*ComputedStyle{nil, {}, func() *ComputedStyle { s := DefaultComputedStyle(); return &s }()} {
		r := newRenderElement(1, blockDisplay)
		r.Style = style
		trees, err := BuildLayoutTrees(r, BuildOptions{})
		if err != nil || trees.Flow == nil || len(trees.Fixed) != 0 || trees.Flow.Style.Position != PositionStatic || trees.Flow.Style.Fixed != nil {
			t.Fatalf("static defaults changed: %+v, %v", trees, err)
		}
		legacy, err := BuildLayoutTree(r, BuildOptions{})
		if err != nil || !reflect.DeepEqual(legacy, trees.Flow) {
			t.Fatal("legacy and new builds differ for static input")
		}
	}
	for _, r := range []*RenderNode{nil, fixedElement(1, noneDisplay, &RenderNode{})} {
		trees, err := BuildLayoutTrees(r, BuildOptions{})
		if err != nil || trees == nil || trees.Flow != nil || len(trees.Fixed) != 0 {
			t.Fatalf("empty/suppressed input: %+v, %v", trees, err)
		}
	}
	if trees, err := BuildLayoutTrees(newRenderElement(1, inlineDisplay), BuildOptions{}); trees != nil || !errors.Is(err, ErrInvalidRoot) {
		t.Fatal("static inline root unexpectedly allowed")
	}
}

func TestPositionValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*RenderNode)
		want error
	}{
		{"unknown position", func(n *RenderNode) { n.Style.Position = PositionKind(99) }, ErrUnsupportedPosition},
		{"fixed constraints on static", func(n *RenderNode) { n.Style.Position = PositionStatic }, ErrInvalidPosition},
		{"invalid inset kind", func(n *RenderNode) { n.Style.Fixed.Insets.Top.Kind = LengthKind(99) }, ErrInvalidPosition},
		{"nan inset", func(n *RenderNode) { n.Style.Fixed.Insets.Right.Value = float32(math.NaN()) }, ErrInvalidPosition},
		{"infinite height", func(n *RenderNode) { n.Style.Fixed.Height.Value = float32(math.Inf(1)) }, ErrInvalidPosition},
		{"invalid height kind", func(n *RenderNode) { n.Style.Fixed.Height.Kind = LengthKind(99) }, ErrInvalidPosition},
		{"negative height", func(n *RenderNode) { n.Style.Fixed.Height = lenPx(-1) }, ErrInvalidPosition},
		{"replaced fixed", func(n *RenderNode) { n.Replaced = &ReplacedContent{Height: 10} }, ErrUnsupportedPosition},
		{"positioned text", func(n *RenderNode) {
			text := newRenderText(n.ID, "x")
			n.HTML, n.Text, n.Display = text.HTML, text.Text, Display{}
		}, ErrInvalidPosition},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := fixedElement(3, inlineDisplay)
			f := DefaultFixedStyle()
			n.Style.Fixed = &f
			tc.edit(n)
			root := newRenderElement(1, blockDisplay, fixedElement(2, blockDisplay), n)
			if trees, err := BuildLayoutTrees(root, BuildOptions{}); trees != nil || !errors.Is(err, tc.want) {
				t.Fatalf("got (%+v, %v), want nil, %v", trees, err, tc.want)
			}
		})
	}
	for _, prop := range []string{"position", "top", "right", "bottom", "left", "inset", "inset-block", "inset-inline", "inset-block-start", "inset-block-end", "inset-inline-start", "inset-inline-end", "height"} {
		n := fixedElement(2, blockDisplay)
		n.Styles[prop] = "0"
		if trees, err := BuildLayoutTrees(newRenderElement(1, blockDisplay, n), BuildOptions{}); err == nil || trees != nil {
			t.Fatalf("legacy %s silently accepted", prop)
		}
	}
	// Suppression precedes style/subtree validation, including positioning.
	hidden := fixedElement(2, noneDisplay, fixedElement(3, blockDisplay))
	hidden.Style.Position = PositionKind(99)
	trees, err := BuildLayoutTrees(newRenderElement(1, blockDisplay, hidden), BuildOptions{})
	if err != nil || len(trees.Fixed) != 0 {
		t.Fatalf("suppressed positioned subtree: %+v, %v", trees, err)
	}
}

func TestFixedCannotEnterNormalFlowAPIs(t *testing.T) {
	input := newRenderElement(1, blockDisplay, fixedElement(2, blockDisplay))
	if tree, err := BuildLayoutTree(input, BuildOptions{}); tree != nil || !errors.Is(err, ErrFixedLayoutRequired) {
		t.Fatalf("legacy builder: %+v, %v", tree, err)
	}
	for _, nested := range []bool{false, true} {
		s := DefaultComputedStyle()
		s.Position = PositionFixed
		n := &LayoutNode{BoxID: 3, Box: BoxBlock, Style: &s}
		if nested {
			n = &LayoutNode{BoxID: 1, Box: BoxBlock, Children: []*LayoutNode{{BoxID: 2, Box: BoxAnonymousInline, Children: []*LayoutNode{n}}}}
		}
		if used, err := ResolveUsedValues(n, ResolveContext{}); used != nil || !errors.Is(err, ErrFixedLayoutRequired) {
			t.Fatalf("normal resolver accepted fixed input: %v, %v", used, err)
		}
		calls := 0
		inline := resultFunc(func(InlineRequest) (InlineResult, error) { calls++; return InlineResult{}, nil })
		measure := measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) { calls++; return IntrinsicWidths{}, nil })
		if result, err := FlowLayout(n, nil, inline, measure, LayoutContext{}, LayoutOptions{}); result != nil || !errors.Is(err, ErrFixedLayoutRequired) || calls != 0 {
			t.Fatalf("normal layout accepted fixed input: %+v, %v, calls %d", result, err, calls)
		}
	}
}

func TestFixedBuildConcurrentReplay(t *testing.T) {
	input := newRenderElement(5207740746832417946, blockDisplay,
		fixedElement(2, inlineDisplay, fixedElement(3, blockDisplay)), newRenderElement(4, blockDisplay))
	want, err := BuildLayoutTrees(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := BuildLayoutTrees(input, BuildOptions{})
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("concurrent build changed identities/structure: %v", err)
			}
		}()
	}
	wg.Wait()
}
