package layout

import (
	"github.com/npillmayer/css-box-layout/text"
)

type Edges struct{ Top, Right, Bottom, Left float32 }

// LayoutNode is structural input. Geometry belongs only to LayoutResult.Geometry;
// callers and adapters must treat a published tree and its styles as immutable.
type LayoutNode struct {
	BoxID    BoxID
	NodeID   NodeID
	Box      BoxKind
	FC       FormattingContextKind
	Style    *ComputedStyle
	Children []*LayoutNode
	Text     text.TextRef // For BoxText only (range in base rope)
}

type BoxKind uint8

const (
	BoxBlock BoxKind = iota
	BoxInline
	BoxText
	BoxAnonymousBlock
	BoxAnonymousInline
	BoxInlineBlock // atomic inline, lays out children with block rules
)

// IsBlockContainer describes internal layout, independent of outer participation.
func IsBlockContainer(kind BoxKind) bool {
	switch kind {
	case BoxBlock, BoxAnonymousBlock, BoxInlineBlock:
		return true
	default:
		return false
	}
}

// IsBlockLevel excludes inline-blocks, which participate atomically in inline flow.
func IsBlockLevel(kind BoxKind) bool {
	return kind == BoxBlock || kind == BoxAnonymousBlock
}

type FormattingContextKind uint8

const (
	FCNone FormattingContextKind = iota
	FCBlock
	FCInline
)

type Rect struct{ X, Y, W, H float32 }

type BoxMetrics struct {
	MarginTop, MarginBottom float32
	// later: left/right, padding, border
}

type LengthKind uint8

const (
	LenPx LengthKind = iota
	LenPercent
	LenEm
	LenAuto
)

type Length struct {
	Kind  LengthKind
	Value float32 // px, percent (0..1), em
}
