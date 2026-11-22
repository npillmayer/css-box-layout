package layout

import "fmt"

// fixedSizing is invocation-local root sizing, not published geometry. Used has
// final width/edges for later descendant resolution. Vertical must not be read
// while HeightPending; FixedLayout completes it with the interior's content height.
// The captured vertical constraints contain values, never aliases to input style.
type fixedSizing struct {
	Used          UsedValues
	Horizontal    fixedAxisResult
	Vertical      fixedAxisResult
	HeightPending bool
	vertical      fixedAxis
	boxID         BoxID
}

type fixedLength struct {
	value float64
	auto  bool
}

type fixedAxis struct {
	containing             float64
	start, end, size       fixedLength
	marginStart, marginEnd fixedLength
	edgeStart, edgeEnd     float64 // padding + border
}

type fixedAxisResult struct {
	Start, End, Size       float32 // insets to margin edges; Size is content size
	MarginStart, MarginEnd float32
}

// resolveFixedSizing resolves one constructed fixed root in the bounded LTR
// profile. It measures only when auto width has one definite inset. It does not
// lay out interiors, resolve descendants, produce geometry, or call FlowLayout.
// Both-auto inset axes require static-position support and are rejected here.
func resolveFixedSizing(root *LayoutNode, viewport Rect, fontSizePx float32, measure IntrinsicMeasurer) (*fixedSizing, error) {
	if err := validateFixedContext(viewport, fontSizePx); err != nil {
		return nil, err
	}
	if err := validateFixedSizingTree(root, fontSizePx); err != nil {
		return nil, err
	}
	pass := intrinsicPass{measure: measure, bounds: map[BoxID]IntrinsicWidths{}}
	return pass.resolveFixedSizing(root, viewport, fontSizePx)
}

// The caller validates the tree/context first. Keeping one intrinsic pass for
// sizing and descendant preparation avoids measuring nested auto boxes twice.
func (pass *intrinsicPass) resolveFixedSizing(root *LayoutNode, viewport Rect, fontSizePx float32) (*fixedSizing, error) {
	c, err := prepareFixedConstraints(styleOrDefault(root.Style), viewport, fontSizePx)
	if err != nil {
		return nil, fmt.Errorf("box %d: %w", root.BoxID, err)
	}
	var contentWidth float64
	if c.horizontal.needsContentSize() {
		edges := BoxEdges{c.used.Margin, c.used.Padding, c.used.Border}
		bounds, err := pass.content(root, c.used.fontSizePx, edges)
		if err != nil {
			return nil, err
		}
		x := c.horizontal
		// Auto margins and the unresolved inset are zero for available width.
		available := max(0, x.containing-x.start.value-x.end.value-x.marginStart.value-x.marginEnd.value-x.edgeStart-x.edgeEnd)
		contentWidth = min(max(float64(bounds.MinContent), available), float64(bounds.MaxContent))
	}
	out, err := c.resolve(contentWidth)
	if err != nil {
		return nil, fmt.Errorf("box %d: %w", root.BoxID, err)
	}
	out.boxID = root.BoxID
	return &out, nil
}

// fixedConstraints is the shared, tree-free sizing kernel. Preparation snapshots
// typed values and resolves definite lengths; only the tree path supplies an
// intrinsic content width or later completes a pending content height.
type fixedConstraints struct {
	horizontal, vertical fixedAxis
	used                 UsedValues
	verticalResult       fixedAxisResult // Solved before any intrinsic measurement, unless pending.
}

// The caller validates context and computed style before preparation.
func prepareFixedConstraints(s ComputedStyle, viewport Rect, fontSizePx float32) (fixedConstraints, error) {
	if s.FontSizePx > 0 {
		fontSizePx = s.FontSizePx
	}
	f := DefaultFixedStyle()
	if s.Fixed != nil {
		f = *s.Fixed
	}
	if f.Insets.Left.Kind == LenAuto && f.Insets.Right.Kind == LenAuto || f.Insets.Top.Kind == LenAuto && f.Insets.Bottom.Kind == LenAuto {
		return fixedConstraints{}, fmt.Errorf("%w: static-position fallback requires a non-auto inset on each axis", ErrUnsupportedPosition)
	}
	ctx := ResolveContext{ContainingBlock: viewport, FontSizePx: fontSizePx}
	edges, _, err := resolveStyleEdges(s, ctx)
	if err != nil {
		return fixedConstraints{}, err
	}
	x, err := makeFixedAxis(viewport.W, fontSizePx, f.Insets.Left, f.Insets.Right, s.Width,
		s.Margin.Left, s.Margin.Right, edges.Margin.Left, edges.Margin.Right,
		edges.Padding.Left, edges.Border.Left, edges.Padding.Right, edges.Border.Right)
	if err != nil {
		return fixedConstraints{}, fmt.Errorf("horizontal: %w", err)
	}
	y, err := makeFixedAxis(viewport.H, fontSizePx, f.Insets.Top, f.Insets.Bottom, f.Height,
		s.Margin.Top, s.Margin.Bottom, edges.Margin.Top, edges.Margin.Bottom,
		edges.Padding.Top, edges.Border.Top, edges.Padding.Bottom, edges.Border.Bottom)
	if err != nil {
		return fixedConstraints{}, fmt.Errorf("vertical: %w", err)
	}
	c := fixedConstraints{horizontal: x, vertical: y, used: UsedValues{
		Margin: edges.Margin, Padding: edges.Padding, Border: edges.Border,
		containingWidth: viewport.W, fontSizePx: fontSizePx}}
	if !y.needsContentSize() {
		c.verticalResult, err = y.solve(0, false)
		if err != nil {
			return fixedConstraints{}, fmt.Errorf("vertical: %w", err)
		}
	}
	return c, nil
}

func (c fixedConstraints) resolve(contentWidth float64) (fixedSizing, error) {
	out := fixedSizing{
		Used: c.used, Vertical: c.verticalResult, HeightPending: c.vertical.needsContentSize(), vertical: c.vertical,
	}
	var err error
	if !out.HeightPending {
		out.Used.Margin.Top, out.Used.Margin.Bottom = out.Vertical.MarginStart, out.Vertical.MarginEnd
	}
	out.Horizontal, err = c.horizontal.solve(contentWidth, true)
	if err != nil {
		return fixedSizing{}, fmt.Errorf("horizontal: %w", err)
	}
	out.Used.ContentWidth = out.Horizontal.Size
	out.Used.Margin.Left, out.Used.Margin.Right = out.Horizontal.MarginStart, out.Horizontal.MarginEnd
	return out, nil
}

// withContentHeight completes an auto-content height without mutating the receiver.
// Prescribed/stretch heights are retained even if natural content overflows them.
func (s fixedSizing) withContentHeight(height float32) (*fixedSizing, error) {
	if !finite(height) || height < 0 {
		return nil, fmt.Errorf("box %d: invalid fixed content height", s.boxID)
	}
	if s.HeightPending {
		v, err := s.vertical.solve(float64(height), false)
		if err != nil {
			return nil, fmt.Errorf("box %d: vertical: %w", s.boxID, err)
		}
		s.Vertical, s.HeightPending = v, false
		s.Used.Margin.Top, s.Used.Margin.Bottom = v.MarginStart, v.MarginEnd
	}
	return &s, nil
}

func makeFixedAxis(containing, font float32, start, end, size, marginStart, marginEnd Length,
	usedMarginStart, usedMarginEnd, paddingStart, borderStart, paddingEnd, borderEnd float32) (fixedAxis, error) {
	length := func(l Length) (fixedLength, error) {
		v, auto := resolveLength(l, ResolveContext{ContainingBlock: Rect{W: containing}, FontSizePx: font})
		if !finite(v) {
			return fixedLength{}, fmt.Errorf("non-finite fixed length")
		}
		return fixedLength{float64(v), auto}, nil
	}
	a, err := length(start)
	if err != nil {
		return fixedAxis{}, err
	}
	b, err := length(end)
	if err != nil {
		return fixedAxis{}, err
	}
	c, err := length(size)
	if err != nil {
		return fixedAxis{}, err
	}
	return fixedAxis{
		containing: float64(containing), start: a, end: b, size: c,
		marginStart: fixedLength{float64(usedMarginStart), marginStart.Kind == LenAuto},
		marginEnd:   fixedLength{float64(usedMarginEnd), marginEnd.Kind == LenAuto},
		edgeStart:   float64(paddingStart) + float64(borderStart),
		edgeEnd:     float64(paddingEnd) + float64(borderEnd),
	}, nil
}

func (a fixedAxis) needsContentSize() bool {
	return a.size.auto && (a.start.auto || a.end.auto)
}

// solve implements CSS 2.2 10.3.7/10.6.4 with a zero minimum content size.
// Accumulate signed constraints in float64; validate every published float32.
func (a fixedAxis) solve(contentSize float64, horizontal bool) (fixedAxisResult, error) {
	start, end, size := a.start.value, a.end.value, a.size.value
	ms, me := a.marginStart.value, a.marginEnd.value
	fixedSize := !a.size.auto
	if a.size.auto {
		if a.needsContentSize() {
			size = contentSize
		} else {
			size = a.containing - start - end - ms - me - a.edgeStart - a.edgeEnd
			if size < 0 {
				// Re-solve with the default min-size (0) as a definite size,
				// including auto-margin behavior, without changing computed style.
				size, fixedSize = 0, true
			}
		}
	}
	if fixedSize && !a.start.auto && !a.end.auto {
		remaining := a.containing - start - end - ms - me - a.edgeStart - size - a.edgeEnd
		switch {
		case a.marginStart.auto && a.marginEnd.auto:
			if horizontal && remaining < 0 {
				ms, me = 0, remaining
			} else {
				ms, me = remaining/2, remaining/2
			}
		case a.marginStart.auto:
			ms = remaining
		case a.marginEnd.auto:
			me = remaining
		default:
			end = a.containing - start - ms - a.edgeStart - size - a.edgeEnd - me
		}
	} else if a.start.auto {
		start = a.containing - end - ms - a.edgeStart - size - a.edgeEnd - me
	} else {
		end = a.containing - start - ms - a.edgeStart - size - a.edgeEnd - me
	}
	for _, v := range []float64{start, end, size, ms, me, a.edgeStart + size + a.edgeEnd, start + ms, start + ms + a.edgeStart} {
		if !finite(float32(v)) {
			return fixedAxisResult{}, fmt.Errorf("fixed sizing overflow")
		}
	}
	return fixedAxisResult{float32(start), float32(end), float32(size), float32(ms), float32(me)}, nil
}
