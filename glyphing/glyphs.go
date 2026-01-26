package glyphing

type GlyphID = uint32

type Glyph struct {
	ID      GlyphID
	Advance float32 // advance in layout units (px or font units scaled)
	OffsetX float32 // optional
	OffsetY float32 // optional

	// Optional but strongly recommended even now:
	// maps glyph back to a byte offset (or cluster start) in the shaped text range.
	Cluster TextPos
}

type GlyphBuffer struct {
	// The source text slice this buffer was shaped from.
	Text TextRef

	// Glyph sequence in logical order for LTR.
	Glyphs []Glyph

	// Metrics needed for line layout:
	Ascent  float32
	Descent float32
	// Optionally: LineGap, etc.
}
