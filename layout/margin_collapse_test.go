package layout

import (
	"reflect"
	"strings"
	"testing"
)

func TestBlockCollapseEligibility(t *testing.T) {
	all := collapseEligibility{true, true, true}
	for _, tc := range []struct {
		name string
		kind BoxKind
		root bool
		u    UsedValues
		want collapseEligibility
	}{
		{"ordinary FCBlock", BoxBlock, false, UsedValues{}, all},
		{"anonymous block", BoxAnonymousBlock, false, UsedValues{}, all},
		{"supplied root", BoxBlock, true, UsedValues{}, collapseEligibility{}},
		{"inline-block", BoxInlineBlock, false, UsedValues{}, collapseEligibility{}},
		{"inline", BoxInline, false, UsedValues{}, collapseEligibility{}},
		{"top border", BoxBlock, false, UsedValues{Border: Edges{Top: 1}}, collapseEligibility{false, true, false}},
		{"bottom border", BoxBlock, false, UsedValues{Border: Edges{Bottom: 1}}, collapseEligibility{true, false, false}},
		{"top padding", BoxBlock, false, UsedValues{Padding: Edges{Top: 1}}, collapseEligibility{false, true, false}},
		{"bottom padding", BoxBlock, false, UsedValues{Padding: Edges{Bottom: 1}}, collapseEligibility{true, false, false}},
		{"horizontal edges", BoxBlock, false, UsedValues{Padding: Edges{Left: 1}, Border: Edges{Right: 2}}, all},
		{"signed margins", BoxBlock, false, UsedValues{Margin: Edges{Top: -20, Bottom: 30}}, all},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := &LayoutNode{Box: tc.kind, FC: FCBlock}
			if got := blockCollapseEligibility(n, tc.u, tc.root, true); got != tc.want {
				t.Fatalf("eligibility = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestInlineCollapseEligibility(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result InlineResult
		want   bool
	}{
		{"unspecified zero extent", InlineResult{}, false},
		{"explicit empty output", InlineResult{AllowsMarginCollapse: true}, true},
		{"reserved space", InlineResult{Extent: InlineExtent{H: 12}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, err := BuildLayoutTree(newRenderElement(1, "block"), BuildOptions{})
			if err != nil {
				t.Fatal(err)
			}
			u, err := ResolveUsedValues(n, ResolveContext{ContainingBlock: Rect{W: 100}})
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			adapter := resultFunc(func(InlineRequest) (InlineResult, error) {
				calls++
				return tc.result, nil
			})
			g := make(LayoutGeometryTable)
			eligibility, err := layoutBlockContainer(n, u, g, adapter, nil, false)
			if err != nil || eligibility.Through != tc.want || calls != 1 {
				t.Fatalf("eligibility=%+v, calls=%d, error=%v", eligibility, calls, err)
			}
			// The same subtree passed as a layout root is a containment boundary.
			rootEligibility, err := layoutBlockContainer(n, u, g, adapter, nil, true)
			if err != nil || rootEligibility.collapseEligibility != (collapseEligibility{}) {
				t.Fatalf("root eligibility=%+v, error=%v", rootEligibility, err)
			}
		})
	}
}

func TestNestedCollapseEligibilityAndHeight(t *testing.T) {
	leaf := newRenderElement(3, "block")
	leaf.Style = &ComputedStyle{Margin: EdgeLengths{Top: lenPx(7), Bottom: lenPx(11)}}
	middle := newRenderElement(2, "block", leaf)
	middle.Style = &ComputedStyle{Margin: EdgeLengths{Top: lenPx(3), Bottom: lenPx(5)}}
	n, err := BuildLayoutTree(newRenderElement(1, "block", middle), BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	u, err := ResolveUsedValues(n, ResolveContext{ContainingBlock: Rect{W: 100}})
	if err != nil {
		t.Fatal(err)
	}
	for _, barrier := range []bool{false, true} {
		leafID := n.Children[0].Children[0].BoxID
		v := u[leafID]
		if barrier {
			v.Padding.Bottom = 1
		}
		u[leafID] = v
		adapter := &emptyInlineRecorder{}
		g := make(LayoutGeometryTable)
		eligibility, err := layoutBlockContainer(n, u, g, adapter, nil, false)
		if err != nil || eligibility.Through == barrier || adapter.calls != 1 {
			t.Fatalf("barrier=%v: eligibility=%+v, calls=%d, error=%v", barrier, eligibility, adapter.calls, err)
		}
		wantHeight := float32(0)
		if barrier {
			wantHeight++
		}
		if g[n.BoxID].Content.H != wantHeight || g[leafID].Frame.Y != 0 {
			t.Fatalf("incorrect collapsed geometry: %+v", g)
		}
	}
}

func TestInvalidInlineCollapseDeclaration(t *testing.T) {
	for _, atomic := range []bool{false, true} {
		input := newRenderElement(1, "block")
		if atomic {
			a := newRenderElement(2, "inline-block")
			a.Style = &ComputedStyle{Width: lenPx(0)}
			input = newRenderElement(1, "block", a)
		}
		n, err := BuildLayoutTree(input, BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		u, err := ResolveUsedValues(n, ResolveContext{ContainingBlock: Rect{W: 100}})
		if err != nil {
			t.Fatal(err)
		}
		original := copyTestTree(n)
		before := make(UsedValuesTable)
		for id, v := range u {
			before[id] = v
		}
		adapter := resultFunc(func(req InlineRequest) (InlineResult, error) {
			r := InlineResult{AllowsMarginCollapse: true}
			if !atomic {
				r.Extent.H = 1 // Reserved space contradicts collapse permission.
			} else if req.OwnerID == n.BoxID {
				// Even a zero-sized atomic separates margins.
				r.Atomics = []AtomicPlacement{{BoxID: n.Children[0].Children[0].BoxID}}
			}
			return r, nil
		})
		result, err := FlowLayout(n, u, adapter, nil, LayoutContext{}, LayoutOptions{})
		if result != nil || err == nil || !strings.Contains(err.Error(), "margin collapse requires") {
			t.Fatalf("atomic=%v: result=%v, error=%v", atomic, result, err)
		}
		if !reflect.DeepEqual(n, original) || !reflect.DeepEqual(u, before) {
			t.Fatal("failed declaration mutated inputs")
		}
	}
}
