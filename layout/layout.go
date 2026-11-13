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

// FlowLayout computes fresh geometry without mutating root or used. Callbacks
// must honor the read-only input contract and keep retained results per invocation.
// On failure no result is returned; previous results and inputs remain unchanged.
func FlowLayout(
	root *LayoutNode,
	used UsedValuesTable,
	inline InlineLayouter, // black box, produces group extent and atomic placements
	intrinsic IntrinsicMeasurer, // measures groups for auto sizing; fixed-width flow does not call it
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
	if err := validateAtomicWidths(root); err != nil {
		return nil, err
	}
	resolved, err := finalizeAutoWidths(root, used, intrinsic)
	if err != nil {
		return nil, err
	}
	used = resolved
	geom := make(LayoutGeometryTable)
	err = layoutBlockContainer(root, used, geom, inline, intrinsic)
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

// LayoutResult owns a fresh geometry table and shares the immutable structural
// root. Treat results as read-only; use a new tree for changed styles/structure.
// FlowLayout does not retain or mutate the caller's used-values table.
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
		items, err := prepareAtomics(inlineRoot, used, geom, inline, intrinsic)
		if err != nil {
			return err
		}
		result, err := inline.LayoutInline(InlineRequest{
			Group: inlineRoot, OwnerID: node.BoxID, ContentWidth: content.W,
			Used:    usedValuesView{table: used},
			Atomics: preparedAtomicsView{items: items},
		})
		if err != nil {
			return fmt.Errorf("inline group %d (owner %d): %w", inlineRoot.BoxID, node.BoxID, err)
		}
		extent := result.Extent
		if !finite(extent.W) || !finite(extent.H) || extent.W < 0 || extent.H < 0 {
			return fmt.Errorf("inline group %d: invalid layout extent %+v", inlineRoot.BoxID, extent)
		}
		if err := placeAtomics(result.Atomics, items, node.BoxID, geom); err != nil {
			return fmt.Errorf("inline group %d: %w", inlineRoot.BoxID, err)
		}
		content.H = extent.H
	} else {
		h, err := layoutBlockChildrenVertical(node.BoxID, node.Children, used, geom, inline, intrinsic)
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
	return nil
}

func layoutBlockChildrenVertical(
	owner BoxID,
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
		childGeom.RelativeTo = owner
		// Child geometry is relative to the parent content origin, not its border box.
		childGeom.Frame.X = cu.Margin.Left
		childGeom.Frame.Y = y
		childGeom.Content.X += childGeom.Frame.X
		childGeom.Content.Y += childGeom.Frame.Y
		geom[child.BoxID] = childGeom
		y += childGeom.Frame.H + cu.Margin.Bottom
	}
	return y, nil
}
