package layout

import "fmt"

// Validate before callbacks so missing/invalid inputs cannot masquerade as zero
// width or leave a partially populated adapter result table for input errors.
func validateUsedValues(node *LayoutNode, used UsedValuesTable) error {
	if node == nil {
		return nil
	}
	u, ok := used[node.BoxID]
	if !ok {
		return fmt.Errorf("box %d: missing used values", node.BoxID)
	}
	if !finite(u.ContentWidth) || u.ContentWidth < 0 {
		return fmt.Errorf("box %d: invalid content width", node.BoxID)
	}
	for _, g := range []struct {
		name     string
		e        Edges
		negative bool
	}{
		{"margin", u.Margin, true}, {"padding", u.Padding, false}, {"border", u.Border, false},
	} {
		for _, v := range []float32{g.e.Top, g.e.Right, g.e.Bottom, g.e.Left} {
			if !finite(v) || (!g.negative && v < 0) {
				return fmt.Errorf("box %d: invalid %s", node.BoxID, g.name)
			}
		}
	}
	for _, c := range node.Children {
		if err := validateUsedValues(c, used); err != nil {
			return err
		}
	}
	return nil
}
