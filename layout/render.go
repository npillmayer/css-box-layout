package layout

import (
	"github.com/npillmayer/css-box-layout/text"
	"golang.org/x/net/html"
)

// ReplacedContent is an explicitly sized, non-text inline leaf. Width comes
// from Style.Width (fixed px); Height is content height in CSS pixels.
// Payload ownership stays with the adapter/renderer. Zero dimensions are valid.
type ReplacedContent struct{ Height float32 }

// RenderNode is the adapter's render-tree projection for both layout builders.
// HTML must identify an element or text node; other DOM nodes are filtered by
// the adapter. Display must be explicit for elements and zero for text nodes.
type RenderNode struct {
	Replaced *ReplacedContent
	ID       NodeID
	HTML     *html.Node
	Display  Display
	Styles   map[string]string // adapter metadata; display and box-value strings are rejected
	// Style is a complete typed box style. Nil selects defaults and inherits
	// the source parent's white-space and font size.
	// FontSizePx == 0 inherits the source parent's size or ResolveContext fallback.
	Style *ComputedStyle
	// Text identifies source bytes for a text node. The adapter owns source lookup.
	// ReadText in BuildOptions supplies that range when HTML.Data does not.
	Text          *text.TextRef
	ChildrenNodes []*RenderNode
}

func (r *RenderNode) Children() []*RenderNode {
	if r == nil {
		return nil
	}
	return r.ChildrenNodes
}

func (r *RenderNode) HTMLNode() *html.Node {
	if r == nil {
		return nil
	}
	return r.HTML
}
