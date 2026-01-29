package layout

type UsedValuesTable map[BoxID]UsedValues

type UsedValues struct {
	Margin       Edges
	Padding      Edges
	Border       Edges
	ContentWidth float32
}

type ResolvePolicy struct{}

type ResolveContext struct {
	ContainingBlock Rect
	FontSizePx      float32
	Policy          ResolvePolicy
}

type EdgeLengths struct {
	Top, Right, Bottom, Left Length
}

type ComputedStyle struct {
	Width      Length
	Margin     EdgeLengths
	Padding    EdgeLengths
	Border     EdgeLengths
	FontSizePx float32
}

func defaultComputedStyle() ComputedStyle {
	return ComputedStyle{
		Width:   Length{Kind: LenAuto},
		Margin:  EdgeLengths{},
		Padding: EdgeLengths{},
		Border:  EdgeLengths{},
	}
}

func resolveLength(l Length, ctx ResolveContext) (px float32, isAuto bool) {
	switch l.Kind {
	case LenPx:
		return l.Value, false
	case LenPercent:
		return ctx.ContainingBlock.W * l.Value, false
	case LenEm:
		return ctx.FontSizePx * l.Value, false
	case LenAuto:
		return 0, true
	default:
		return 0, true
	}
}

type marginAutoFlags struct {
	Left  bool
	Right bool
}

func resolveEdges(style ComputedStyle, ctx ResolveContext) (margin, padding, border Edges, marginAuto marginAutoFlags) {
	var auto bool

	margin.Left, marginAuto.Left = resolveLength(style.Margin.Left, ctx)
	margin.Right, marginAuto.Right = resolveLength(style.Margin.Right, ctx)
	margin.Top, _ = resolveLength(style.Margin.Top, ctx)
	margin.Bottom, _ = resolveLength(style.Margin.Bottom, ctx)

	padding.Left, auto = resolveLength(style.Padding.Left, ctx)
	if auto {
		padding.Left = 0
	}
	padding.Right, auto = resolveLength(style.Padding.Right, ctx)
	if auto {
		padding.Right = 0
	}
	padding.Top, auto = resolveLength(style.Padding.Top, ctx)
	if auto {
		padding.Top = 0
	}
	padding.Bottom, auto = resolveLength(style.Padding.Bottom, ctx)
	if auto {
		padding.Bottom = 0
	}

	border.Left, auto = resolveLength(style.Border.Left, ctx)
	if auto {
		border.Left = 0
	}
	border.Right, auto = resolveLength(style.Border.Right, ctx)
	if auto {
		border.Right = 0
	}
	border.Top, auto = resolveLength(style.Border.Top, ctx)
	if auto {
		border.Top = 0
	}
	border.Bottom, auto = resolveLength(style.Border.Bottom, ctx)
	if auto {
		border.Bottom = 0
	}

	return margin, padding, border, marginAuto
}

func resolveContentWidth(kind BoxKind, style ComputedStyle, ctx ResolveContext, margin, padding, border Edges) float32 {
	if !IsBlockLevel(kind) {
		return 0
	}

	width, isAuto := resolveLength(style.Width, ctx)
	if isAuto {
		if kind == BoxInlineBlock {
			return 0
		}
		content := ctx.ContainingBlock.W -
			(margin.Left + margin.Right + padding.Left + padding.Right + border.Left + border.Right)
		if content < 0 {
			return 0
		}
		return content
	}
	if width < 0 {
		return 0
	}
	return width
}

func styleOrDefault(s *ComputedStyle) ComputedStyle {
	if s == nil {
		return defaultComputedStyle()
	}
	return *s
}

func childResolveContext(node *LayoutNode, parent ResolveContext, used UsedValues) ResolveContext {
	ctx := parent
	if node != nil && IsBlockLevel(node.Box) && node.Box != BoxInlineBlock {
		ctx.ContainingBlock.W = used.ContentWidth
	}
	if node != nil && node.Box == BoxInlineBlock {
		ctx.ContainingBlock.W = parent.ContainingBlock.W
	}
	if node != nil && node.Style != nil && node.Style.FontSizePx > 0 {
		ctx.FontSizePx = node.Style.FontSizePx
	}
	return ctx
}

func resolveUsedValues(node *LayoutNode, ctx ResolveContext, table UsedValuesTable) {
	if node == nil {
		return
	}
	style := styleOrDefault(node.Style)

	margin, padding, border, _ := resolveEdges(style, ctx)
	contentW := resolveContentWidth(node.Box, style, ctx, margin, padding, border)

	used := UsedValues{
		Margin:       margin,
		Padding:      padding,
		Border:       border,
		ContentWidth: contentW,
	}
	table[node.BoxID] = used

	childCtx := childResolveContext(node, ctx, used)
	for _, child := range node.Children {
		resolveUsedValues(child, childCtx, table)
	}
}
