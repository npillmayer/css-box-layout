package glyphing

import "github.com/npillmayer/css-box-layout/layout"

type BrokenLine struct {
	Baseline float32
	Segs     []BrokenSeg
	Width    float32
	Ascent   float32
	Descent  float32
}

type BrokenSeg struct {
	SourceID layout.NodeID // which RunNode leaf/span this segment belongs to (see note below)

	Kind SegKind
	// When Kind==SegGlyphSlice:
	Slice GlyphSlice

	// When Kind==SegSynthetic (e.g., hyphen):
	Synth SyntheticGlyphs
}

type SegKind uint8

const (
	SegGlyphSlice SegKind = iota
	SegSynthetic
)

type LineBox struct {
	Frame    layout.Rect
	Baseline float32
	Frags    []GlyphFragment
}

type GlyphFragment struct {
	SourceID layout.NodeID
	Frame    layout.Rect // relative to block content box

	// Content:
	Kind  FragKind
	Slice GlyphSlice
	Synth SyntheticGlyphs
}

type FragKind uint8

const (
	FragGlyphSlice FragKind = iota
	FragGlyphSynthetic
)

type LayoutResult struct {
	Root *layout.LayoutNode

	Inline struct {
		LinesByBlock map[layout.NodeID][]LineBox
		GlyphsByLeaf map[layout.NodeID]GlyphBuffer // shaped buffers produced during layout
	}
}
