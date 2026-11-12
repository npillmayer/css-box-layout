package layout

import (
	"github.com/npillmayer/css-box-layout/text"
	"golang.org/x/net/html"
)

// RenderNode is a minimal render tree node for BuildLayoutTree.
// This is a stub for now and will be replaced by the real CSSDOM adapter.
type RenderNode struct {
	ID     NodeID
	HTML   *html.Node
	Styles map[string]string // display and adapter metadata; not parsed as lengths
	// Style is a complete typed box style. Nil selects defaults.
	// FontSizePx == 0 inherits the source parent's size or ResolveContext fallback.
	Style *ComputedStyle
	// Text identifies source bytes for a text node. The adapter owns source lookup.
	// A supplied range is authoritative, even when HTML.Data is empty.
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

func (r *RenderNode) ComputedStyle(prop string) string {
	if r == nil || r.Styles == nil {
		return ""
	}
	return r.Styles[prop]
}
