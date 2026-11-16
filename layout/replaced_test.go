package layout

import (
	"golang.org/x/net/html"
	"math"
	"testing"
)

func TestExplicitReplacedLeaf(t *testing.T) {
	for _, height := range []float32{0, 20} {
		style := DefaultComputedStyle()
		style.Width = Length{Kind: LenPx, Value: 30}
		style.Padding = EdgeLengths{Left: Length{Value: 2}, Right: Length{Value: 2}}
		payload := &ReplacedContent{Height: height}
		root, err := BuildLayoutTree(&RenderNode{ID: 1, HTML: &html.Node{Type: html.ElementNode, Data: "div"}, Styles: map[string]string{"display": "block"}, ChildrenNodes: []*RenderNode{{ID: 2, HTML: &html.Node{Type: html.ElementNode, Data: "img"}, Style: &style, Replaced: payload}}}, BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		leaf := root.Children[0].Children[0]
		payload.Height = 999
		if len(leaf.Children) != 0 || leaf.Replaced.Height != height {
			t.Fatal("leaf acquired children or aliased input")
		}
		used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 100}})
		if err != nil {
			t.Fatal(err)
		}
		geom := LayoutGeometryTable{}
		if _, err := layoutBlockContainer(leaf, used, geom, nil, nil, false); err != nil {
			t.Fatal(err)
		}
		if geom[leaf.BoxID].Frame.W != 34 || geom[leaf.BoxID].Frame.H != height {
			t.Fatal(geom)
		}
		pass := intrinsicPass{}
		bounds, _, err := pass.border(leaf, ResolveContext{})
		if err != nil || bounds.MinContent != 34 || bounds.MaxContent != 34 {
			t.Fatal(bounds, err)
		}
	}
}
func TestInvalidReplacedLeaf(t *testing.T) {
	for _, h := range []float32{-1, float32(math.Inf(1)), float32(math.NaN())} {
		s := DefaultComputedStyle()
		s.Width = Length{Value: 20}
		_, err := BuildLayoutTree(&RenderNode{HTML: &html.Node{Type: html.ElementNode}, Styles: map[string]string{"display": "block"}, ChildrenNodes: []*RenderNode{{HTML: &html.Node{Type: html.ElementNode}, Style: &s, Replaced: &ReplacedContent{Height: h}}}}, BuildOptions{})
		if err == nil {
			t.Fatal("accepted invalid height", h)
		}
	}
}
