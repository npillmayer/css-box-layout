package layout

type UsedValuesTable map[BoxID]UsedValues

type UsedValues struct {
	Margin       Edges
	Padding      Edges
	Border       Edges
	ContentWidth float32
	// Pending values must not be consumed as final layout constraints.
	WidthPending    bool
	EdgesPending    bool
	containingWidth float32
	fontSizePx      float32
}

type ResolvePolicy struct{}

type ResolveContext struct {
	widthUnresolved bool
	ContainingBlock Rect
	FontSizePx      float32
	Policy          ResolvePolicy
}

type EdgeLengths struct {
	Top, Right, Bottom, Left Length
}

type ComputedStyle struct {
	WhiteSpace WhiteSpace // computed inherited value; zero is normal
	Width      Length
	Margin     EdgeLengths
	Padding    EdgeLengths
	Border     EdgeLengths
	FontSizePx float32
}

// DefaultComputedStyle returns auto width and zero edges. Set fields on this
// value before passing it to RenderNode.Style; a zero Width means explicit 0px.
func DefaultComputedStyle() ComputedStyle {
	return ComputedStyle{
		Width:   Length{Kind: LenAuto},
		Margin:  EdgeLengths{},
		Padding: EdgeLengths{},
		Border:  EdgeLengths{},
	}
}

func defaultComputedStyle() ComputedStyle { return DefaultComputedStyle() }

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
	if !IsBlockContainer(kind) {
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
	if node != nil && IsBlockContainer(node.Box) {
		ctx.ContainingBlock.W = used.ContentWidth
		ctx.widthUnresolved = used.WidthPending
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
	// em lengths use this element's computed font size, not its parent's.
	if style.FontSizePx > 0 {
		ctx.FontSizePx = style.FontSizePx
	}

	margin, padding, border, auto := resolveEdges(style, ctx)
	contentW := resolveContentWidth(node.Box, style, ctx, margin, padding, border)

	used := UsedValues{
		Margin:       margin,
		Padding:      padding,
		Border:       border,
		ContentWidth: contentW,
		WidthPending: node.Box == BoxInlineBlock && style.Width.Kind == LenAuto ||
			IsBlockContainer(node.Box) && ctx.widthUnresolved && (style.Width.Kind == LenAuto || style.Width.Kind == LenPercent),
		EdgesPending:    ctx.widthUnresolved && (percentEdges(style) || IsBlockLevel(node.Box)),
		containingWidth: ctx.ContainingBlock.W,
		fontSizePx:      ctx.FontSizePx,
	}
	// Block margins depend on the containing width even when this box has a
	// definite width. Keep them pending below an auto inline-block until its
	// final width triggers private descendant re-resolution.
	if IsBlockLevel(node.Box) && !ctx.widthUnresolved {
		used.Margin = resolveBlockMargins(style.Width.Kind == LenAuto, auto, ctx.ContainingBlock.W, used)
	}
	table[node.BoxID] = used

	childCtx := childResolveContext(node, ctx, used)
	for _, child := range node.Children {
		resolveUsedValues(child, childCtx, table)
	}
}

// resolveBlockMargins applies the CSS normal-flow width equation for the current
// LTR outer-flow contract. Inline-blocks are intentionally excluded: their auto
// margins resolve to zero, independently of their shrink-to-fit width.
func resolveBlockMargins(widthAuto bool, auto marginAutoFlags, containing float32, u UsedValues) Edges {
	m := u.Margin // auto margins already have zero provisional values
	// Accumulate in float64 so finite signed edges do not overflow intermediate
	// float32 sums. The final used values remain subject to flow validation.
	available := float64(containing) - float64(u.ContentWidth) -
		float64(u.Padding.Left) - float64(u.Padding.Right) -
		float64(u.Border.Left) - float64(u.Border.Right)
	remaining := available - float64(m.Left) - float64(m.Right)
	// A positive auto width has already absorbed the available space. Do not
	// turn float32 rounding residue into a nonzero auto margin.
	if widthAuto && (u.ContentWidth > 0 || remaining >= 0) {
		return m
	}
	if !widthAuto && remaining >= 0 {
		switch {
		case auto.Left && auto.Right:
			m.Left, m.Right = float32(remaining/2), float32(remaining/2)
			return m
		case auto.Left:
			m.Left = float32(remaining)
			return m
		case auto.Right:
			m.Right = float32(remaining)
			return m
		}
	}
	// Auto width fills the available space with auto margins at zero. If that
	// width was clamped to zero, or a definite box overflows, preserve the left
	// edge and solve the right margin. The same LTR rule resolves fixed margins
	// when no value is auto, even when the residual is positive.
	m.Right = float32(available - float64(m.Left))
	return m
}
