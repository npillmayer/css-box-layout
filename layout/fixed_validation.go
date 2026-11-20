package layout

import "fmt"

// Validate the entire root before intrinsic callbacks. Shared/duplicate nodes
// would corrupt the per-BoxID intrinsic cache; nested fixed roots must have been
// diverted by construction. This does not expand the descendant layout profile.
func validateFixedSizingTree(root *LayoutNode, fallbackFont float32) error {
	if root == nil || root.Box != BoxBlock || root.Replaced != nil || styleOrDefault(root.Style).Position != PositionFixed {
		return fmt.Errorf("fixed sizing requires a non-replaced fixed block root")
	}
	seen := map[BoxID]bool{}
	var visit func(*LayoutNode, float32) error
	visit = func(n *LayoutNode, font float32) error {
		if n == nil {
			return fmt.Errorf("nil fixed subtree node")
		}
		if n.BoxID == 0 || seen[n.BoxID] {
			return fmt.Errorf("box %d: zero or duplicate fixed subtree identity", n.BoxID)
		}
		seen[n.BoxID] = true
		s := styleOrDefault(n.Style)
		if err := validateComputedStyle(s); err != nil {
			return fmt.Errorf("box %d: %w", n.BoxID, err)
		}
		if n != root && s.Position != PositionStatic {
			return fmt.Errorf("box %d: fixed descendant must be diverted", n.BoxID)
		}
		if s.FontSizePx > 0 {
			font = s.FontSizePx
		}
		if font == 0 && fixedStyleUsesEm(s) {
			return fmt.Errorf("box %d: em length requires a positive font size", n.BoxID)
		}
		switch n.Box {
		case BoxBlock, BoxAnonymousBlock, BoxInlineBlock:
			if n.Replaced == nil {
				if len(n.Children) == 0 {
					return fmt.Errorf("box %d: missing normalized inline group", n.BoxID)
				}
				if !isInlineOnlyBlockContainer(n) {
					for _, c := range n.Children {
						if c == nil || !IsBlockLevel(c.Box) {
							return fmt.Errorf("box %d: mixed or invalid block children", n.BoxID)
						}
					}
				}
			}
		case BoxInline, BoxAnonymousInline:
			for _, c := range n.Children {
				if c == nil || (c.Box != BoxInline && c.Box != BoxText && c.Box != BoxInlineBlock) {
					return fmt.Errorf("box %d: invalid inline children", n.BoxID)
				}
			}
		case BoxText:
			if len(n.Children) != 0 || n.Text.Range.End <= n.Text.Range.Start {
				return fmt.Errorf("box %d: invalid text leaf", n.BoxID)
			}
		default:
			return fmt.Errorf("box %d: unsupported box kind", n.BoxID)
		}
		for _, c := range n.Children {
			if err := visit(c, font); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(root, fallbackFont); err != nil {
		return err
	}
	return validateAtomicWidths(root)
}

func fixedStyleUsesEm(s ComputedStyle) bool {
	if s.Width.Kind == LenEm {
		return true
	}
	for _, e := range []EdgeLengths{s.Margin, s.Padding, s.Border} {
		for _, l := range []Length{e.Top, e.Right, e.Bottom, e.Left} {
			if l.Kind == LenEm {
				return true
			}
		}
	}
	if s.Fixed != nil {
		f := s.Fixed
		for _, l := range []Length{f.Insets.Top, f.Insets.Right, f.Insets.Bottom, f.Insets.Left, f.Height} {
			if l.Kind == LenEm {
				return true
			}
		}
	}
	return false
}
