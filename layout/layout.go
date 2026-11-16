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

	var contentThrough bool
	var childMargins childMarginSummary
	if node.Replaced != nil {
		content.H = node.Replaced.Height
	} else if isInlineOnlyBlockContainer(node) {
		if inline == nil {
			return blockMarginSummary{}, fmt.Errorf("box %d: inline layouter required", node.BoxID)
		}
		inlineRoot := node.Children[0]
		items, err := prepareAtomics(inlineRoot, used, geom, inline, intrinsic)
		if err != nil {
			return blockMarginSummary{}, err
		}
		result, err := inline.LayoutInline(InlineRequest{
			Group: inlineRoot, OwnerID: node.BoxID, ContentWidth: content.W,
			Used:    usedValuesView{table: used},
			Atomics: preparedAtomicsView{items: items},
		})
		if err != nil {
			return blockMarginSummary{}, fmt.Errorf("inline group %d (owner %d): %w", inlineRoot.BoxID, node.BoxID, err)
		}
		extent := result.Extent
		if !finite(extent.W) || !finite(extent.H) || extent.W < 0 || extent.H < 0 {
			return blockMarginSummary{}, fmt.Errorf("inline group %d: invalid layout extent %+v", inlineRoot.BoxID, extent)
		}
		if result.AllowsMarginCollapse && (extent.H != 0 || len(items) != 0 || len(result.Atomics) != 0) {
			return blockMarginSummary{}, fmt.Errorf("inline group %d: margin collapse requires zero height and no atomics", inlineRoot.BoxID)
		}
		if err := placeAtomics(result.Atomics, items, node.BoxID, geom); err != nil {
			return blockMarginSummary{}, fmt.Errorf("inline group %d: %w", inlineRoot.BoxID, err)
		}
		content.H = extent.H
		contentThrough = result.AllowsMarginCollapse
	} else {
		h, margins, err := layoutBlockChildrenVertical(node, used, geom, inline, intrinsic, isRoot)
		if err != nil {
			return blockMarginSummary{}, err
		}
		content.H = h
		childMargins = margins
		contentThrough = margins.Through
	}

	frame.H = content.H + u.Padding.Top + u.Padding.Bottom + u.Border.Top + u.Border.Bottom

	if !finite(frame.W) || !finite(frame.H) {
		return blockMarginSummary{}, fmt.Errorf("box %d: non-finite layout dimensions", node.BoxID)
	}
	geom[node.BoxID] = LayoutGeometry{Frame: frame, Content: content}
	eligibility := blockCollapseEligibility(node, u, isRoot, contentThrough)
	return summarizeBlockMargins(u, eligibility, childMargins, frame), nil
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
