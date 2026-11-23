package layout

import (
	"fmt"
	"maps"
	"math"
	"slices"
)

// buildStyle snapshots the adapter's typed values before structural normalization.
func buildStyle(r *RenderNode, inheritedFont float32, inheritedWhitespace WhiteSpace) (*ComputedStyle, error) {
	for _, prop := range slices.Sorted(maps.Keys(r.Styles)) {
		switch prop {
		case "position", "top", "right", "bottom", "left", "inset", "height",
			"inset-block", "inset-inline", "inset-block-start", "inset-block-end", "inset-inline-start", "inset-inline-end",
			"width", "margin", "margin-top", "margin-right", "margin-bottom", "margin-left",
			"padding", "padding-top", "padding-right", "padding-bottom", "padding-left",
			"border", "border-width", "border-top-width", "border-right-width", "border-bottom-width", "border-left-width", "font-size", "white-space":
			return nil, buildError(r.ID, fmt.Errorf("provide %s through typed RenderNode.Style", prop))
		}
	}
	style := styleOrDefault(r.Style)
	if r.Style == nil {
		style.WhiteSpace = inheritedWhitespace
	}
	if err := validateComputedStyle(style); err != nil {
		return nil, buildError(r.ID, err)
	}
	if isTextNode(r.HTMLNode()) && style.Position != PositionStatic {
		return nil, buildError(r.ID, fmt.Errorf("%w: text nodes cannot be positioned", ErrInvalidPosition))
	}
	if style.Fixed != nil {
		fixed := *style.Fixed
		style.Fixed = &fixed
	}
	if style.FontSizePx == 0 {
		style.FontSizePx = inheritedFont
	}
	return &style, nil
}

// Shared by construction and the fixed resolver, including hand-built inputs.
func validateComputedStyle(style ComputedStyle) error {
	if err := validatePosition(style); err != nil {
		return err
	}
	if style.WhiteSpace > WhiteSpaceBreakSpaces {
		return fmt.Errorf("invalid computed white-space")
	}
	if !finite(style.FontSizePx) || style.FontSizePx < 0 {
		return fmt.Errorf("invalid computed font size")
	}
	check := func(name string, length Length, auto, negative bool) error {
		if length.Kind > LenAuto || !finite(length.Value) || (!auto && length.Kind == LenAuto) || (!negative && length.Kind != LenAuto && length.Value < 0) {
			return fmt.Errorf("invalid computed %s", name)
		}
		return nil
	}
	if err := check("width", style.Width, true, false); err != nil {
		return err
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
				return err
			}
		}
	}
	return nil
}

func finite(v float32) bool { return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0) }

// Anonymous wrappers inherit font size and white-space, not box widths or edges.
func setAnonymousStyles(children []*LayoutNode, fontSize float32, whitespace WhiteSpace) {
	for _, child := range children {
		if child.Box != BoxAnonymousBlock && child.Box != BoxAnonymousInline {
			continue
		}
		style := defaultComputedStyle()
		style.FontSizePx = fontSize
		style.WhiteSpace = whitespace
		child.Style = &style
		setAnonymousStyles(child.Children, fontSize, whitespace)
	}
}
