package layout

import "fmt"

// FixedLayoutOptions supplies the fallback font size where computed font size
// is absent. It must be finite and nonnegative; em lengths require a positive
// effective font size. FixedLayout does not invent a default font size.
type FixedLayoutOptions struct {
	FontSizePx float32
}

// FixedLayoutResult owns a fresh root slice and geometry table, sharing the
// immutable structural nodes. Treat the result as read-only. Each root has
// RelativeTo == 0 and viewport-local Frame/Content coordinates; add Viewport.X/Y
// once when composing with an external coordinate space. Ordinary descendants
// remain relative to their parent's content box. Roots retain input order,
// which is not a paint-order decision.
type FixedLayoutResult struct {
	Roots    []*LayoutNode
	Viewport Rect
	Geometry LayoutGeometryTable
}

// FixedLayout lays out the diverted fixed roots from BuildLayoutTrees against
// one caller-supplied viewport. It does not invoke FlowLayout. Every root must
// be a normalized non-replaced fixed block with at least one non-auto inset on
// each axis. BoxIDs must be nonzero and unique across the entire forest.
// Viewport dimensions must be finite and nonnegative (zero is definite), and
// its origin must be finite. Empty input returns an empty result.
//
// Inputs and earlier results are never mutated. All widths and descendant used
// values are prepared before final inline callbacks; every inline group is laid
// out once. Callbacks must honor the read-only contract and retain their results
// per invocation. On any error, no partial result is returned; the client must
// discard any adapter results from that invocation as well.
//
// This branch defines no page-layout, clipping, painting, or repetition policy.
func FixedLayout(
	roots []*LayoutNode,
	viewport Rect,
	inline InlineLayouter,
	intrinsic IntrinsicMeasurer,
	opts FixedLayoutOptions,
) (*FixedLayoutResult, error) {
	if err := validateFixedContext(viewport, opts.FontSizePx); err != nil {
		return nil, err
	}
	seen := map[BoxID]bool{}
	for _, root := range roots {
		if err := validateFixedTree(root, opts.FontSizePx, seen); err != nil {
			return nil, err
		}
	}
	// Size every root before final callbacks. The cache belongs to this call;
	// forest validation ensures that identities cannot alias its entries.
	pass := intrinsicPass{measure: intrinsic, bounds: map[BoxID]IntrinsicWidths{}}
	type preparedRoot struct {
		sizing *fixedSizing
		used   UsedValuesTable
	}
	prepared := make([]preparedRoot, len(roots))
	for i, root := range roots {
		sizing, err := pass.resolveFixedSizing(root, viewport, opts.FontSizePx)
		if err != nil {
			return nil, err
		}
		// The fixed equations own root sizing/margins. Only descendants use
		// ordinary flow resolution, based on the root's selected content width.
		used := UsedValuesTable{root.BoxID: sizing.Used}
		ctx := childResolveContext(root, ResolveContext{
			ContainingBlock: viewport, FontSizePx: sizing.Used.fontSizePx,
		}, sizing.Used)
		for _, child := range root.Children {
			resolveUsedValues(child, ctx, used)
		}
		used, err = pass.finalizeAutoWidths(root, used)
		if err != nil {
			return nil, err
		}
		prepared[i] = preparedRoot{sizing, used}
	}
	geom := make(LayoutGeometryTable)
	for i, root := range roots {
		p := prepared[i]
		interior, err := layoutBlockContent(root, p.used, geom, inline, intrinsic, true)
		if err != nil {
			return nil, err
		}
		sizing, err := p.sizing.withContentHeight(interior.Height)
		if err != nil {
			return nil, err
		}
		g, err := fixedRootGeometry(*sizing)
		if err != nil {
			return nil, err
		}
		// Set the root's final size and origin once. Descendants already use
		// parent-content coordinates, including any overflow of a fixed height.
		geom[root.BoxID] = g
	}
	return &FixedLayoutResult{
		Roots: append([]*LayoutNode(nil), roots...), Viewport: viewport, Geometry: geom,
	}, nil
}

func fixedRootGeometry(s fixedSizing) (LayoutGeometry, error) {
	u, h, v := s.Used, s.Horizontal, s.Vertical
	x, y := float64(h.Start)+float64(h.MarginStart), float64(v.Start)+float64(v.MarginStart)
	left, top := float64(u.Padding.Left)+float64(u.Border.Left), float64(u.Padding.Top)+float64(u.Border.Top)
	g := LayoutGeometry{
		Frame: Rect{X: float32(x), Y: float32(y),
			W: float32(left + float64(h.Size) + float64(u.Padding.Right) + float64(u.Border.Right)),
			H: float32(top + float64(v.Size) + float64(u.Padding.Bottom) + float64(u.Border.Bottom))},
		Content: Rect{X: float32(x + left), Y: float32(y + top), W: h.Size, H: v.Size},
	}
	for _, value := range []float32{g.Frame.X, g.Frame.Y, g.Frame.W, g.Frame.H, g.Content.X, g.Content.Y} {
		if !finite(value) {
			return LayoutGeometry{}, fmt.Errorf("box %d: fixed geometry overflow", s.boxID)
		}
	}
	return g, nil
}
