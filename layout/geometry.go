package layout

// LayoutGeometry uses an explicit content-coordinate parent; zero denotes root.
type LayoutGeometry struct {
	RelativeTo BoxID
	Frame      Rect
	Content    Rect
}

type LayoutGeometryTable map[BoxID]LayoutGeometry

type LayoutPolicy struct{}

type LayoutContext struct {
	ContainingBlock Rect
	Policy          LayoutPolicy
}
