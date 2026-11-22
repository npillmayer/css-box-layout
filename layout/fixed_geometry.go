package layout

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidFixedGeometry identifies invalid typed input, unresolved em font
	// context, or numeric overflow in ResolveFixedGeometry. Wrapped validation
	// errors remain available through errors.Is/errors.As.
	ErrInvalidFixedGeometry = errors.New("invalid fixed geometry")
	// ErrFixedContentDependency means an auto width or height has only one
	// definite inset and cannot be resolved without content measurement/layout.
	ErrFixedContentDependency = errors.New("fixed geometry requires content sizing")
)

// ResolveFixedGeometry resolves one non-replaced, content-box fixed box in the
// current LTR profile. It uses the same constraints, axis equations and rectangle
// construction as FixedLayout, without a DOM, layout tree, identities or callbacks.
// Position must be PositionFixed. Start with DefaultComputedStyle and
// DefaultFixedStyle: a zero Length is explicit 0px, while nil Fixed means auto.
//
// Width/height must be definite or stretch between opposing definite insets.
// At least one inset per axis is required; both-auto static-position fallback
// and non-fixed positioning return ErrUnsupportedPosition. Content-dependent
// sizes return ErrFixedContentDependency, never an assumed empty-content size.
// Other invalid input and numeric overflow return ErrInvalidFixedGeometry.
// Every error returns a zero LayoutGeometry, without partial results.
//
// Px, viewport percentages and em with a positive effective font size are
// supported. Computed FontSizePx overrides the options fallback. Insets and
// sizes use their axis's viewport dimension; percentage margins/padding/borders
// use viewport width. Zero dimensions, negative insets/margins, auto margins and
// finite overflow outside the viewport follow FixedLayout's existing rules.
// Viewport coordinates and font sizes must be finite; W/H and fonts nonnegative.
//
// Returned Frame (border box) and Content rectangles are in viewport-local CSS
// pixels, with RelativeTo == 0. Viewport.X/Y are validated but not added; apply
// that external origin once when composing results. Inputs are read-only and
// nothing is retained, so independent calls may share immutable styles safely.
// This API defines no clipping, overlap, painting, or positive-receiver policy.
func ResolveFixedGeometry(style ComputedStyle, viewport Rect, opts FixedLayoutOptions) (LayoutGeometry, error) {
	invalid := func(err error) (LayoutGeometry, error) {
		return LayoutGeometry{}, fmt.Errorf("%w: %w", ErrInvalidFixedGeometry, err)
	}
	if err := validateFixedContext(viewport, opts.FontSizePx); err != nil {
		return invalid(err)
	}
	if style.Position != PositionFixed {
		return LayoutGeometry{}, fmt.Errorf("%w: geometry requires PositionFixed", ErrUnsupportedPosition)
	}
	if err := validateComputedStyle(style); err != nil {
		return invalid(err)
	}
	font := opts.FontSizePx
	if style.FontSizePx > 0 {
		font = style.FontSizePx
	}
	if font == 0 && fixedStyleUsesEm(style) {
		return invalid(fmt.Errorf("em length requires a positive font size"))
	}
	c, err := prepareFixedConstraints(style, viewport, font)
	if err != nil {
		if errors.Is(err, ErrUnsupportedPosition) {
			return LayoutGeometry{}, err
		}
		return invalid(err)
	}
	for _, axis := range []struct {
		name  string
		value fixedAxis
	}{{"width", c.horizontal}, {"height", c.vertical}} {
		if axis.value.needsContentSize() {
			return LayoutGeometry{}, fmt.Errorf("%w: auto %s requires opposing definite insets", ErrFixedContentDependency, axis.name)
		}
	}
	// Neither axis consumes this value after the dependency checks above.
	sizing, err := c.resolve(0)
	if err != nil {
		return invalid(err)
	}
	g, err := fixedRootGeometry(sizing)
	if err != nil {
		return invalid(err)
	}
	return g, nil
}

func fixedRootGeometry(s fixedSizing) (LayoutGeometry, error) {
	u, h, v := s.Used, s.Horizontal, s.Vertical
	x, y := float64(h.Start)+float64(h.MarginStart), float64(v.Start)+float64(v.MarginStart)
	left, top := float64(u.Padding.Left)+float64(u.Border.Left), float64(u.Padding.Top)+float64(u.Border.Top)
	g := LayoutGeometry{
		Frame: Rect{X: float32(x), Y: float32(y),
			W: float32(left + float64(h.Size) + float64(u.Padding.Right) + float64(u.Border.Right)),
			H: float32(top + float64(v.Size) + float64(u.Padding.Bottom) + float64(u.Border.Bottom))},
		Content: Rect{X: float32(x + left), Y: float32(y + top), W: h.Size, H: v.Size},
	}
	for _, value := range []float32{g.Frame.X, g.Frame.Y, g.Frame.W, g.Frame.H, g.Content.X, g.Content.Y} {
		if !finite(value) {
			return LayoutGeometry{}, fmt.Errorf("fixed geometry overflow")
		}
	}
	return g, nil
}
