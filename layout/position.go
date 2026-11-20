package layout

import (
	"errors"
	"fmt"
)

// PositionKind selects participation in normal flow or the fixed-box branch.
// Position is not inherited. Other CSS positioning schemes are not supported.
type PositionKind uint8

const (
	PositionStatic PositionKind = iota
	PositionFixed
)

var (
	ErrInvalidPosition     = errors.New("invalid positioning input")
	ErrUnsupportedPosition = errors.New("unsupported positioning")
	// ErrFixedLayoutRequired prevents normal-flow APIs from dropping or laying
	// out fixed boxes. Fixed sizing and placement are not implemented yet.
	ErrFixedLayoutRequired = errors.New("fixed boxes require separate layout")
)

// FixedStyle contains constraints for a fixed root. Width and box edges remain
// in ComputedStyle. A nil ComputedStyle.Fixed means auto insets and auto height.
// A non-nil zero FixedStyle means explicit zero insets and height; use
// DefaultFixedStyle to start with auto values. FixedStyle is not inherited.
// These values are validated and copied during construction, not resolved.
type FixedStyle struct {
	Insets EdgeLengths
	Height Length
}

// DefaultFixedStyle returns auto insets and auto content height.
func DefaultFixedStyle() FixedStyle {
	auto := Length{Kind: LenAuto}
	return FixedStyle{Insets: EdgeLengths{auto, auto, auto, auto}, Height: auto}
}

func validatePosition(style ComputedStyle) error {
	switch style.Position {
	case PositionStatic:
		if style.Fixed != nil {
			return fmt.Errorf("%w: Fixed requires PositionFixed", ErrInvalidPosition)
		}
	case PositionFixed:
	default:
		return fmt.Errorf("%w: position=%d", ErrUnsupportedPosition, style.Position)
	}
	if style.Fixed == nil {
		return nil
	}
	for _, inset := range []Length{style.Fixed.Insets.Top, style.Fixed.Insets.Right, style.Fixed.Insets.Bottom, style.Fixed.Insets.Left} {
		if inset.Kind > LenAuto || !finite(inset.Value) {
			return fmt.Errorf("%w: invalid inset", ErrInvalidPosition)
		}
	}
	h := style.Fixed.Height
	if h.Kind > LenAuto || !finite(h.Value) || (h.Kind != LenAuto && h.Value < 0) {
		return fmt.Errorf("%w: invalid fixed height", ErrInvalidPosition)
	}
	return nil
}

// Protect both public normal-flow entry points, including hand-built input.
// Run this before resolution, intrinsic measurement or final callbacks.
func validateNormalFlowPositions(n *LayoutNode) error {
	if n == nil {
		return nil
	}
	s := styleOrDefault(n.Style)
	if err := validatePosition(s); err != nil {
		return fmt.Errorf("box %d: %w", n.BoxID, err)
	}
	if s.Position == PositionFixed {
		return fmt.Errorf("box %d: %w", n.BoxID, ErrFixedLayoutRequired)
	}
	for _, child := range n.Children {
		if err := validateNormalFlowPositions(child); err != nil {
			return err
		}
	}
	return nil
}
