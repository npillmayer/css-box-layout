package layout

import "fmt"

// Reject unsupported widths before any external work, including in nested groups.
func validateAtomicWidths(n *LayoutNode) error {
	if n == nil {
		return nil
	}
	if n.Box == BoxInlineBlock {
		s := styleOrDefault(n.Style)
		if s.Width.Kind != LenAuto && (s.Width.Kind != LenPx || !finite(s.Width.Value) || s.Width.Value < 0) {
			return fmt.Errorf("atomic box %d: only auto or nonnegative fixed pixel widths are supported", n.BoxID)
		}
	}
	for _, child := range n.Children {
		if err := validateAtomicWidths(child); err != nil {
			return err
		}
	}
	return nil
}

func prepareAtomics(group *LayoutNode, used UsedValuesTable, geom LayoutGeometryTable, inline InlineLayouter, intrinsic IntrinsicMeasurer) (map[BoxID]PreparedAtomic, error) {
	items := make(map[BoxID]PreparedAtomic)
	var walk func(*LayoutNode) error
	walk = func(n *LayoutNode) error {
		if n == nil {
			return nil
		}
		switch n.Box {
		case BoxInlineBlock:
			if _, exists := items[n.BoxID]; exists {
				return fmt.Errorf("duplicate atomic box %d", n.BoxID)
			}
			if err := layoutBlockContainer(n, used, geom, inline, intrinsic); err != nil {
				return err
			}
			g := geom[n.BoxID]
			items[n.BoxID] = PreparedAtomic{BoxID: n.BoxID, BorderWidth: g.Frame.W, BorderHeight: g.Frame.H}
			return nil // Its interior belongs to independent groups.
		case BoxInline, BoxAnonymousInline, BoxText:
		default:
			return fmt.Errorf("inline group %d contains block-flow box %d", group.BoxID, n.BoxID)
		}
		for _, child := range n.Children {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(group); err != nil {
		return nil, err
	}
	return items, nil
}

func placeAtomics(placements []AtomicPlacement, items map[BoxID]PreparedAtomic, owner BoxID, geom LayoutGeometryTable) error {
	if len(placements) != len(items) {
		return fmt.Errorf("expected %d atomic placements, got %d", len(items), len(placements))
	}
	pending := make(map[BoxID]LayoutGeometry, len(items))
	for _, p := range placements {
		if _, ok := items[p.BoxID]; !ok {
			return fmt.Errorf("unprepared atomic box %d", p.BoxID)
		}
		if _, ok := pending[p.BoxID]; ok {
			return fmt.Errorf("duplicate placement for atomic box %d", p.BoxID)
		}
		if !finite(p.X) || !finite(p.Y) {
			return fmt.Errorf("non-finite placement for atomic box %d", p.BoxID)
		}
		g := geom[p.BoxID]
		g.Content.X += p.X - g.Frame.X
		g.Content.Y += p.Y - g.Frame.Y
		g.Frame.X, g.Frame.Y, g.RelativeTo = p.X, p.Y, owner
		if !finite(g.Content.X) || !finite(g.Content.Y) {
			return fmt.Errorf("atomic box %d: coordinate overflow", p.BoxID)
		}
		pending[p.BoxID] = g
	}
	// Validate the entire answer before changing even invocation-local geometry.
	for id, g := range pending {
		geom[id] = g
	}
	return nil
}
