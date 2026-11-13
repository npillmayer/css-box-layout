package layout

import "fmt"

type intrinsicAtoms map[BoxID]IntrinsicAtomic

func (v intrinsicAtoms) Lookup(id BoxID) (IntrinsicAtomic, bool) { a, ok := v[id]; return a, ok }

type intrinsicEdges map[BoxID]BoxEdges

func (v intrinsicEdges) Lookup(id BoxID) (BoxEdges, bool) { e, ok := v[id]; return e, ok }

func percentEdges(s ComputedStyle) bool {
	for _, e := range []EdgeLengths{s.Margin, s.Padding, s.Border} {
		for _, l := range []Length{e.Top, e.Right, e.Bottom, e.Left} {
			if l.Kind == LenPercent {
				return true
			}
		}
	}
	return false
}
func validIntrinsic(w IntrinsicWidths) bool {
	return finite(w.MinContent) && finite(w.MaxContent) && w.MinContent >= 0 && w.MaxContent >= w.MinContent
}
func horizontal(e Edges) float32 { return e.Left + e.Right }
func addIntrinsic(w IntrinsicWidths, extra float32) (IntrinsicWidths, error) {
	w.MinContent = max(float32(0), w.MinContent+extra)
	w.MaxContent = max(float32(0), w.MaxContent+extra)
	if !finite(extra) || !validIntrinsic(w) {
		return IntrinsicWidths{}, fmt.Errorf("invalid intrinsic contribution")
	}
	return w, nil
}

type intrinsicPass struct {
	measure IntrinsicMeasurer
	bounds  map[BoxID]IntrinsicWidths // Content bounds only; no retained layout payload.
}

func measureEdges(n *LayoutNode, ctx ResolveContext) (BoxEdges, ResolveContext, error) {
	s := styleOrDefault(n.Style)
	if s.FontSizePx > 0 {
		ctx.FontSizePx = s.FontSizePx
	}
	if ctx.widthUnresolved && percentEdges(s) {
		return BoxEdges{}, ctx, fmt.Errorf("box %d: percentage edges depend on unresolved auto width", n.BoxID)
	}
	m, p, b, _ := resolveEdges(s, ctx)
	e := BoxEdges{m, p, b}
	for _, g := range []struct {
		e      Edges
		signed bool
	}{{m, true}, {p, false}, {b, false}} {
		for _, v := range []float32{g.e.Top, g.e.Right, g.e.Bottom, g.e.Left} {
			if !finite(v) || (!g.signed && v < 0) {
				return BoxEdges{}, ctx, fmt.Errorf("box %d: invalid measurement edges", n.BoxID)
			}
		}
	}
	return e, ctx, nil
}

func (p *intrinsicPass) border(n *LayoutNode, ctx ResolveContext) (IntrinsicWidths, BoxEdges, error) {
	e, ctx, err := measureEdges(n, ctx)
	if err != nil {
		return IntrinsicWidths{}, e, err
	}
	s := styleOrDefault(n.Style)
	var w IntrinsicWidths
	if s.Width.Kind == LenAuto {
		w, err = p.content(n, ctx.FontSizePx, e)
	} else {
		if s.Width.Kind == LenPercent && ctx.widthUnresolved {
			return w, e, fmt.Errorf("box %d: percentage width depends on unresolved auto width", n.BoxID)
		}
		width, auto := resolveLength(s.Width, ctx)
		if auto || !finite(width) || width < 0 {
			return w, e, fmt.Errorf("box %d: invalid intrinsic width", n.BoxID)
		}
		w = IntrinsicWidths{width, width}
	}
	if err != nil {
		return w, e, err
	}
	w, err = addIntrinsic(w, horizontal(e.Padding)+horizontal(e.Border))
	return w, e, err
}

func (p *intrinsicPass) content(n *LayoutNode, font float32, own BoxEdges) (IntrinsicWidths, error) {
	if w, ok := p.bounds[n.BoxID]; ok {
		return w, nil
	}
	ctx := ResolveContext{FontSizePx: font, widthUnresolved: true}
	var w IntrinsicWidths
	if isInlineOnlyBlockContainer(n) {
		if p.measure == nil {
			return w, fmt.Errorf("box %d: intrinsic measurer required for auto width", n.BoxID)
		}
		group := n.Children[0]
		atoms := intrinsicAtoms{}
		edges := intrinsicEdges{n.BoxID: own}
		var walk func(*LayoutNode, ResolveContext) error
		walk = func(c *LayoutNode, context ResolveContext) error {
			if c == nil {
				return nil
			}
			if c.Box == BoxInlineBlock {
				b, e, err := p.border(c, context)
				if err != nil {
					return err
				}
				atoms[c.BoxID] = IntrinsicAtomic{c.BoxID, b}
				edges[c.BoxID] = e
				return nil
			}
			if c.Box != BoxInline && c.Box != BoxAnonymousInline && c.Box != BoxText {
				return fmt.Errorf("group %d contains block-flow box %d", group.BoxID, c.BoxID)
			}
			e, next, err := measureEdges(c, context)
			if err != nil {
				return err
			}
			edges[c.BoxID] = e
			for _, child := range c.Children {
				if err := walk(child, next); err != nil {
					return err
				}
			}
			return nil
		}
		if err := walk(group, ctx); err != nil {
			return w, err
		}
		var err error
		w, err = p.measure.MeasureInline(InlineMeasureRequest{Group: group, OwnerID: n.BoxID, Edges: edges, Atomics: atoms})
		if err != nil {
			return w, fmt.Errorf("measure inline group %d (owner %d): %w", group.BoxID, n.BoxID, err)
		}
	} else {
		for _, child := range n.Children {
			if child == nil {
				continue
			}
			b, e, err := p.border(child, ctx)
			if err != nil {
				return w, err
			}
			b, err = addIntrinsic(b, horizontal(e.Margin))
			if err != nil {
				return w, err
			}
			w.MinContent = max(w.MinContent, b.MinContent)
			w.MaxContent = max(w.MaxContent, b.MaxContent)
		}
	}
	if !validIntrinsic(w) {
		return w, fmt.Errorf("box %d: invalid intrinsic bounds %+v", n.BoxID, w)
	}
	p.bounds[n.BoxID] = w
	return w, nil
}

// Work only on a private copy. Complete late resolution before any final layout
// call, so callbacks cannot observe a partially updated used-values view.
func finalizeAutoWidths(root *LayoutNode, input UsedValuesTable, measure IntrinsicMeasurer) (UsedValuesTable, error) {
	used := make(UsedValuesTable, len(input))
	for id, u := range input {
		used[id] = u
	}
	pass := intrinsicPass{measure: measure, bounds: map[BoxID]IntrinsicWidths{}}
	var visit func(*LayoutNode, ResolveContext, bool) error
	visit = func(n *LayoutNode, ctx ResolveContext, refresh bool) error {
		if n == nil {
			return nil
		}
		if refresh {
			resolveUsedValues(n, ctx, used)
		}
		u := used[n.BoxID]
		if u.EdgesPending {
			return fmt.Errorf("box %d: unresolved edges", n.BoxID)
		}
		auto := n.Box == BoxInlineBlock && styleOrDefault(n.Style).Width.Kind == LenAuto
		if auto {
			w, err := pass.content(n, u.fontSizePx, BoxEdges{u.Margin, u.Padding, u.Border})
			if err != nil {
				return err
			}
			available := ctx.ContainingBlock.W - horizontal(u.Margin) - horizontal(u.Padding) - horizontal(u.Border)
			if !finite(available) {
				return fmt.Errorf("box %d: invalid available width", n.BoxID)
			}
			available = max(float32(0), available)
			u.ContentWidth = min(max(w.MinContent, available), w.MaxContent)
			u.WidthPending = false
			used[n.BoxID] = u
		}
		if u.WidthPending {
			return fmt.Errorf("box %d: unresolved content width", n.BoxID)
		}
		next := childResolveContext(n, ctx, u)
		next.FontSizePx = u.fontSizePx
		for _, c := range n.Children {
			if err := visit(c, next, auto); err != nil {
				return err
			}
		}
		return nil
	}
	u := used[root.BoxID]
	if err := visit(root, ResolveContext{ContainingBlock: Rect{W: u.containingWidth}, FontSizePx: u.fontSizePx}, false); err != nil {
		return nil, err
	}
	if err := validateUsedValues(root, used); err != nil {
		return nil, err
	}
	return used, nil
}
