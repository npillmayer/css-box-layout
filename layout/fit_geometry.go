package layout

import (
	"fmt"
	"maps"
	"slices"
)

// Geometry returns copied, fragment-specific rectangles. RelativeTo names the
// parent content origin; zero denotes the receiving rectangle. Only selected
// containers and consumed atomics have rectangles; paragraph positions are in
// Paragraphs. Original source geometry is never modified.
func (a FitAlternative) Geometry() LayoutGeometryTable {
	if a.result == nil {
		return nil
	}
	return maps.Clone(a.result.geometry)
}

// UsedValues returns resolved values for selected material, with crossed top or
// bottom edges omitted. Use these facts with source paint, not source edge widths.
func (a FitAlternative) UsedValues() UsedValuesTable {
	if a.result == nil {
		return nil
	}
	return maps.Clone(a.result.used)
}

func cloneFitParagraphs(input []ParagraphFragment) []ParagraphFragment {
	out := slices.Clone(input)
	for i := range out {
		out[i].Output = out[i].Output.Clone()
		out[i].atomics = slices.Clone(out[i].atomics)
	}
	return out
}

// Replay only already frozen paragraphs. Pruning atomic descendants prevents
// FlowLayout from re-realizing them; accepted interior geometry is merged below.
type fitReplay map[BoxID]ParagraphFragment

func (p fitReplay) LayoutInline(req InlineRequest) (InlineResult, error) {
	a, ok := p[req.Group.BoxID]
	if !ok {
		return InlineResult{}, fmt.Errorf("%w: missing selected paragraph", ErrFitProtocol)
	}
	return InlineResult{Extent: a.Extent, AllowsMarginCollapse: a.allowsMarginCollapse}, nil
}

func (r *fitRun) fragmentGeometry(s fitState) (*fitResult, bool, error) {
	result := &fitResult{fragments: slices.Clone(s.paragraphs), containers: slices.Clone(s.containers), used: UsedValuesTable{}, geometry: LayoutGeometryTable{}}
	if !s.progress {
		return result, true, nil
	}
	containers := make(map[BoxID]ContainerFragment, len(s.containers))
	for _, c := range s.containers {
		containers[c.Box] = c
	}
	paragraphs := fitReplay{}
	for _, p := range s.paragraphs {
		paragraphs[p.Group] = p
	}
	var prune func(*LayoutNode) *LayoutNode
	prune = func(n *LayoutNode) *LayoutNode {
		c, block := containers[n.BoxID]
		_, group := paragraphs[n.BoxID]
		if !block && !group {
			return nil
		}
		copy := *n
		copy.Children = nil
		u := r.used[n.BoxID]
		if block {
			if !c.First {
				u.Margin.Top, u.Padding.Top, u.Border.Top = 0, 0, 0
			}
			if !c.Last {
				u.Margin.Bottom, u.Padding.Bottom, u.Border.Bottom = 0, 0, 0
			}
			for _, child := range n.Children {
				if kept := prune(child); kept != nil {
					copy.Children = append(copy.Children, kept)
				}
			}
		}
		result.used[n.BoxID] = u
		return &copy
	}
	root := prune(r.fitter.program.tree)
	laid, err := FlowLayout(root, result.used, paragraphs, nil, LayoutContext{}, LayoutOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("%w: fragment layout: %w", ErrInvalidFit, err)
	}
	result.geometry = laid.Geometry
	g := result.geometry[root.BoxID]
	u := result.used[root.BoxID]
	g.Frame.X += u.Margin.Left
	g.Frame.Y += u.Margin.Top
	g.Content.X += u.Margin.Left
	g.Content.Y += u.Margin.Top
	result.geometry[root.BoxID] = g
	for i := range result.containers {
		c := &result.containers[i]
		u := result.used[c.Box]
		c.Geometry, c.Edges = result.geometry[c.Box], BoxEdges{u.Margin, u.Padding, u.Border}
	}
	for _, p := range result.fragments {
		for _, a := range p.atomics {
			maps.Copy(result.used, a.interior.used)
			maps.Copy(result.geometry, a.interior.geometry)
			if err := placeAtomics([]AtomicPlacement{a.placement}, map[BoxID]RealizedAtomic{a.Item().BoxID: a.Item()}, a.owner, result.geometry); err != nil {
				return nil, false, fmt.Errorf("%w: %w", ErrFitProtocol, err)
			}
		}
	}
	// Root margins advance within the receiver, without collapsing against a
	// previous/next frame. Physical descendants also count, even if signed margins
	// make the containing block shorter than their occupied bounds.
	w, h := max(float32(0), g.Frame.X+g.Frame.W+u.Margin.Right), max(float32(0), g.Frame.Y+g.Frame.H+u.Margin.Bottom)
	fits := true
	include := func(rect Rect) error {
		if !finite(rect.X) || !finite(rect.Y) || !finite(rect.W) || !finite(rect.H) || !finite(rect.X+rect.W) || !finite(rect.Y+rect.H) || rect.W < 0 || rect.H < 0 {
			return fmt.Errorf("%w: invalid fragment bounds", ErrInvalidFit)
		}
		if rect.X < 0 || !r.req.DeferVerticalBounds && rect.Y < 0 {
			fits = false
		}
		w, h = max(w, rect.X+rect.W), max(h, rect.Y+rect.H)
		return nil
	}
	for _, g := range result.geometry {
		x, y := fitContentOrigin(g.RelativeTo, result.geometry)
		frame := g.Frame
		frame.X += x
		frame.Y += y
		if err := include(frame); err != nil {
			return nil, false, err
		}
	}
	for i := range result.fragments {
		p := &result.fragments[i]
		p.X, p.Y = fitContentOrigin(p.Owner, result.geometry)
		if err := include(Rect{p.X, p.Y, p.Extent.W, p.Extent.H}); err != nil {
			return nil, false, err
		}
		for _, a := range p.atomics {
			// Check every interior paragraph's actual bounds as well as boxes.
			for _, inner := range a.interior.paragraphs {
				x, y := fitContentOrigin(inner.Owner, result.geometry)
				if err := include(Rect{x, y, inner.Extent.W, inner.Extent.H}); err != nil {
					return nil, false, err
				}
			}
		}
	}
	if !finite(w) || !finite(h) {
		return nil, false, fmt.Errorf("%w: fragment extent overflow", ErrInvalidFit)
	}
	result.extent = InlineExtent{w, h}
	return result, fits && w <= r.req.Width && (r.req.DeferVerticalBounds || h <= r.req.Height), nil
}

func fitContentOrigin(id BoxID, geometry LayoutGeometryTable) (x, y float32) {
	for id != 0 {
		g := geometry[id]
		x += g.Content.X
		y += g.Content.Y
		id = g.RelativeTo
	}
	return
}
