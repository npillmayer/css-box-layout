package layout

func isInlineOnlyBlockContainer(n *LayoutNode) bool {
	if n == nil || !IsBlockContainer(n.Box) {
		return false
	}
	if len(n.Children) != 1 {
		return false
	}
	return n.Children[0] != nil && n.Children[0].Box == BoxAnonymousInline
}
