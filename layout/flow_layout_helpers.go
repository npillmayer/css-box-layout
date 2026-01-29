package layout

func isInlineOnlyBlockContainer(n *LayoutNode) bool {
	if n == nil || !IsBlockLevel(n.Box) {
		return false
	}
	if len(n.Children) != 1 {
		return false
	}
	return n.Children[0] != nil && n.Children[0].Box == BoxAnonymousInline
}

func shouldStoreLines(n *LayoutNode) bool {
	if n == nil {
		return false
	}
	switch n.Box {
	case BoxBlock, BoxInlineBlock:
		return true
	default:
		return false
	}
}

func lineExtent(lines []LineBox) float32 {
	var max float32
	for _, line := range lines {
		end := line.Frame.Y + line.Frame.H
		if end > max {
			max = end
		}
	}
	return max
}
