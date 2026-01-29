package layout

type LayoutGeometry struct {
	Frame   Rect
	Content Rect
}

type LayoutGeometryTable map[BoxID]LayoutGeometry

type LinesByBlock map[BoxID][]LineBox

type LayoutPolicy struct{}

type LayoutContext struct {
	ContainingBlock Rect
	Policy          LayoutPolicy
}
