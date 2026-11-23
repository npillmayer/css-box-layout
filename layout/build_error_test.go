package layout

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/npillmayer/css-box-layout/text"
	"golang.org/x/net/html"
)

func TestBuildErrorsLocateSources(t *testing.T) {
	cases := []struct {
		name   string
		change func(*RenderNode, *RenderNode)
		cause  error
		node   NodeID
	}{
		{"root-kind", func(r, c *RenderNode) { r.HTML.Type = html.TextNode }, ErrInvalidRoot, 10},
		{"root-inline", func(r, c *RenderNode) { r.Display = inlineDisplay }, ErrInvalidRoot, 10},
		{"root-replaced", func(r, c *RenderNode) { r.Replaced = &ReplacedContent{} }, ErrInvalidRoot, 10},
		{"display", func(r, c *RenderNode) { c.Display = Display{} }, ErrInvalidDisplay, 20},
		{"legacy-display", func(r, c *RenderNode) { c.Styles["display"] = "block" }, ErrInvalidDisplay, 20},
		{"missing-html", func(r, c *RenderNode) { c.HTML = nil }, nil, 20},
		{"legacy-style", func(r, c *RenderNode) { c.Styles["width"] = "10px" }, nil, 20},
		{"typed-style", func(r, c *RenderNode) { c.Style = &ComputedStyle{FontSizePx: -1} }, nil, 20},
		{"position", func(r, c *RenderNode) { c.Style = &ComputedStyle{Position: PositionKind(99)} }, ErrUnsupportedPosition, 20},
		{"fixed-on-static", func(r, c *RenderNode) { c.Style = &ComputedStyle{Fixed: &FixedStyle{}} }, ErrInvalidPosition, 20},
		{"fixed-replaced", func(r, c *RenderNode) {
			c.Style = &ComputedStyle{Position: PositionFixed}
			c.Replaced = &ReplacedContent{}
		}, ErrUnsupportedPosition, 20},
		{"replaced", func(r, c *RenderNode) { c.Replaced = &ReplacedContent{} }, nil, 20},
		{"text-display", func(r, c *RenderNode) { c.HTML.Type = html.TextNode }, ErrInvalidDisplay, 20},
		{"text-position", func(r, c *RenderNode) {
			*c = *newRenderText(20, "x")
			c.Style = &ComputedStyle{Position: PositionFixed}
		}, ErrInvalidPosition, 20},
		{"missing-text-ref", func(r, c *RenderNode) { *c = *newRenderText(20, "x"); c.Text = nil }, nil, 20},
		{"reversed-range", func(r, c *RenderNode) { *c = *newRenderText(20, "x"); c.Text.Range.Start = 2 }, nil, 20},
		{"wrong-length", func(r, c *RenderNode) { *c = *newRenderText(20, "x"); c.Text.Range.End = 3 }, nil, 20},
		{"invalid-utf8", func(r, c *RenderNode) { *c = *newRenderText(20, "\xff") }, nil, 20},
		{"zero-source-id", func(r, c *RenderNode) { c.ID = 0; c.Display = Display{} }, ErrInvalidDisplay, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			child := newRenderElement(20, blockDisplay)
			root := newRenderElement(10, blockDisplay, newRenderElement(11, blockDisplay, newRenderText(12, "valid earlier branch")), child)
			tc.change(root, child)
			for _, multi := range []bool{false, true} {
				var err error
				if multi {
					var got *LayoutTrees
					got, err = BuildLayoutTrees(root, BuildOptions{})
					if got != nil {
						t.Fatal("partial trees")
					}
				} else {
					var got *LayoutNode
					got, err = BuildLayoutTree(root, BuildOptions{})
					if got != nil {
						t.Fatal("partial tree")
					}
				}
				var located *BuildError
				if !errors.As(err, &located) || !located.HasNode || located.RootID != 10 || located.NodeID != tc.node || located.Err == nil {
					t.Fatalf("bad location: %+v (%v)", located, err)
				}
				if tc.cause != nil && !errors.Is(err, tc.cause) {
					t.Fatalf("lost sentinel %v: %v", tc.cause, err)
				}
			}
		})
	}
}

func TestBuildErrorPreservesReaderCauseAndNestedLocation(t *testing.T) {
	cause := errors.New("text storage unavailable")
	readerCause := &fs.PathError{Op: "read", Path: "content.html", Err: cause}
	upstream := &BuildError{RootID: 999, NodeID: 998, HasNode: true, Err: readerCause}
	root := newRenderElement(7, blockDisplay, newRenderElement(8, inlineDisplay, newRenderElement(9, inlineBlockDisplay, newRenderText(10, "x"))))
	trees, err := BuildLayoutTrees(root, BuildOptions{ReadText: func(text.TextRef) (string, error) { return "", upstream }})
	var located *BuildError
	if trees != nil || !errors.As(err, &located) || located.RootID != 7 || located.NodeID != 10 || !errors.Is(err, cause) || !errors.Is(err, upstream) {
		t.Fatalf("lost local/cause identity: %v", err)
	}
	var pathError *fs.PathError
	if !errors.As(err, &pathError) || pathError != readerCause {
		t.Fatal("typed reader cause lost", err)
	}
	if upstream.RootID != 999 || upstream.NodeID != 998 {
		t.Fatal("mutated caller error")
	}
	if located == upstream || !strings.Contains(located.Error(), "read text") {
		t.Fatal("reader location replaced builder location")
	}
}

func TestBuildErrorFixedAndTreeBoundary(t *testing.T) {
	fixed := newRenderElement(4, blockDisplay)
	fixed.Style = &ComputedStyle{Position: PositionFixed}
	root := newRenderElement(1, blockDisplay, fixed)
	got, err := BuildLayoutTree(root, BuildOptions{})
	var located *BuildError
	if got != nil || !errors.As(err, &located) || located.RootID != 1 || located.NodeID != 4 || !located.HasNode || !errors.Is(err, ErrFixedLayoutRequired) {
		t.Fatalf("fixed diagnostic: %v", err)
	}
	// Allocation-wide failures have a branch boundary, not an invented source
	// for an anonymous wrapper. Exhausting uint64 space is not a practical fixture.
	err = buildRootError(17, errBoxIDExhausted)
	if !errors.As(err, &located) || located.RootID != 17 || located.HasNode || located.NodeID != 0 || !errors.Is(err, errBoxIDExhausted) {
		t.Fatalf("tree boundary: %v", err)
	}
	if !strings.Contains(err.Error(), "tree rooted at node 17") {
		t.Fatal(err)
	}
	// The public result is independently wrapped, not a mutated inner error.
	original := buildError(2, fmt.Errorf("%w", ErrInvalidDisplay))
	err = buildRootError(18, original)
	if original.(*BuildError).RootID != 0 || err.(*BuildError).RootID != 18 {
		t.Fatal("root context mutated inner error")
	}
}

func TestBuildStyleErrorOrderIsDeterministic(t *testing.T) {
	for range 20 {
		root := newRenderElement(1, blockDisplay)
		root.Styles = map[string]string{"width": "10px", "font-size": "20px", "padding": "1px"}
		_, err := BuildLayoutTree(root, BuildOptions{})
		if err == nil || !strings.Contains(err.Error(), "provide font-size through typed") {
			t.Fatal(err)
		}
	}
}

func ExampleBuildError() {
	root := &RenderNode{ID: 10, HTML: &html.Node{Type: html.ElementNode, Data: "div"}}
	_, err := BuildLayoutTree(root, BuildOptions{}) // Missing typed display.
	var construction *BuildError
	if errors.As(err, &construction) {
		fmt.Println(construction.RootID, construction.HasNode, construction.NodeID)
	}
	fmt.Println(errors.Is(err, ErrInvalidDisplay))
	// Output:
	// 10 true 10
	// true
}
