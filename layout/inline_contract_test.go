package layout

import (
	"errors"
	"math"
	"testing"
)

type inlineFunc func(InlineRequest) (InlineExtent, error)

func (f inlineFunc) LayoutInline(req InlineRequest) (InlineResult, error) {
	e, err := f(req)
	return InlineResult{Extent: e}, err
}

// Stand-in for P's concrete result: retained by the adapter, never by the core.
type testParagraph struct{ body, trailing float32 }
type testRetainedParagraph struct {
	owner  BoxID
	result testParagraph
}

func TestInlineBoundary_TypedRetentionAndFullExtent(t *testing.T) {
	note := newRenderElement(3, "block", newRenderText(4, "inside"))
	note.Style = &ComputedStyle{Width: lenPx(120)}
	input := newRenderElement(1, "block", newRenderText(2, "before"), note, newRenderText(5, "after"))
	input.Style = &ComputedStyle{Width: lenPx(240)}
	root, err := BuildLayoutTree(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 800}})
	if err != nil {
		t.Fatal(err)
	}
	retained := make(map[BoxID]testRetainedParagraph)
	paragraphs := []testParagraph{{20, 4}, {30, 6}, {10, 2}}
	calls := 0
	adapter := inlineFunc(func(req InlineRequest) (InlineExtent, error) {
		i := calls
		calls++
		if i >= len(paragraphs) {
			t.Fatal("unexpected paragraph request")
		}
		owner := root.Children[i]
		if req.Group != owner.Children[0] || req.OwnerID != owner.BoxID {
			t.Fatal("group/owner association lost")
		}
		if req.ContentWidth != []float32{240, 120, 240}[i] {
			t.Fatalf("request width=%v", req.ContentWidth)
		}
		value, ok := req.Used.Lookup(req.OwnerID)
		if !ok || value.ContentWidth != req.ContentWidth {
			t.Fatal("missing owner used values")
		}
		value.ContentWidth = 999 // Lookup returns a value, not a mutable map entry.
		again, _ := req.Used.Lookup(req.OwnerID)
		if again.ContentWidth != req.ContentWidth {
			t.Fatal("used values view leaked mutation")
		}
		if _, ok := req.Used.Lookup(req.Group.BoxID); !ok {
			t.Fatal("missing group used values")
		}
		if _, ok := req.Used.Lookup(0); ok {
			t.Fatal("unknown identity found")
		}
		retained[req.Group.BoxID] = testRetainedParagraph{req.OwnerID, paragraphs[i]}
		return InlineExtent{W: req.ContentWidth + 10, H: paragraphs[i].body + paragraphs[i].trailing}, nil
	})
	result, err := FlowLayout(root, used, adapter, nil, LayoutContext{}, LayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 || len(retained) != 3 {
		t.Fatal("anonymous groups were omitted")
	}
	for i, owner := range root.Children {
		r, ok := retained[owner.Children[0].BoxID]
		if !ok || r.owner != owner.BoxID || r.result != paragraphs[i] {
			t.Fatal("typed result association lost")
		}
		g := result.Geometry[owner.BoxID]
		if g.Content.H != paragraphs[i].body+paragraphs[i].trailing || g.Content.W != []float32{240, 120, 240}[i] {
			t.Fatalf("extent did not set height or changed allocated width: %+v", g)
		}
		if g.Frame.Y != []float32{0, 24, 60}[i] {
			t.Fatalf("owner %d Y=%v", i, g.Frame.Y)
		}
	}
	if result.Geometry[root.BoxID].Content.H != 72 {
		t.Fatal("trailing paragraph space lost")
	}
}

func TestInlineBoundary_EmptyGroupCanReserveSpace(t *testing.T) {
	for _, height := range []float32{0, 8} {
		root, err := BuildLayoutTree(newRenderElement(1, "block"), BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		used, _ := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 0}})
		u := used[root.BoxID]
		u.Padding = Edges{Top: 2, Bottom: 3}
		used[root.BoxID] = u
		adapter := inlineFunc(func(req InlineRequest) (InlineExtent, error) {
			if len(req.Group.Children) != 0 || req.ContentWidth != 0 {
				t.Fatal("expected empty group at zero width")
			}
			return InlineExtent{H: height}, nil
		})
		result, err := FlowLayout(root, used, adapter, nil, LayoutContext{}, LayoutOptions{})
		if err != nil {
			t.Fatal(err)
		}
		g := result.Geometry[root.BoxID]
		if g.Content.H != height || g.Frame.H != height+5 {
			t.Fatalf("empty group extent=%+v", g)
		}
	}
}

func TestInlineBoundary_InvalidExtents(t *testing.T) {
	for _, extent := range []InlineExtent{{W: -1}, {H: -1}, {W: float32(math.NaN())}, {H: float32(math.NaN())}, {W: float32(math.Inf(1))}, {H: float32(math.Inf(1))}} {
		root, _ := BuildLayoutTree(newRenderElement(1, "block"), BuildOptions{})
		used, _ := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 100}})
		result, err := FlowLayout(root, used, inlineFunc(func(InlineRequest) (InlineExtent, error) { return extent, nil }), nil, LayoutContext{}, LayoutOptions{})
		if err == nil || result != nil {
			t.Fatalf("accepted invalid extent %+v", extent)
		}
	}
}

func TestInlineBoundary_RejectsBadInputBeforeCallbacks(t *testing.T) {
	for _, tc := range []struct {
		name    string
		corrupt func(*LayoutNode, UsedValuesTable)
	}{
		{"missing_owner", func(n *LayoutNode, u UsedValuesTable) { delete(u, n.BoxID) }},
		{"missing_group", func(n *LayoutNode, u UsedValuesTable) { delete(u, n.Children[0].BoxID) }},
		{"negative_width", func(n *LayoutNode, u UsedValuesTable) { v := u[n.BoxID]; v.ContentWidth = -1; u[n.BoxID] = v }},
		{"nan_width", func(n *LayoutNode, u UsedValuesTable) {
			v := u[n.BoxID]
			v.ContentWidth = float32(math.NaN())
			u[n.BoxID] = v
		}},
		{"infinite_padding", func(n *LayoutNode, u UsedValuesTable) {
			v := u[n.BoxID]
			v.Padding.Left = float32(math.Inf(1))
			u[n.BoxID] = v
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := BuildLayoutTree(newRenderElement(1, "block"), BuildOptions{})
			used, _ := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 100}})
			tc.corrupt(root, used)
			calls := 0
			result, err := FlowLayout(root, used, inlineFunc(func(InlineRequest) (InlineExtent, error) { calls++; return InlineExtent{}, nil }), nil, LayoutContext{}, LayoutOptions{})
			if err == nil || result != nil || calls != 0 {
				t.Fatal("invalid input reached paragraph callback")
			}
		})
	}
}

func TestInlineBoundary_PropagatesAdapterError(t *testing.T) {
	root, _ := BuildLayoutTree(newRenderElement(1, "block"), BuildOptions{})
	used, _ := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 100}})
	failure := errors.New("paragraph failed")
	result, err := FlowLayout(root, used, inlineFunc(func(InlineRequest) (InlineExtent, error) { return InlineExtent{}, failure }), nil, LayoutContext{}, LayoutOptions{})
	if result != nil || !errors.Is(err, failure) {
		t.Fatalf("lost adapter error: %v", err)
	}
	result, err = FlowLayout(root, used, nil, nil, LayoutContext{}, LayoutOptions{})
	if result != nil || err == nil {
		t.Fatal("missing inline service must fail")
	}
}
