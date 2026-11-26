package layout

import "fmt"

// Own the structure as well as the styles: continuations must survive edits to
// the caller's input. Atomic interiors get provenance, but no resumable cuts.
func snapshotFitProgram(root *LayoutNode, fallbackFont float32) (*fitProgram, error) {
	p := &fitProgram{root: root.BoxID, cuts: map[fitEdge]fitCut{}, all: map[fitEdge]StructuralBoundary{}, nodes: map[BoxID]*LayoutNode{}, atomics: map[BoxID][]BoxID{}}
	var order uint64
	var walk func(*LayoutNode, BoxID, BoxID, bool, float32) (*LayoutNode, error)
	walk = func(src *LayoutNode, owner, group BoxID, interior bool, font float32) (*LayoutNode, error) {
		if src == nil || src.BoxID == 0 || p.nodes[src.BoxID] != nil {
			return nil, fitError(0, ErrInvalidFit, "nil, duplicate or zero box identity")
		}
		n := *src
		style := styleOrDefault(src.Style)
		if err := validateComputedStyle(style); err != nil {
			return nil, &FitError{n.BoxID, fmt.Errorf("%w: %w", ErrInvalidFit, err)}
		}
		if style.Position != PositionStatic {
			return nil, fitError(n.BoxID, ErrUnsupportedFit, "positioned material")
		}
		if style.FontSizePx > 0 {
			font = style.FontSizePx
		}
		if font == 0 && fixedStyleUsesEm(style) {
			return nil, fitError(n.BoxID, ErrInvalidFit, "em length requires a positive font size")
		}
		if style.Fixed != nil {
			fixed := *style.Fixed
			style.Fixed = &fixed
		}
		n.Style, n.Children = &style, nil
		if src.Replaced != nil {
			value := *src.Replaced
			n.Replaced = &value
		}
		p.nodes[n.BoxID] = &n
		block, atomic := IsBlockLevel(n.Box), n.Box == BoxInlineBlock
		if atomic {
			p.atomics[group] = append(p.atomics[group], n.BoxID)
			interior = true
		}
		if block || atomic {
			owner = n.BoxID
		}
		if n.Box == BoxAnonymousInline {
			group = n.BoxID
		}
		order++
		before := StructuralBoundary{Root: p.root, Box: n.BoxID, Group: group, Edge: BoundaryBefore, Order: order}
		p.all[fitEdge{n.BoxID, BoundaryBefore}] = before
		if !interior && (block || n.Box == BoxAnonymousInline) {
			p.cuts[fitEdge{n.BoxID, BoundaryBefore}] = fitCut{len(p.ops), before}
			kind := fitGroup
			if block {
				kind = fitEnter
			}
			p.ops = append(p.ops, fitOp{kind, n.BoxID, owner})
		}
		switch {
		case block || atomic:
			if n.Replaced == nil && len(src.Children) == 0 {
				return nil, fitError(n.BoxID, ErrInvalidFit, "normalized container needs an empty group")
			}
			inline := len(src.Children) == 1 && src.Children[0] != nil && src.Children[0].Box == BoxAnonymousInline
			for _, c := range src.Children {
				if c == nil || !inline && !IsBlockLevel(c.Box) {
					return nil, fitError(n.BoxID, ErrInvalidFit, "mixed or unnormalized block children")
				}
			}
		case n.Box == BoxInline || n.Box == BoxAnonymousInline:
			for _, c := range src.Children {
				if c == nil || c.Box != BoxInline && c.Box != BoxText && c.Box != BoxInlineBlock {
					return nil, fitError(n.BoxID, ErrInvalidFit, "unnormalized inline children")
				}
			}
		case n.Box == BoxText:
			if len(src.Children) != 0 || n.Text.Range.End < n.Text.Range.Start {
				return nil, fitError(n.BoxID, ErrInvalidFit, "invalid text leaf")
			}
		default:
			return nil, fitError(n.BoxID, ErrUnsupportedFit, "box kind")
		}
		childGroup := group
		if atomic {
			childGroup = 0
		}
		for _, c := range src.Children {
			child, err := walk(c, owner, childGroup, interior, font)
			if err != nil {
				return nil, err
			}
			n.Children = append(n.Children, child)
		}
		if block && !interior {
			p.ops = append(p.ops, fitOp{fitExit, n.BoxID, owner})
		}
		order++
		after := StructuralBoundary{Root: p.root, Box: n.BoxID, Group: group, Edge: BoundaryAfter, Order: order}
		p.all[fitEdge{n.BoxID, BoundaryAfter}] = after
		if !interior && (block || n.Box == BoxAnonymousInline) {
			p.cuts[fitEdge{n.BoxID, BoundaryAfter}] = fitCut{len(p.ops), after}
		}
		return &n, nil
	}
	var err error
	p.tree, err = walk(root, 0, 0, false, fallbackFont)
	if err != nil {
		return nil, err
	}
	if err = validateAtomicWidths(p.tree); err != nil {
		return nil, &FitError{root.BoxID, fmt.Errorf("%w: %w", ErrInvalidFit, err)}
	}
	for key, cut := range p.cuts {
		if key.edge == BoundaryAfter {
			for cut.index < len(p.ops) && p.ops[cut.index].kind == fitExit {
				cut.index++
			}
			p.cuts[key] = cut
		}
	}
	return p, nil
}
