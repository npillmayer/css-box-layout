package layout

// D -> E: create a correct CSS2 box tree with anonymous boxes and FC boundaries.
func BuildLayoutTree(renderRoot *RenderNode, opts BuildOptions) (*LayoutNode, error) {
	if renderRoot == nil {
		return nil, nil
	}
	gen := newBoxIDGen()
	rootID := gen.newRoot(renderRoot.ID)
	return buildBlockContainer(gen, renderRoot, BoxBlock, rootID)
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
	inline InlineLayouter, // black box, produces line boxes for BoxAnonymousInline
	intrinsic IntrinsicMeasurer, // provides initial max-content approximations
	ctx LayoutContext,
	opts LayoutOptions,
) (*LayoutResult, error) {
	if root == nil {
		return &LayoutResult{
			Root:     nil,
			Geometry: make(LayoutGeometryTable),
			Lines:    make(LinesByBlock),
		}, nil
	}
	geom := make(LayoutGeometryTable)
	lines := make(LinesByBlock)
	err := layoutBlockContainer(root, used, geom, lines, inline, intrinsic)
	if err != nil {
		return nil, err
	}
	return &LayoutResult{
		Root:     root,
		Geometry: geom,
		Lines:    lines,
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
	Lines    LinesByBlock // line boxes produced for each block container
}

func layoutBlockContainer(
	node *LayoutNode,
	used UsedValuesTable,
	geom LayoutGeometryTable,
	lines LinesByBlock,
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
			return errNotImplemented
		}
		inlineRoot := node.Children[0]
		lineBoxes, err := inline.LayoutInline(
			inlineRoot,
			content.W,
			atomicSizer{inline: inline, intrinsic: intrinsic, used: used, geom: geom, lines: lines},
		)
		if err != nil {
			return err
		}
		if shouldStoreLines(node) {
			lines[node.BoxID] = lineBoxes
		}
		content.H = lineExtent(lineBoxes)
	} else {
		h, err := layoutBlockChildrenVertical(node, node.Children, content, used, geom, lines, inline, intrinsic)
		if err != nil {
			return err
		}
		content.H = h
	}

	frame.H = content.H + u.Padding.Top + u.Padding.Bottom + u.Border.Top + u.Border.Bottom

	geom[node.BoxID] = LayoutGeometry{Frame: frame, Content: content}
	node.Frame = frame
	node.Content = content
	return nil
}

func layoutBlockChildrenVertical(
	parent *LayoutNode,
	children []*LayoutNode,
	content Rect,
	used UsedValuesTable,
	geom LayoutGeometryTable,
	lines LinesByBlock,
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
		err := layoutBlockContainer(child, used, geom, lines, inline, intrinsic)
		if err != nil {
			return 0, err
		}
		childGeom := geom[child.BoxID]
		childGeom.Frame.X = content.X + cu.Margin.Left
		childGeom.Frame.Y = content.Y + y
		childGeom.Content.X += childGeom.Frame.X
		childGeom.Content.Y += childGeom.Frame.Y
		geom[child.BoxID] = childGeom
		child.Frame = childGeom.Frame
		child.Content = childGeom.Content
		y += childGeom.Frame.H + cu.Margin.Bottom
	}
	return y, nil
}
