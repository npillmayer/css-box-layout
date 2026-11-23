package layout

import "fmt"

// BuildLayoutTree builds normal-flow input. Use BuildLayoutTrees when fixed
// boxes may occur: this entry point returns ErrFixedLayoutRequired rather than
// silently discarding that branch. A static root must be a non-replaced block
// flow or block flow-root element. Nil/display:none input returns (nil, nil).
// Errors are *BuildError values and retain errors.Is/errors.As causes. BoxIDs
// are scoped to this build; qualify them with the caller's tree identity.
func BuildLayoutTree(renderRoot *RenderNode, opts BuildOptions) (*LayoutNode, error) {
	trees, err := BuildLayoutTrees(renderRoot, opts)
	if err != nil {
		return nil, err
	}
	if len(trees.Fixed) != 0 {
		return nil, buildRootError(renderRoot.ID, buildError(trees.Fixed[0].NodeID, fmt.Errorf("%w: use BuildLayoutTrees", ErrFixedLayoutRequired)))
	}
	return trees.Flow, nil
}

// E -> used values: resolve margins/padding/borders/widths into a table keyed by BoxID.
func ResolveUsedValues(root *LayoutNode, ctx ResolveContext) (UsedValuesTable, error) {
	if err := validateNormalFlowPositions(root); err != nil {
		return nil, err
	}
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
	if err := validateNormalFlowPositions(root); err != nil {
		return nil, err
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
	_, err = layoutBlockContainer(root, used, geom, inline, intrinsic, true)
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
	isRoot bool,
) (blockMarginSummary, error) {
	if node == nil {
		return blockMarginSummary{}, nil
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

	interior, err := layoutBlockContent(node, used, geom, inline, intrinsic, isRoot)
	if err != nil {
		return blockMarginSummary{}, err
	}
	content.H = interior.Height
	frame.H = content.H + u.Padding.Top + u.Padding.Bottom + u.Border.Top + u.Border.Bottom

	if !finite(frame.W) || !finite(frame.H) {
		return blockMarginSummary{}, fmt.Errorf("box %d: non-finite layout dimensions", node.BoxID)
	}
	geom[node.BoxID] = LayoutGeometry{Frame: frame, Content: content}
	eligibility := blockCollapseEligibility(node, u, isRoot, interior.Through)
	return summarizeBlockMargins(u, eligibility, interior.Margins, frame), nil
}

type blockContentResult struct {
	Height  float32
	Through bool
	Margins childMarginSummary
}

// Lay out the interior and descendant geometry without constructing the owner's
// frame. Normal flow uses the natural height; fixed roots apply their own height
// constraints before constructing and validating their final geometry.
func layoutBlockContent(
	node *LayoutNode,
	used UsedValuesTable,
	geom LayoutGeometryTable,
	inline InlineLayouter,
	intrinsic IntrinsicMeasurer,
	isRoot bool,
) (blockContentResult, error) {
	if node.Replaced != nil {
		return blockContentResult{Height: node.Replaced.Height}, nil
	}
	if isInlineOnlyBlockContainer(node) {
		if inline == nil {
			return blockContentResult{}, fmt.Errorf("box %d: inline layouter required", node.BoxID)
		}
		inlineRoot := node.Children[0]
		items, err := prepareAtomics(inlineRoot, used, geom, inline, intrinsic)
		if err != nil {
			return blockContentResult{}, err
		}
		result, err := inline.LayoutInline(InlineRequest{
			Group: inlineRoot, OwnerID: node.BoxID, ContentWidth: used[node.BoxID].ContentWidth,
			Used:    usedValuesView{table: used},
			Atomics: preparedAtomicsView{items: items},
		})
		if err != nil {
			return blockContentResult{}, fmt.Errorf("inline group %d (owner %d): %w", inlineRoot.BoxID, node.BoxID, err)
		}
		extent := result.Extent
		if !finite(extent.W) || !finite(extent.H) || extent.W < 0 || extent.H < 0 {
			return blockContentResult{}, fmt.Errorf("inline group %d: invalid layout extent %+v", inlineRoot.BoxID, extent)
		}
		if result.AllowsMarginCollapse && (extent.H != 0 || len(items) != 0 || len(result.Atomics) != 0) {
			return blockContentResult{}, fmt.Errorf("inline group %d: margin collapse requires zero height and no atomics", inlineRoot.BoxID)
		}
		if err := placeAtomics(result.Atomics, items, node.BoxID, geom); err != nil {
			return blockContentResult{}, fmt.Errorf("inline group %d: %w", inlineRoot.BoxID, err)
		}
		return blockContentResult{Height: extent.H, Through: result.AllowsMarginCollapse}, nil
	}
	h, margins, err := layoutBlockChildrenVertical(node, used, geom, inline, intrinsic, isRoot)
	if err != nil {
		return blockContentResult{}, err
	}
	return blockContentResult{Height: h, Through: margins.Through, Margins: margins}, nil
}

// Defer each adjoining group until a non-through child or the owner boundary.
// Empty boxes get their own CSS hypothetical-border origin; they never advance
// the flow cursor. Children retain coordinates relative to this owner's content.
func layoutBlockChildrenVertical(
	owner *LayoutNode,
	used UsedValuesTable,
	geom LayoutGeometryTable,
	inline InlineLayouter,
	intrinsic IntrinsicMeasurer,
	isRoot bool,
) (contentHeight float32, margins childMarginSummary, err error) {
	edges := blockCollapseEligibility(owner, used[owner.BoxID], isRoot, false)
	var y float64
	var pending marginStrut
	atTop := true
	margins.Through = true
	for _, child := range owner.Children {
		if child == nil {
			continue
		}
		summary, err := layoutBlockContainer(child, used, geom, inline, intrinsic, false)
		if err != nil {
			return 0, childMarginSummary{}, err
		}
		margins = margins.append(summary)
		var childY float64
		if summary.Through {
			if !(atTop && edges.TopWithChildren) {
				childY = y + float64(pending.merge(summary.BeforeBottom).value())
			}
			pending = pending.merge(summary.Leading)
		} else {
			pending = pending.merge(summary.Leading)
			if !(atTop && edges.TopWithChildren) {
				childY = y + float64(pending.value())
			}
			y = childY + float64(summary.BorderHeight)
			pending = summary.Trailing
			atTop = false
		}
		if err := placeBlockChild(child.BoxID, owner.BoxID, used[child.BoxID].Margin.Left, childY, geom); err != nil {
			return 0, childMarginSummary{}, err
		}
	}
	// The trailing group is outside an open bottom boundary. If every child
	// collapses through an open top, that same group has already escaped there.
	if !edges.BottomWithChildren && !(atTop && edges.TopWithChildren) {
		y += float64(pending.value())
	}
	height := float32(max(0, y))
	if !finite(height) {
		return 0, childMarginSummary{}, fmt.Errorf("box %d: content height overflow", owner.BoxID)
	}
	return height, margins, nil
}

func placeBlockChild(id, owner BoxID, x float32, y float64, geom LayoutGeometryTable) error {
	g := geom[id]
	contentX := float32(float64(g.Content.X) + float64(x) - float64(g.Frame.X))
	contentY := float32(float64(g.Content.Y) + y - float64(g.Frame.Y))
	if !finite(x) || !finite(float32(y)) || !finite(contentX) || !finite(contentY) {
		return fmt.Errorf("box %d: block coordinate overflow", id)
	}
	g.RelativeTo = owner
	g.Frame.X, g.Frame.Y = x, float32(y)
	g.Content.X, g.Content.Y = contentX, contentY
	geom[id] = g
	return nil
}
