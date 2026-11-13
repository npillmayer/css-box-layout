package layout

import "fmt"

// D -> E: create a correct CSS2 box tree with anonymous boxes and FC boundaries.
func BuildLayoutTree(renderRoot *RenderNode, opts BuildOptions) (*LayoutNode, error) {
	if renderRoot == nil {
		return nil, nil
	}
	gen := newBoxIDGen()
	rootID := gen.newRoot(renderRoot.ID)
	root, err := buildBlockContainer(gen, renderRoot, BoxBlock, rootID)
	if err != nil {
		return nil, err
	}
	if gen.err != nil {
		return nil, gen.err
	}
	return root, nil
}

// E -> used values: resolve margins/padding/borders/widths into a table keyed by BoxID.
func ResolveUsedValues(root *LayoutNode, ctx ResolveContext) (UsedValuesTable, error) {
	used := make(UsedValuesTable)
	if root == nil {
		return used, nil
	}
	resolveUsedValues(root, ctx, used)
	return used, nil
}

// E -> F: compute block geometry; call a black-box inline layouter for BoxAnonymousInline owners.
func FlowLayout(
	root *LayoutNode,
	used UsedValuesTable,
	inline InlineLayouter, // black box, produces overall group extent
	intrinsic IntrinsicMeasurer, // provides initial max-content approximations
	ctx LayoutContext,
	opts LayoutOptions,
) (*LayoutResult, error) {
	if root == nil {
		return &LayoutResult{
			Root:     nil,
			Geometry: make(LayoutGeometryTable),
		}, nil
	}
	if err := validateUsedValues(root, used); err != nil {
		return nil, err
	}
	geom := make(LayoutGeometryTable)
	err := layoutBlockContainer(root, used, geom, inline, intrinsic)
	if err != nil {
		return nil, err
	}
	return &LayoutResult{
		Root:     root,
		Geometry: geom,
	}, nil
}

// ComputeLayoutWithConstraints is kept for compatibility; prefer FlowLayout.
func ComputeLayoutWithConstraints(
	root *LayoutNode,
	used UsedValuesTable,
	inline InlineLayouter,
	intrinsic IntrinsicMeasurer,
	ctx LayoutContext,
	opts LayoutOptions,
) (*LayoutResult, error) {
	return FlowLayout(root, used, inline, intrinsic, ctx, opts)
}

type LayoutResult struct {
	Root     *LayoutNode
	Geometry LayoutGeometryTable
}

func layoutBlockContainer(
	node *LayoutNode,
	used UsedValuesTable,
	geom LayoutGeometryTable,
	inline InlineLayouter,
	intrinsic IntrinsicMeasurer,
) error {
	if node == nil {
		return nil
	}
	u := used[node.BoxID]
	content := Rect{
		X: u.Border.Left + u.Padding.Left,
		Y: u.Border.Top + u.Padding.Top,
		W: u.ContentWidth,
	}
	frame := Rect{
		X: 0,
		Y: 0,
		W: u.ContentWidth + u.Padding.Left + u.Padding.Right + u.Border.Left + u.Border.Right,
	}

	if isInlineOnlyBlockContainer(node) {
		if inline == nil {
			return fmt.Errorf("box %d: inline layouter required", node.BoxID)
		}
		inlineRoot := node.Children[0]
		extent, err := inline.LayoutInline(InlineRequest{
			Group: inlineRoot, OwnerID: node.BoxID, ContentWidth: content.W,
			Used:   usedValuesView{table: used},
			Atomic: atomicSizer{inline: inline, intrinsic: intrinsic, used: used, geom: geom},
		})
		if err != nil {
			return fmt.Errorf("inline group %d (owner %d): %w", inlineRoot.BoxID, node.BoxID, err)
		}
		if !finite(extent.W) || !finite(extent.H) || extent.W < 0 || extent.H < 0 {
			return fmt.Errorf("inline group %d: invalid layout extent %+v", inlineRoot.BoxID, extent)
		}
		content.H = extent.H
	} else {
		h, err := layoutBlockChildrenVertical(node.Children, used, geom, inline, intrinsic)
		if err != nil {
			return err
		}
		content.H = h
	}

	frame.H = content.H + u.Padding.Top + u.Padding.Bottom + u.Border.Top + u.Border.Bottom

	if !finite(frame.W) || !finite(frame.H) {
		return fmt.Errorf("box %d: non-finite layout dimensions", node.BoxID)
	}
	geom[node.BoxID] = LayoutGeometry{Frame: frame, Content: content}
	node.Frame = frame
	node.Content = content
	return nil
}

func layoutBlockChildrenVertical(
	children []*LayoutNode,
	used UsedValuesTable,
	geom LayoutGeometryTable,
	inline InlineLayouter,
	intrinsic IntrinsicMeasurer,
) (contentHeight float32, err error) {
	var y float32
	for _, child := range children {
		if child == nil {
			continue
		}
		cu := used[child.BoxID]
		y += cu.Margin.Top
		err := layoutBlockContainer(child, used, geom, inline, intrinsic)
		if err != nil {
			return 0, err
		}
		childGeom := geom[child.BoxID]
		// Child geometry is relative to the parent content origin, not its border box.
		childGeom.Frame.X = cu.Margin.Left
		childGeom.Frame.Y = y
		childGeom.Content.X += childGeom.Frame.X
		childGeom.Content.Y += childGeom.Frame.Y
		geom[child.BoxID] = childGeom
		child.Frame = childGeom.Frame
		child.Content = childGeom.Content
		y += childGeom.Frame.H + cu.Margin.Bottom
	}
	return y, nil
}
