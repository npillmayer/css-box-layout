package glyphing

import "github.com/npillmayer/css-box-layout/text"

type GlyphID = uint32

type Glyph struct {
	ID      GlyphID
	Advance float32 // advance in layout units (px or font units scaled)
	OffsetX float32 // optional
	OffsetY float32 // optional

	// Optional but strongly recommended even now:
	// maps glyph back to a byte offset (or cluster start) in the shaped text range.
	Cluster text.TextPos
}

type GlyphBuffer struct {
	// The source text slice this buffer was shaped from.
	Text text.TextRef

	// Glyph sequence in logical order for LTR.
	Glyphs []Glyph

	// Metrics needed for line layout:
	Ascent  float32
	Descent float32
	// Optionally: LineGap, etc.
}

type GlyphSlice struct {
	BufferOwner NodeID // which leaf (typically BoxText) produced the GlyphBuffer
	From, To    int    // indices into GlyphBuffer.Glyphs: [From,To)
	// Optional: also report the subrange of text covered by this slice
	TextRange text.TextRange
}

type SyntheticGlyphs struct {
	Glyphs []Glyph
	// Optional: attach semantic cause for debugging:
	Reason SyntheticReason
}

type SyntheticReason uint8

const (
	SynthHyphen SyntheticReason = iota
	SynthCollapsedWhitespace
)
