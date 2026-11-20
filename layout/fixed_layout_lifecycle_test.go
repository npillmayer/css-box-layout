package layout

import (
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
)

func fixedLayoutPair(t *testing.T) []*LayoutNode {
	t.Helper()
	var children []*RenderNode
	for _, id := range []NodeID{2, 4} {
		s := anchoredFixedStyle()
		s.Fixed.Insets = EdgeLengths{lenAuto(), lenPx(20), lenPx(10), lenAuto()}
		n := fixedElement(id, blockDisplay, newRenderText(id+1, "text"))
		n.Style = &s
		children = append(children, n)
	}
	trees, err := BuildLayoutTrees(newRenderElement(1, blockDisplay, children...), BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return trees.Fixed
}

func TestFixedLayoutEmptyAndZeroViewport(t *testing.T) {
	for _, roots := range [][]*LayoutNode{nil, {}} {
		r, err := FixedLayout(roots, Rect{}, nil, nil, FixedLayoutOptions{})
		if err != nil || r == nil || r.Geometry == nil || len(r.Geometry) != 0 || len(r.Roots) != 0 || r.Viewport != (Rect{}) {
			t.Fatalf("empty result=%+v, error=%v", r, err)
		}
	}
	for _, tc := range []struct {
		viewport Rect
		font     float32
	}{
		{Rect{W: -1}, 0}, {Rect{H: -1}, 0},
		{Rect{X: float32(math.NaN())}, 0}, {Rect{Y: float32(math.Inf(1))}, 0},
		{Rect{W: float32(math.Inf(1))}, 0}, {Rect{H: float32(math.NaN())}, 0},
		{Rect{}, -1}, {Rect{}, float32(math.NaN())},
	} {
		r, err := FixedLayout(nil, tc.viewport, nil, nil, FixedLayoutOptions{FontSizePx: tc.font})
		if r != nil || err == nil {
			t.Fatalf("invalid empty context accepted: %+v, %v", tc, err)
		}
	}
	for _, size := range []Length{lenPx(0), lenAuto()} {
		s := anchoredFixedStyle()
		s.Width, s.Fixed.Height = size, size
		s.Fixed.Insets = EdgeLengths{} // stretch at zero must not become pending
		root := fixedSizingRoot(t, s)
		adapter := newAtomicFake(t)
		r, err := FixedLayout([]*LayoutNode{root}, Rect{}, adapter, nil, FixedLayoutOptions{})
		if err != nil || r == nil || r.Geometry[root.BoxID] != (LayoutGeometry{}) || len(adapter.calls) != 1 {
			t.Fatalf("zero viewport result=%+v, error=%v", r, err)
		}
	}
}

func TestFixedLayoutValidatesWholeForestBeforeCallbacks(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func([]*LayoutNode)
	}{
		{"nil later root", func(r []*LayoutNode) { r[1] = nil }},
		{"duplicate root", func(r []*LayoutNode) { r[1] = r[0] }},
		{"cross-root identity", func(r []*LayoutNode) { r[1].Children[0].BoxID = r[0].BoxID }},
		{"shared subtree", func(r []*LayoutNode) { r[1].Children = r[0].Children }},
		{"zero identity", func(r []*LayoutNode) { r[1].BoxID = 0 }},
		{"later malformed text", func(r []*LayoutNode) { r[1].Children[0].Children[0].Text.Range.End = 0 }},
		{"later static root", func(r []*LayoutNode) { r[1].Style.Position = PositionStatic }},
		{"nested fixed leak", func(r []*LayoutNode) { r[1].Children = []*LayoutNode{r[0]} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			roots := fixedLayoutPair(t)
			tc.edit(roots)
			measured, laidOut := 0, 0
			r, err := FixedLayout(roots, Rect{W: 300, H: 200}, resultFunc(func(InlineRequest) (InlineResult, error) {
				laidOut++
				return InlineResult{}, nil
			}), measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) {
				measured++
				return IntrinsicWidths{40, 200}, nil
			}), FixedLayoutOptions{})
			if err == nil || r != nil || measured != 0 || laidOut != 0 {
				t.Fatalf("result=%+v, error=%v, measured=%d, laidOut=%d", r, err, measured, laidOut)
			}
		})
	}
}

func TestFixedLayoutPreparesWholeForestBeforeFinalCallbacks(t *testing.T) {
	failure := errors.New("late measurement failed")
	for _, scenario := range []string{"static fallback", "width overflow", "descendant edges overflow", "measurement failure", "invalid measurement", "missing measurement"} {
		t.Run(scenario, func(t *testing.T) {
			roots := fixedLayoutPair(t)
			later := roots[1]
			switch scenario {
			case "static fallback":
				later.Style.Fixed.Insets.Right = lenAuto()
			case "width overflow":
				later.Style.Width, later.Style.Padding.Right = lenPx(math.MaxFloat32), lenPx(math.MaxFloat32)
			case "descendant edges overflow":
				later.Style.Width = lenPx(80)
				later.Children[0].Style.Margin.Left = lenEm(math.MaxFloat32)
			}
			var measure IntrinsicMeasurer = measureFunc(func(req InlineMeasureRequest) (IntrinsicWidths, error) {
				if req.OwnerID == later.BoxID {
					if scenario == "measurement failure" {
						return IntrinsicWidths{}, failure
					}
					if scenario == "invalid measurement" {
						return IntrinsicWidths{20, 10}, nil
					}
				}
				return IntrinsicWidths{40, 200}, nil
			})
			if scenario == "missing measurement" {
				measure = nil
			}
			calls := 0
			r, err := FixedLayout(roots, Rect{W: 300, H: 200}, resultFunc(func(InlineRequest) (InlineResult, error) {
				calls++
				return InlineResult{}, nil
			}), measure, FixedLayoutOptions{FontSizePx: 16})
			if r != nil || err == nil || calls != 0 {
				t.Fatalf("result=%+v, error=%v, final calls=%d", r, err, calls)
			}
			if scenario == "measurement failure" && !errors.Is(err, failure) {
				t.Fatalf("lost measurement error: %v", err)
			}
			if scenario == "static fallback" && !errors.Is(err, ErrUnsupportedPosition) {
				t.Fatalf("lost unsupported-position error: %v", err)
			}
		})
	}
}

func TestFixedLayoutRejectsLateLayoutErrors(t *testing.T) {
	for _, scenario := range []string{"invalid extent", "height overflow", "missing layouter"} {
		t.Run(scenario, func(t *testing.T) {
			roots := fixedLayoutPair(t)
			for _, root := range roots {
				root.Style.Width = lenPx(80)
			}
			if scenario == "height overflow" {
				roots[1].Style.Fixed.Insets.Top = lenPx(math.MaxFloat32)
				roots[1].Style.Fixed.Insets.Bottom = lenAuto()
			}
			calls := 0
			var inline InlineLayouter = resultFunc(func(InlineRequest) (InlineResult, error) {
				calls++
				h := float32(10)
				if calls == 2 {
					h = math.MaxFloat32
					if scenario == "invalid extent" {
						h = float32(math.NaN())
					}
				}
				return InlineResult{Extent: InlineExtent{W: 80, H: h}}, nil
			})
			if scenario == "missing layouter" {
				inline = nil
			}
			r, err := FixedLayout(roots, Rect{W: 300, H: 200}, inline, nil, FixedLayoutOptions{})
			if r != nil || err == nil || (inline != nil && calls != 2) {
				t.Fatalf("result=%+v, error=%v, calls=%d", r, err, calls)
			}
		})
	}
}

func TestFixedLayoutResizeAndFailedPublication(t *testing.T) {
	roots := fixedLayoutPair(t)
	before := []*LayoutNode{copyTestTree(roots[0]), copyTestTree(roots[1])}
	failure := errors.New("second paragraph failed")
	type publication struct {
		layout     *FixedLayoutResult
		paragraphs map[BoxID]InlineExtent
	}
	var published *publication
	measure := measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) { return IntrinsicWidths{40, 200}, nil })
	run := func(width float32, fail bool) error {
		pending := &publication{paragraphs: map[BoxID]InlineExtent{}}
		calls := 0
		r, err := FixedLayout(roots, Rect{10, 20, width, 100}, resultFunc(func(req InlineRequest) (InlineResult, error) {
			calls++
			if fail && calls == 2 {
				return InlineResult{}, failure
			}
			extent := InlineExtent{W: req.ContentWidth, H: 1200 / req.ContentWidth}
			pending.paragraphs[req.Group.BoxID] = extent
			return InlineResult{Extent: extent}, nil
		}), measure, FixedLayoutOptions{})
		if err != nil {
			if r != nil || len(pending.paragraphs) != 1 {
				t.Fatal("failure did not exercise partial adapter retention")
			}
			return err // Discard the entire pending adapter invocation.
		}
		pending.layout, published = r, pending
		return nil
	}
	if err := run(300, false); err != nil {
		t.Fatal(err)
	}
	first := published
	geometry := copyTestGeometry(first.layout.Geometry)
	if err := run(100, false); err != nil {
		t.Fatal(err)
	}
	second := published
	for _, root := range roots {
		id, group := root.BoxID, root.Children[0].BoxID
		if geometry[id].Frame != (Rect{80, 84, 200, 6}) || second.layout.Geometry[id].Frame != (Rect{0, 75, 80, 15}) {
			t.Fatalf("resize geometry: first=%+v, second=%+v", geometry[id], second.layout.Geometry[id])
		}
		if first.paragraphs[group] != (InlineExtent{200, 6}) || second.paragraphs[group] != (InlineExtent{80, 15}) {
			t.Fatal("paragraph snapshots changed or were not resized")
		}
	}
	secondGeometry := copyTestGeometry(second.layout.Geometry)
	if err := run(140, true); !errors.Is(err, failure) || published != second {
		t.Fatalf("failed invocation changed publication or lost error: %v", err)
	}
	if !reflect.DeepEqual(first.layout.Geometry, geometry) || !reflect.DeepEqual(second.layout.Geometry, secondGeometry) || !reflect.DeepEqual(roots, before) {
		t.Fatal("layout changed prior snapshots or inputs")
	}
	if err := run(300, false); err != nil || !reflect.DeepEqual(published.layout.Geometry, geometry) {
		t.Fatalf("replay after failure: %v", err)
	}
}

func TestFixedLayoutConcurrentReadOnlyInputs(t *testing.T) {
	roots := fixedLayoutPair(t)
	before := []*LayoutNode{copyTestTree(roots[0]), copyTestTree(roots[1])}
	var wg sync.WaitGroup
	results := make(chan *FixedLayoutResult, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := FixedLayout(roots, Rect{W: 100, H: 100}, resultFunc(func(req InlineRequest) (InlineResult, error) {
				return InlineResult{Extent: InlineExtent{W: req.ContentWidth, H: 15}}, nil
			}), measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) { return IntrinsicWidths{40, 200}, nil }), FixedLayoutOptions{})
			if err != nil {
				errs <- err
				return
			}
			results <- r
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	for r := range results {
		for _, root := range roots {
			if r.Geometry[root.BoxID].Frame != (Rect{0, 75, 80, 15}) {
				t.Fatal("concurrent layout changed geometry")
			}
		}
	}
	if !reflect.DeepEqual(roots, before) {
		t.Fatal("concurrent calls changed input")
	}
}
