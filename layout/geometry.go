package layout

type LayoutGeometry struct {
	Frame   Rect
	Content Rect
}

type LayoutGeometryTable map[BoxID]LayoutGeometry

type LayoutPolicy struct{}

type LayoutContext struct {
	ContainingBlock Rect
	Policy          LayoutPolicy
}
