package layout

import "golang.org/x/net/html"

// RenderNode is a minimal render tree node for BuildLayoutTree.
// This is a stub for now and will be replaced by the real CSSDOM adapter.
type RenderNode struct {
	ID            NodeID
	HTML          *html.Node
	Styles        map[string]string
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
