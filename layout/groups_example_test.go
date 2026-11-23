package layout_test

import (
	"fmt"

	"github.com/npillmayer/css-box-layout/layout"
)

func ExampleInlineGroups() {
	// A small already-normalized structural tree. Enumeration needs neither
	// source text nor styles, fonts or geometry to identify its groups.
	interior := &layout.LayoutNode{BoxID: 4, Box: layout.BoxAnonymousInline, FC: layout.FCInline}
	atomic := &layout.LayoutNode{BoxID: 3, Box: layout.BoxInlineBlock, FC: layout.FCBlock,
		Children: []*layout.LayoutNode{interior}}
	outer := &layout.LayoutNode{BoxID: 2, Box: layout.BoxAnonymousInline, FC: layout.FCInline,
		Children: []*layout.LayoutNode{atomic}}
	root := &layout.LayoutNode{BoxID: 1, Box: layout.BoxBlock, FC: layout.FCBlock,
		Children: []*layout.LayoutNode{outer}}
	for g := range layout.InlineGroups(root) {
		var parent layout.BoxID
		if g.ParentAtomic != nil {
			parent = g.ParentAtomic.BoxID
		}
		fmt.Printf("group=%d owner=%d atomics=%d parent-atomic=%d\n",
			g.Group.BoxID, g.Owner.BoxID, len(g.Atomics), parent)
	}
	// Output:
	// group=2 owner=1 atomics=1 parent-atomic=0
	// group=4 owner=3 atomics=0 parent-atomic=3
}
