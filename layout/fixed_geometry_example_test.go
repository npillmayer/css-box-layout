package layout_test

import (
	"fmt"

	"github.com/npillmayer/css-box-layout/layout"
)

func ExampleResolveFixedGeometry() {
	px := func(v float32) layout.Length { return layout.Length{Kind: layout.LenPx, Value: v} }
	s := layout.DefaultComputedStyle()
	s.Position = layout.PositionFixed
	s.Width = px(80)
	f := layout.DefaultFixedStyle()
	f.Height, f.Insets.Right, f.Insets.Top = px(30), px(20), px(10)
	s.Fixed = &f
	s.Padding = layout.EdgeLengths{Top: px(3), Right: px(3), Bottom: px(3), Left: px(3)}
	s.Border = layout.EdgeLengths{Top: px(2), Right: px(2), Bottom: px(2), Left: px(2)}
	g, err := layout.ResolveFixedGeometry(s, layout.Rect{W: 300, H: 200}, layout.FixedLayoutOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Println(g.Frame)
	fmt.Println(g.Content)
	// Output:
	// {190 10 90 40}
	// {195 15 80 30}
}
