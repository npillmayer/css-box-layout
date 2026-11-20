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
	// Pass trees.Fixed and a viewport to FixedLayout independently.
	// Output:
	// flow root: 1
	// fixed roots: 1
	// fixed source: 2
}

// This example adapter handles the empty inline groups in the example. A real
// adapter retains its prepared paragraphs in the same invocation-local manner.
type emptyFixedExample struct {
	paragraphs map[layout.BoxID]layout.InlineResult
}

func (a emptyFixedExample) LayoutInline(req layout.InlineRequest) (layout.InlineResult, error) {
	r := layout.InlineResult{AllowsMarginCollapse: true}
	a.paragraphs[req.Group.BoxID] = r
	return r, nil
}

func ExampleFixedLayout() {
	block := layout.Display{Kind: layout.DisplayBox, Outer: layout.OuterBlock, Inner: layout.InnerFlow}
	style := layout.DefaultComputedStyle()
	style.Position = layout.PositionFixed
	style.Width = layout.Length{Kind: layout.LenPx, Value: 80}
	fixed := layout.DefaultFixedStyle()
	fixed.Insets.Top = layout.Length{Kind: layout.LenPx, Value: 10}
	fixed.Insets.Right = layout.Length{Kind: layout.LenPx, Value: 20}
	fixed.Height = layout.Length{Kind: layout.LenPx, Value: 30}
	style.Fixed = &fixed
	render := &layout.RenderNode{
		ID: 1, HTML: &html.Node{Type: html.ElementNode}, Display: block,
		ChildrenNodes: []*layout.RenderNode{{
			ID: 2, HTML: &html.Node{Type: html.ElementNode}, Display: block, Style: &style,
		}},
	}
	trees, err := layout.BuildLayoutTrees(render, layout.BuildOptions{})
	if err != nil {
		panic(err)
	}
	viewport := layout.Rect{X: 100, Y: 200, W: 300, H: 200}
	adapter := emptyFixedExample{paragraphs: make(map[layout.BoxID]layout.InlineResult)}
	result, err := layout.FixedLayout(trees.Fixed, viewport, adapter, nil, layout.FixedLayoutOptions{})
	if err != nil {
		// Discard adapter.paragraphs along with this failed invocation.
		panic(err)
	}
	g := result.Geometry[result.Roots[0].BoxID]
	fmt.Println("viewport-local frame:", g.Frame.X, g.Frame.Y, g.Frame.W, g.Frame.H)
	fmt.Println("external origin:", result.Viewport.X+g.Frame.X, result.Viewport.Y+g.Frame.Y)
	fmt.Println("retained groups:", len(adapter.paragraphs))

	// Normal flow can run independently, with its own adapter state.
	used, err := layout.ResolveUsedValues(trees.Flow, layout.ResolveContext{ContainingBlock: viewport})
	if err != nil {
		panic(err)
	}
	flowAdapter := emptyFixedExample{paragraphs: make(map[layout.BoxID]layout.InlineResult)}
	flow, err := layout.FlowLayout(trees.Flow, used, flowAdapter, nil, layout.LayoutContext{}, layout.LayoutOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Println("normal-flow height:", flow.Geometry[flow.Root.BoxID].Frame.H)
	// Output:
	// viewport-local frame: 200 10 80 30
	// external origin: 300 210
	// retained groups: 1
	// normal-flow height: 0
}
