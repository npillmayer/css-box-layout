package layout

import (
	"fmt"
	"math"
)

// buildStyle snapshots the adapter's typed values before structural normalization.
func buildStyle(r *RenderNode, inheritedFont float32) (*ComputedStyle, error) {
	for prop := range r.Styles {
		switch prop {
		case "width", "margin", "margin-top", "margin-right", "margin-bottom", "margin-left",
			"padding", "padding-top", "padding-right", "padding-bottom", "padding-left",
			"border", "border-width", "border-top-width", "border-right-width", "border-bottom-width", "border-left-width", "font-size":
			return nil, fmt.Errorf("node %d: provide %s through typed RenderNode.Style", r.ID, prop)
		}
	}
	style := styleOrDefault(r.Style)
	if !finite(style.FontSizePx) || style.FontSizePx < 0 {
		return nil, fmt.Errorf("node %d: invalid computed font size", r.ID)
	}
	if style.FontSizePx == 0 {
		style.FontSizePx = inheritedFont
	}
	check := func(name string, length Length, auto, negative bool) error {
		if length.Kind > LenAuto || !finite(length.Value) || (!auto && length.Kind == LenAuto) || (!negative && length.Kind != LenAuto && length.Value < 0) {
			return fmt.Errorf("node %d: invalid computed %s", r.ID, name)
		}
		return nil
	}
	if err := check("width", style.Width, true, false); err != nil {
		return nil, err
	}
	for _, group := range []struct {
		name           string
		edges          EdgeLengths
		auto, negative bool
	}{
		{"margin", style.Margin, true, true}, {"padding", style.Padding, false, false}, {"border", style.Border, false, false},
	} {
		for _, side := range []struct {
			name   string
			length Length
		}{
			{"top", group.edges.Top}, {"right", group.edges.Right}, {"bottom", group.edges.Bottom}, {"left", group.edges.Left},
		} {
			if err := check(group.name+"-"+side.name, side.length, group.auto, group.negative); err != nil {
				return nil, err
			}
		}
	}
	return &style, nil
}

func finite(v float32) bool { return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) }

// Anonymous wrappers inherit font size, but not source box widths or edges.
func setAnonymousStyles(children []*LayoutNode, fontSize float32) {
	for _, child := range children {
		if child.Box != BoxAnonymousBlock && child.Box != BoxAnonymousInline {
			continue
		}
		style := defaultComputedStyle()
		style.FontSizePx = fontSize
		child.Style = &style
		setAnonymousStyles(child.Children, fontSize)
	}
}
