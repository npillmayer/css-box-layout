package layout

import (
	"fmt"

	"github.com/npillmayer/css-box-layout/text"
)

type Edges struct{ Top, Right, Bottom, Left float32 }

type LayoutNode struct {
	BoxID    BoxID
	NodeID   NodeID
	Box      BoxKind
	FC       FormattingContextKind
	Style    *ComputedStyle
	Children []*LayoutNode
	Text     text.TextRef // For BoxText only (range in base rope)

	// Computed during layout: border/content rects relative to parent content box.
	Frame   Rect // border box (recommended)
	Content Rect // content box
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

func IsBlockLevel(kind BoxKind) bool {
	switch kind {
	case BoxBlock, BoxAnonymousBlock, BoxInlineBlock:
		return true
	default:
		return false
	}
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

type LineBox struct {
	Frame    Rect
	Baseline float32
	// Optional:
	Ascent  float32
	Descent float32
	Payload any
}

type BlockContext struct {
	AvailableWidth float32 // width of parent content box
	// Later: current BFC, floats, etc.
}

type atomicSizer struct {
	inline    InlineLayouter
	intrinsic IntrinsicMeasurer
	res       *LayoutResult
}

func (a atomicSizer) SizeAtomicInline(n *LayoutNode, maxWidth float32) (float32, float32, error) {
	if n.Box != BoxInlineBlock {
		// later: replaced elements etc.
		return 0, 0, fmt.Errorf("unsupported atomic inline kind")
	}

	// Approximate auto width:
	maxContent, err := a.intrinsic.MaxContentWidth(n)
	if err != nil {
		return 0, 0, err
	}

	usedW := min(maxWidth, maxContent)

	// Layout internal contents as a block container with usedW.
	ctx := BlockContext{AvailableWidth: usedW}
	err = layoutBlockContainer(n, ctx, a.inline, a.intrinsic, a.res)
	if err != nil {
		return 0, 0, err
	}

	// After layoutBlockContainer, n.Frame.H is known; width is usedW.
	n.Frame.W = usedW
	n.Content.W = usedW

	return usedW, n.Frame.H, nil
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
