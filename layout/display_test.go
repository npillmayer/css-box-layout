package layout

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestDisplayValidate(t *testing.T) {
	for _, tc := range []struct {
		display Display
		name    string
	}{
		{blockDisplay, "block flow"},
		{inlineDisplay, "inline flow"},
		{inlineBlockDisplay, "inline flow-root"},
		{Display{Kind: DisplayBox, Outer: OuterBlock, Inner: InnerFlowRoot}, "block flow-root"},
		{noneDisplay, "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.display.Validate(); err != nil {
				t.Fatal(err)
			}
			if got := tc.display.String(); got != tc.name {
				t.Fatalf("String() = %q, want %q", got, tc.name)
			}
		})
	}
}

func TestInvalidDisplayValues(t *testing.T) {
	for _, d := range []Display{
		{},
		{Kind: DisplayKind(99)},
		{Outer: OuterBlock, Inner: InnerFlow},
		{Kind: DisplayBox},
		{Kind: DisplayBox, Outer: OuterBlock},
		{Kind: DisplayBox, Inner: InnerFlow},
		{Kind: DisplayBox, Outer: DisplayOuter(99), Inner: InnerFlow},
		{Kind: DisplayBox, Outer: OuterBlock, Inner: DisplayInner(99)},
		{Kind: DisplayNone, Outer: OuterBlock},
		{Kind: DisplayNone, Inner: InnerFlow},
		{Kind: DisplayNone, Outer: OuterInline, Inner: InnerFlowRoot},
	} {
		t.Run(d.String(), func(t *testing.T) {
			if err := d.Validate(); !errors.Is(err, ErrInvalidDisplay) {
				t.Fatalf("Validate() = %v, want ErrInvalidDisplay", err)
			}
			// The public builder validates both root and descendant input.
			for _, isRoot := range []bool{true, false} {
				input := newRenderElement(42, d)
				if !isRoot {
					input = newRenderElement(1, blockDisplay, input)
				}
				assertBuildDisplayError(t, input, ErrInvalidDisplay, 42)
			}
		})
	}
}

func TestBuildLayoutTreeTypedDisplay(t *testing.T) {
	for _, tc := range []struct {
		display Display
		box     BoxKind
		fc      FormattingContextKind
	}{
		{blockDisplay, BoxBlock, FCBlock},
		{flowRootDisplay, BoxBlock, FCBlock},
		{inlineDisplay, BoxInline, FCInline},
		{inlineBlockDisplay, BoxInlineBlock, FCBlock},
	} {
		t.Run(tc.display.String(), func(t *testing.T) {
			child := newRenderElement(2, tc.display, newRenderText(3, "contents"))
			input := newRenderElement(1, blockDisplay, child)
			root, err := BuildLayoutTree(input, BuildOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if root.Box != BoxBlock || root.FC != FCBlock || root.NodeID != 1 || len(root.Children) != 1 {
				t.Fatalf("unexpected root: %+v", root)
			}
			node := root.Children[0]
			if tc.display.Outer == OuterInline {
				if node.Box != BoxAnonymousInline || len(node.Children) != 1 {
					t.Fatalf("inline child lost its surrounding inline group: %+v", node)
				}
				node = node.Children[0]
			}
			if node.Box != tc.box || node.FC != tc.fc || node.NodeID != 2 {
				t.Fatalf("unexpected child: %+v", node)
			}
			if node.EstablishesBFC != (tc.display.Inner == InnerFlowRoot) || root.EstablishesBFC {
				t.Fatalf("incorrect formatting-context boundary: root=%+v, child=%+v", root, node)
			}
			text := node.Children[0]
			if tc.box != BoxInline {
				if text.Box != BoxAnonymousInline || len(text.Children) != 1 {
					t.Fatalf("block container lost its inline contents: %+v", text)
				}
				text = text.Children[0]
			}
			if text.Box != BoxText || text.NodeID != 3 || text.Text != *child.ChildrenNodes[0].Text {
				t.Fatalf("text provenance changed: %+v", text)
			}
			// Input display is a value. Changing the projection later cannot
			// reclassify a previously built tree.
			child.Display = noneDisplay
			if node.Box != tc.box {
				t.Fatal("input mutation changed a retained box")
			}
		})
	}
}

func TestBuildLayoutTreeRootValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		root *RenderNode
	}{
		{"inline", newRenderElement(42, inlineDisplay)},
		{"inline_block", newRenderElement(42, inlineBlockDisplay)},
		{"text", newRenderText(42, "text")},
		{"empty_text", newRenderText(42, "")},
		{"missing_HTML", &RenderNode{ID: 42, Display: blockDisplay}},
		{"document", &RenderNode{ID: 42, HTML: &html.Node{Type: html.DocumentNode}, Display: blockDisplay}},
		{"comment", &RenderNode{ID: 42, HTML: &html.Node{Type: html.CommentNode}, Display: blockDisplay}},
		{"doctype", &RenderNode{ID: 42, HTML: &html.Node{Type: html.DoctypeNode}, Display: blockDisplay}},
		{"hidden_non_element", &RenderNode{ID: 42, HTML: &html.Node{Type: html.DocumentNode}, Display: noneDisplay}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := *tc.root
			assertBuildDisplayError(t, tc.root, ErrInvalidRoot, 42)
			if !reflect.DeepEqual(*tc.root, before) {
				t.Fatal("root validation mutated input")
			}
		})
	}
	for _, display := range []Display{blockDisplay, flowRootDisplay, inlineDisplay, inlineBlockDisplay} {
		t.Run("replaced_"+display.String(), func(t *testing.T) {
			root := newRenderElement(42, display)
			root.Style = &ComputedStyle{Width: Length{Kind: LenPx, Value: 10}}
			root.Replaced = &ReplacedContent{Height: 10}
			assertBuildDisplayError(t, root, ErrInvalidRoot, 42)
		})
	}
}

func TestBuildLayoutTreeBlockFlowRoot(t *testing.T) {
	for _, isRoot := range []bool{true, false} {
		input := newRenderElement(42, flowRootDisplay, newRenderText(43, "contents"))
		if !isRoot {
			input = newRenderElement(1, blockDisplay, input)
		}
		node, err := BuildLayoutTree(input, BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if !isRoot {
			node = node.Children[0]
		}
		if node.Box != BoxBlock || node.FC != FCBlock || !node.EstablishesBFC || node.NodeID != 42 {
			t.Fatalf("incorrect flow-root: %+v", node)
		}
		if len(node.Children) != 1 || node.Children[0].Box != BoxAnonymousInline || node.Children[0].EstablishesBFC {
			t.Fatalf("flow-root must retain a normal inline group: %+v", node.Children)
		}
	}
}

func TestBuildLayoutTreeRejectsLegacyDisplay(t *testing.T) {
	for _, legacy := range []string{"", "block", "inline", "inline-block", "none", "block-inline", "block flow", "grid"} {
		for _, typed := range []Display{{}, blockDisplay, noneDisplay} {
			t.Run(legacy+"/"+typed.String(), func(t *testing.T) {
				for _, isRoot := range []bool{true, false} {
					input := newRenderElement(42, typed)
					input.Styles["display"] = legacy
					if !isRoot {
						input = newRenderElement(1, blockDisplay, input)
					}
					assertBuildDisplayError(t, input, ErrInvalidDisplay, 42)
				}
			})
		}
	}
	text := newRenderText(42, "text")
	text.Styles = map[string]string{"display": "inline"}
	assertBuildDisplayError(t, newRenderElement(1, blockDisplay, text), ErrInvalidDisplay, 42)
}

func TestBuildLayoutTreeTextHasNoDisplay(t *testing.T) {
	for _, d := range []Display{blockDisplay, inlineDisplay, inlineBlockDisplay, noneDisplay} {
		text := newRenderText(42, "text")
		text.Display = d
		assertBuildDisplayError(t, newRenderElement(1, blockDisplay, text), ErrInvalidDisplay, 42)
	}
}

func TestBuildLayoutTreeRejectsNonElementDescendants(t *testing.T) {
	for _, node := range []*html.Node{nil, {Type: html.DocumentNode}, {Type: html.CommentNode}, {Type: html.DoctypeNode}} {
		child := &RenderNode{ID: 42, HTML: node, Display: blockDisplay}
		input := newRenderElement(1, blockDisplay, child)
		if root, err := BuildLayoutTree(input, BuildOptions{}); root != nil || err == nil || !strings.Contains(err.Error(), "node 42:") {
			t.Fatalf("accepted non-element descendant or lost its identity: %+v, %v", root, err)
		}
	}
}

func TestBuildLayoutTreeSuppressesBeforeBuildingContents(t *testing.T) {
	bad := newRenderElement(3, Display{})
	bad.Styles["display"] = "grid"
	hidden := newRenderElement(2, noneDisplay, bad)
	hidden.Style = &ComputedStyle{Width: Length{Value: -1}}
	hidden.Replaced = &ReplacedContent{Height: -1}
	for _, input := range []*RenderNode{nil, hidden} {
		root, err := BuildLayoutTree(input, BuildOptions{})
		if err != nil || root != nil {
			t.Fatalf("expected no root or error, got %+v, %v", root, err)
		}
	}
	root, err := BuildLayoutTree(newRenderElement(1, blockDisplay,
		newRenderText(4, "before"), hidden, newRenderText(5, "after")), BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	group := root.Children[0]
	if len(root.Children) != 1 || group.Box != BoxAnonymousInline || len(group.Children) != 2 ||
		group.Children[0].NodeID != 4 || group.Children[1].NodeID != 5 {
		t.Fatalf("suppressed subtree contributed boxes: %+v", root)
	}
}

func TestBuildLayoutTreeTypedReplacedDisplay(t *testing.T) {
	for _, d := range []Display{inlineDisplay, inlineBlockDisplay} {
		leaf := newRenderElement(2, d)
		leaf.Replaced = &ReplacedContent{Height: 10}
		leaf.Style = &ComputedStyle{Width: Length{Kind: LenPx, Value: 20}}
		root, err := BuildLayoutTree(newRenderElement(1, blockDisplay, leaf), BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		box := root.Children[0].Children[0]
		if box.Box != BoxInlineBlock || box.NodeID != 2 || box.Replaced == nil || box.Replaced.Height != 10 || len(box.Children) != 0 {
			t.Fatalf("replaced element did not produce an atomic leaf: %+v", box)
		}
	}
	leaf := newRenderElement(2, blockDisplay)
	leaf.Replaced = &ReplacedContent{Height: 10}
	leaf.Style = &ComputedStyle{Width: Length{Kind: LenPx, Value: 20}}
	if root, err := BuildLayoutTree(newRenderElement(1, blockDisplay, leaf), BuildOptions{}); err == nil || root != nil {
		t.Fatal("accepted block-level replaced content outside the supported subset")
	}
}

func assertBuildDisplayError(t *testing.T, input *RenderNode, want error, nodeID NodeID) {
	t.Helper()
	root, err := BuildLayoutTree(input, BuildOptions{})
	if root != nil || !errors.Is(err, want) {
		t.Fatalf("BuildLayoutTree() = %+v, %v; want nil, %v", root, err, want)
	}
	// Keep the source node identifiable for adapters reporting DOM locations.
	if !strings.Contains(err.Error(), fmt.Sprintf("node %d:", nodeID)) {
		t.Fatalf("error lost source identity: %v", err)
	}
}
