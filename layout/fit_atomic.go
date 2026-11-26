package layout

import (
	"fmt"
	"maps"
	"math"
	"slices"
)

// ParagraphAtomic is an indivisible occurrence, in source order. Consumed
// occurrences belong to the checkpoint and retain their accepted dimensions;
// they must not be placed again. Margin is separate from the border dimensions.
type ParagraphAtomic struct {
	RealizedAtomic
	Margin   Edges
	Consumed bool
}

// AtomicFragment owns a frozen interior. Geometry is local to the atomic border
// origin; Placement is relative to the enclosing paragraph owner's content.
// Inspection returns copies and never requires the fitting service to be live.
type AtomicFragment struct {
	interior  *fitAtomic
	owner     BoxID
	placement AtomicPlacement
}

type fitAtomic struct {
	item       RealizedAtomic
	geometry   LayoutGeometryTable
	used       UsedValuesTable
	paragraphs []ParagraphFragment
}

func (a AtomicFragment) Item() RealizedAtomic {
	if a.interior == nil {
		return RealizedAtomic{}
	}
	return a.interior.item
}
func (a AtomicFragment) Owner() BoxID               { return a.owner }
func (a AtomicFragment) Placement() AtomicPlacement { return a.placement }
func (a AtomicFragment) Geometry() LayoutGeometryTable {
	if a.interior == nil {
		return nil
	}
	return maps.Clone(a.interior.geometry)
}
func (a AtomicFragment) UsedValues() UsedValuesTable {
	if a.interior == nil {
		return nil
	}
	return maps.Clone(a.interior.used)
}
func (a AtomicFragment) Paragraphs() []ParagraphFragment {
	if a.interior == nil {
		return nil
	}
	return cloneFitParagraphs(a.interior.paragraphs)
}
func (p ParagraphFragment) Atomics() []AtomicFragment { return slices.Clone(p.atomics) }
func (a FitAlternative) Atomics() []AtomicFragment {
	if a.result == nil {
		return nil
	}
	var out []AtomicFragment
	for _, p := range a.result.fragments {
		out = append(out, p.atomics...)
	}
	return out
}

type fitChoice struct {
	ParagraphAlternative
	accepted []AtomicFragment
	locked   []*fitAtomic
}

func atomicFitInputs(items, locked []*fitAtomic) []ParagraphAtomic {
	out := make([]ParagraphAtomic, len(items))
	for i, a := range items {
		out[i] = ParagraphAtomic{a.item, a.used[a.item.BoxID].Margin, i < len(locked)}
	}
	return out
}

func freezeFitChoice(a ParagraphAlternative, owner BoxID, items, locked []*fitAtomic) (fitChoice, error) {
	c := fitChoice{ParagraphAlternative: a, locked: slices.Clone(locked)}
	var err error
	c.Checkpoint, err = cloneFitPayload(a.Checkpoint)
	if err == nil {
		c.Output, err = cloneFitPayload(a.Output)
	}
	if err != nil {
		return fitChoice{}, err
	}
	c.Atomics = slices.Clone(a.Atomics)
	n := len(locked) + len(a.Atomics)
	if n > len(items) || a.Complete && n != len(items) {
		return fitChoice{}, fmt.Errorf("%w: incomplete or repeated atomic consumption", ErrFitProtocol)
	}
	expected := make(map[BoxID]*fitAtomic, len(a.Atomics))
	for _, item := range items[len(locked):n] {
		expected[item.item.BoxID] = item
	}
	for _, place := range a.Atomics {
		item := expected[place.BoxID]
		if item == nil || !finite(place.X) || !finite(place.Y) {
			return fitChoice{}, fmt.Errorf("%w: invalid atomic placement or source order", ErrFitProtocol)
		}
		delete(expected, place.BoxID)
		c.accepted = append(c.accepted, AtomicFragment{item, owner, place})
	}
	c.locked = append(c.locked, items[len(locked):n]...)
	return c, nil
}

func (r *fitRun) groupAtomics(group BoxID, locked []*fitAtomic, base UsedValuesTable) ([]*fitAtomic, UsedValuesTable, error) {
	ids := r.fitter.program.atomics[group]
	used := maps.Clone(base)
	items := make([]*fitAtomic, 0, len(ids))
	for i, id := range ids {
		if !r.check(id, true) {
			return nil, nil, r.err
		}
		var a *fitAtomic
		if i < len(locked) {
			a = locked[i]
		} else {
			var err error
			a, err = r.realizeAtomic(id, base)
			if err != nil {
				return nil, nil, err
			}
			if r.status != FitExhausted {
				return nil, nil, r.err
			}
		}
		items = append(items, a)
		maps.Copy(used, a.used)
	}
	return items, used, nil
}

func (r *fitRun) realizeAtomic(id BoxID, base UsedValuesTable) (*fitAtomic, error) {
	node := r.fitter.program.nodes[id]
	used, err := finalizeAutoWidths(node, base, r.fitter.options.Intrinsic)
	if err != nil {
		return nil, fmt.Errorf("%w: atomic %d: %w", ErrInvalidFit, id, err)
	}
	collector := fitAtomicInline{run: r, used: used, geometry: LayoutGeometryTable{}}
	_, err = layoutBlockContainer(node, used, collector.geometry, &collector, r.fitter.options.Intrinsic, false)
	if err != nil {
		return nil, fmt.Errorf("%w: atomic %d: %w", ErrInvalidFit, id, err)
	}
	return r.snapshotAtomic(id, used, collector.geometry, collector.paragraphs), nil
}

// Core recursively completes inner atomics before invoking this adapter. Thus
// the enclosing callback sees definite dimensions and matching frozen output.
type fitAtomicInline struct {
	run        *fitRun
	used       UsedValuesTable
	geometry   LayoutGeometryTable
	paragraphs []ParagraphFragment
}

func (c *fitAtomicInline) LayoutInline(req InlineRequest) (InlineResult, error) {
	r := c.run
	var items []*fitAtomic
	for _, id := range r.fitter.program.atomics[req.Group.BoxID] {
		items = append(items, r.snapshotAtomic(id, c.used, c.geometry, c.paragraphs))
	}
	choices := r.enumerateParagraph(ParagraphFitRequest{Root: r.fitter.program.root, Group: req.Group.BoxID, Owner: req.OwnerID, Width: req.ContentWidth, Height: math.MaxFloat32, Used: req.Used, Atomics: atomicFitInputs(items, nil)}, items, nil, true)
	if r.status != FitExhausted {
		// The run already records Stopped or Failed. This internal error only
		// unwinds FlowLayout; it must not turn voluntary interruption into failure.
		return InlineResult{}, fmt.Errorf("atomic interior enumeration interrupted")
	}
	if len(choices) == 0 || !choices[0].Complete {
		return InlineResult{}, fmt.Errorf("%w: atomic interior requires a complete paragraph", ErrFitProtocol)
	}
	a := choices[0]
	c.paragraphs = append(c.paragraphs, ParagraphFragment{Group: req.Group.BoxID, Owner: req.OwnerID, To: a.End, Complete: true, Extent: a.Extent, Output: a.Output, allowsMarginCollapse: a.AllowsMarginCollapse, atomics: a.accepted})
	return InlineResult{Extent: a.Extent, Atomics: a.Atomics, AllowsMarginCollapse: a.AllowsMarginCollapse}, nil
}

func (r *fitRun) snapshotAtomic(id BoxID, used UsedValuesTable, geometry LayoutGeometryTable, paragraphs []ParagraphFragment) *fitAtomic {
	a := &fitAtomic{used: UsedValuesTable{}, geometry: LayoutGeometryTable{}}
	var copyTree func(*LayoutNode)
	copyTree = func(n *LayoutNode) {
		a.used[n.BoxID] = used[n.BoxID]
		if g, ok := geometry[n.BoxID]; ok {
			a.geometry[n.BoxID] = g
		}
		for _, child := range n.Children {
			copyTree(child)
		}
	}
	copyTree(r.fitter.program.nodes[id])
	g := a.geometry[id]
	// The snapshot's root is local even when inspected after outer placement.
	g.Content.X -= g.Frame.X
	g.Content.Y -= g.Frame.Y
	g.Frame.X, g.Frame.Y, g.RelativeTo = 0, 0, 0
	a.geometry[id] = g
	a.item = RealizedAtomic{id, g.Frame.W, g.Frame.H}
	for _, p := range paragraphs {
		if _, ok := a.used[p.Group]; ok {
			p.X, p.Y = fitContentOrigin(p.Owner, a.geometry)
			a.paragraphs = append(a.paragraphs, p)
		}
	}
	return a
}
