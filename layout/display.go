package layout

import (
	"errors"
	"fmt"

	"golang.org/x/net/html"
)

// DisplayKind distinguishes generated boxes from subtree suppression.
type DisplayKind uint8

const (
	DisplayInvalid DisplayKind = iota // Zero is not an implicit block or inline.
	DisplayBox                        // An explicit outer/inner pair.
	DisplayNone                       // Suppress this element and its descendants.
)

// DisplayOuter describes participation in the surrounding flow.
type DisplayOuter uint8

const (
	OuterNone DisplayOuter = iota // Not applicable (DisplayNone only).
	OuterBlock
	OuterInline
)

// DisplayInner describes the contents' formatting model. InnerFlow does not
// decide whether normalization produces block children or an inline group.
type DisplayInner uint8

const (
	InnerNone DisplayInner = iota // Not applicable (DisplayNone only).
	InnerFlow
	InnerFlowRoot
)

// Display is the core's comparable, typed display input. Adapters resolve CSS
// syntax, defaults and cascade before projecting into this subset. It has no
// dependency on a DOM/CSS implementation and does not parse CSS strings.
//
// DisplayBox requires both axes; DisplayNone requires neither. The zero value
// is invalid for elements. Text nodes leave Display zero: they have no display
// property and always contribute inline text to their containing flow.
//
// Block flow-root is an ordinary block-level container with an independent
// block formatting context. Inline flow-root is atomic inline-block.
type Display struct {
	Kind  DisplayKind
	Outer DisplayOuter
	Inner DisplayInner
}

var (
	// ErrInvalidDisplay identifies missing, malformed or legacy string input.
	ErrInvalidDisplay = errors.New("invalid display input")
	// ErrUnsupportedDisplay identifies a display outside layout's supported profile.
	ErrUnsupportedDisplay = errors.New("unsupported display")
	// ErrInvalidRoot identifies a root that is not a non-replaced block element.
	ErrInvalidRoot = errors.New("invalid layout root")
)

// Validate checks the value's structure. All four outer/inner pairs in this
// subset are supported for non-replaced elements; root and replaced-content
// restrictions are checked separately during construction.
func (d Display) Validate() error {
	switch d.Kind {
	case DisplayNone:
		if d.Outer == OuterNone && d.Inner == InnerNone {
			return nil
		}
	case DisplayBox:
		if (d.Outer == OuterBlock || d.Outer == OuterInline) &&
			(d.Inner == InnerFlow || d.Inner == InnerFlowRoot) {
			return nil
		}
	}
	return fmt.Errorf("%w: kind=%d outer=%d inner=%d", ErrInvalidDisplay, d.Kind, d.Outer, d.Inner)
}

// String describes the normalized pair (or suppression) for diagnostics.
func (d Display) String() string {
	if err := d.Validate(); err != nil {
		return fmt.Sprintf("Display{Kind:%d, Outer:%d, Inner:%d}", d.Kind, d.Outer, d.Inner)
	}
	if d.Kind == DisplayNone {
		return "none"
	}
	outer, inner := "block", "flow"
	if d.Outer == OuterInline {
		outer = "inline"
	}
	if d.Inner == InnerFlowRoot {
		inner = "flow-root"
	}
	return outer + " " + inner
}

// validateRenderDisplay runs before style construction or subtree suppression.
// Descendants of a suppressed element are deliberately not inspected.
func validateRenderDisplay(r *RenderNode) error {
	if _, legacy := r.Styles["display"]; legacy {
		return fmt.Errorf("node %d: %w: provide display through typed RenderNode.Display, not Styles[\"display\"]", r.ID, ErrInvalidDisplay)
	}
	if isTextNode(r.HTMLNode()) {
		if r.Display != (Display{}) {
			return fmt.Errorf("node %d: %w: text nodes must leave Display zero", r.ID, ErrInvalidDisplay)
		}
		return nil
	}
	if r.HTMLNode() == nil || r.HTMLNode().Type != html.ElementNode {
		return fmt.Errorf("node %d: render input requires an HTML element or text node", r.ID)
	}
	if err := r.Display.Validate(); err != nil {
		return fmt.Errorf("node %d: %w", r.ID, err)
	}
	return nil
}
