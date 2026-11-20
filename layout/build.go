package layout

import (
	"fmt"

	"golang.org/x/net/html"
)

// LayoutTrees separates normal-flow input from fixed roots. IDs are unique
// across both branches of one build. Fixed roots are in source preorder;
// nested fixed boxes are separate roots, not children of another fixed root.
// Nodes, styles and child/root slices must remain read-only after publication.
type LayoutTrees struct {
	Flow  *LayoutNode
	Fixed []*LayoutNode
}

type treeBuilder struct {
	*boxIDGen
	options BuildOptions
	fixed   []*LayoutNode
}

// BuildLayoutTrees normalizes a render subtree and diverts fixed boxes before
// normal-flow grouping, without mutating input. Static roots follow the same
// restrictions as BuildLayoutTree. A non-replaced fixed element may use any
// supported display pair and produces a fixed-only result with Flow == nil.
// Nil or display:none input produces an empty result. Errors return nil, never
// partial branches. Fixed sizing and placement are not implemented by this pass.
func BuildLayoutTrees(renderRoot *RenderNode, opts BuildOptions) (*LayoutTrees, error) {
	if renderRoot == nil {
		return &LayoutTrees{}, nil
	}
	if renderRoot.HTMLNode() == nil || renderRoot.HTMLNode().Type != html.ElementNode {
		return nil, fmt.Errorf("node %d: %w: expected an HTML element", renderRoot.ID, ErrInvalidRoot)
	}
	if err := validateRenderDisplay(renderRoot); err != nil {
		return nil, err
	}
	if renderRoot.Display.Kind == DisplayNone {
		return &LayoutTrees{}, nil
	}
	style, err := buildStyle(renderRoot, 0, WhiteSpaceNormal)
	if err != nil {
		return nil, err
	}
	gen := &treeBuilder{boxIDGen: newBoxIDGen(), options: opts}
	rootID := gen.newRoot(renderRoot.ID)
	var root *LayoutNode
	if style.Position == PositionFixed {
		err = gen.divertFixed(renderRoot, rootID, style)
	} else {
		if renderRoot.Replaced != nil || renderRoot.Display.Outer != OuterBlock {
			return nil, fmt.Errorf("node %d: %w: expected a non-replaced block flow or block flow-root element, got %s", renderRoot.ID, ErrInvalidRoot, renderRoot.Display)
		}
		root, err = buildBlockContainerWithStyle(gen, renderRoot, BoxBlock, rootID, style)
	}
	if err != nil {
		return nil, err
	}
	if gen.err != nil {
		return nil, gen.err
	}
	return &LayoutTrees{Flow: root, Fixed: gen.fixed}, nil
}

func (b *treeBuilder) divertFixed(r *RenderNode, id BoxID, style *ComputedStyle) error {
	if r.Replaced != nil {
		return fmt.Errorf("node %d: %w: fixed replaced elements", r.ID, ErrUnsupportedPosition)
	}
	// Reserve the source-order slot before discovering nested fixed roots.
	index := len(b.fixed)
	b.fixed = append(b.fixed, nil)
	root, err := buildBlockContainerWithStyle(b, r, BoxBlock, id, style)
	if err != nil {
		return err
	}
	b.fixed[index] = root
	return nil
}
