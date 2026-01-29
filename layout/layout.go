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
func ComputeLayoutWithConstraints(
	root *LayoutNode,
	inline InlineLayouter, // black box, produces line boxes for BoxAnonymousInline
	intrinsic IntrinsicMeasurer, // provides initial max-content approximations
	cb ContainingBlock,
	opts LayoutOptions,
) (*LayoutResult, error) {
	return nil, errNotImplemented
}

type LayoutResult struct {
	Root         *LayoutNode
	LinesByBlock map[NodeID][]LineBox // line boxes produced for each block container
}

func layoutBlockContainer(
	node *LayoutNode,
	ctx BlockContext,
	inline InlineLayouter,
	intrinsic IntrinsicMeasurer,
	res *LayoutResult,
) error {
	return errNotImplemented
}

func layoutBlockChildrenVertical(
	parent *LayoutNode,
	children []*LayoutNode,
	ctx BlockContext,
	inline InlineLayouter,
	intrinsic IntrinsicMeasurer,
	res *LayoutResult,
) (contentHeight float32, err error) {
	return 0, errNotImplemented
}
