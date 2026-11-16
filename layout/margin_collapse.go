package layout

// marginStrut retains both extrema of an adjoining group. Reducing a group to
// its scalar value before merging another margin would lose negative margins.
// Values are finite resolved margins; the zero value is the merge identity.
// All operations return copies and leave their operands unchanged.
type marginStrut struct {
	positive float32
	negative float32
}

func strutFromMargin(m float32) marginStrut {
	return marginStrut{positive: max(0, m), negative: min(0, m)}
}

func (s marginStrut) merge(other marginStrut) marginStrut {
	return marginStrut{
		positive: max(s.positive, other.positive),
		negative: min(s.negative, other.negative),
	}
}

func (s marginStrut) value() float32 { return s.positive + s.negative }

// blockMarginSummary is an invocation-local value returned by recursive layout.
// Through means Leading and Trailing describe the same adjoining group, not
// two independent gaps. Boundary boxes retain only their own outside margins.
// Dimensions describe the completed geometry produced by this invocation.
type blockMarginSummary struct {
	collapseEligibility
	Leading, Trailing marginStrut
	// BeforeBottom is the leading group with a hypothetical bottom border.
	// A through box uses it for its own origin, excluding its bottom margin.
	BeforeBottom              marginStrut
	BorderWidth, BorderHeight float32
}

// childMarginSummary retains only the groups touching the content's top/bottom.
// Internal sibling gaps cannot escape past a non-through child. Start an empty
// sequence with Through=true; append is a pure, source-order fold.
type childMarginSummary struct {
	Leading, Trailing marginStrut
	Through           bool
}

func (s childMarginSummary) append(child blockMarginSummary) childMarginSummary {
	if s.Through {
		s.Leading = s.Leading.merge(child.Leading)
	}
	if child.Through {
		s.Trailing = s.Trailing.merge(child.Trailing)
	} else {
		s.Trailing = child.Trailing
	}
	s.Through = s.Through && child.Through
	return s
}

func summarizeBlockMargins(u UsedValues, eligibility collapseEligibility, children childMarginSummary, frame Rect) blockMarginSummary {
	s := blockMarginSummary{
		collapseEligibility: eligibility,
		Leading:             strutFromMargin(u.Margin.Top),
		Trailing:            strutFromMargin(u.Margin.Bottom),
		BorderWidth:         frame.W,
		BorderHeight:        frame.H,
	}
	if eligibility.TopWithChildren {
		s.Leading = s.Leading.merge(children.Leading)
	}
	if eligibility.BottomWithChildren {
		s.Trailing = s.Trailing.merge(children.Trailing)
	}
	s.BeforeBottom = s.Leading
	if eligibility.Through {
		s.Leading = s.Leading.merge(s.Trailing)
		s.Trailing = s.Leading
	}
	return s
}

// collapseEligibility describes boundaries, not collapsed margin values. The
// current profile assumes auto height, zero min-height and normal block flow.
// TopWithChildren/BottomWithChildren permit a child-edge connection; they do not
// assert that a block child exists. Through includes the content's eligibility.
type collapseEligibility struct {
	TopWithChildren    bool
	BottomWithChildren bool
	Through            bool
}

// FCBlock describes internal layout, not whether a box establishes a new BFC.
// The supplied layout root is an explicit containment boundary even when a
// caller lays out a subtree. Inline-block interiors are independent BFCs.
func isMarginCollapseBoundary(node *LayoutNode, isRoot bool) bool {
	return isRoot || node == nil || !IsBlockLevel(node.Box) || node.Replaced != nil
}

func blockCollapseEligibility(node *LayoutNode, u UsedValues, isRoot, contentThrough bool) collapseEligibility {
	if isMarginCollapseBoundary(node, isRoot) {
		return collapseEligibility{}
	}
	top := u.Border.Top == 0 && u.Padding.Top == 0
	bottom := u.Border.Bottom == 0 && u.Padding.Bottom == 0
	return collapseEligibility{
		TopWithChildren:    top,
		BottomWithChildren: bottom,
		Through:            top && bottom && contentThrough,
	}
}
