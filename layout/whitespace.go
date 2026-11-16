package layout

import "strings"

// WhiteSpace is the computed CSS white-space value. Adapters resolve inheritance;
// a nil RenderNode.Style inherits its parent's value during construction.
// Core uses it for box generation; inline services must implement or explicitly
// reject unsupported modes when transforming, measuring and laying out text.
type WhiteSpace uint8

const (
	WhiteSpaceNormal WhiteSpace = iota
	WhiteSpaceNowrap
	WhiteSpacePre
	WhiteSpacePreWrap
	WhiteSpacePreLine
	WhiteSpaceBreakSpaces
)

func (w WhiteSpace) collapsesEntirely(s string) bool {
	switch w {
	case WhiteSpaceNormal, WhiteSpaceNowrap:
		return strings.Trim(s, " \t\n\r\f") == ""
	case WhiteSpacePreLine:
		return strings.Trim(s, " \t") == "" // Segment breaks survive.
	default:
		return s == ""
	}
}

// CSS 2.2 9.4.2: empty inline boxes with nonzero edges participate even without
// text. Font/line-height differences alone do not make an empty line exist.
func hasInlineEdges(s *ComputedStyle) bool {
	if s == nil {
		return false
	}
	for _, edges := range []EdgeLengths{s.Margin, s.Padding, s.Border} {
		for _, v := range []Length{edges.Top, edges.Right, edges.Bottom, edges.Left} {
			if v.Kind != LenAuto && v.Value != 0 {
				return true
			}
		}
	}
	return false
}
