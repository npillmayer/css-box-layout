package layout_test

import (
	"fmt"

	"github.com/npillmayer/css-box-layout/layout"
	"golang.org/x/net/html"
)

func ExampleBuildLayoutTrees() {
	block := layout.Display{Kind: layout.DisplayBox, Outer: layout.OuterBlock, Inner: layout.InnerFlow}
	style := layout.DefaultComputedStyle()
	style.Position = layout.PositionFixed
	fixed := layout.DefaultFixedStyle()
	fixed.Insets.Top = layout.Length{Kind: layout.LenPx, Value: 10}
	fixed.Insets.Right = layout.Length{Kind: layout.LenPx, Value: 20}
	style.Fixed = &fixed
	root := &layout.RenderNode{
		ID: 1, HTML: &html.Node{Type: html.ElementNode}, Display: block,
		ChildrenNodes: []*layout.RenderNode{{
			ID: 2, HTML: &html.Node{Type: html.ElementNode}, Display: block, Style: &style,
		}},
	}
	trees, err := layout.BuildLayoutTrees(root, layout.BuildOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Println("flow root:", trees.Flow.NodeID)
	fmt.Println("fixed roots:", len(trees.Fixed))
	fmt.Println("fixed source:", trees.Fixed[0].NodeID)
	// trees.Flow is ready for ResolveUsedValues/FlowLayout.
	// Retain trees.Fixed separately; fixed sizing/placement is not implemented yet.
	// Output:
	// flow root: 1
	// fixed roots: 1
	// fixed source: 2
}
