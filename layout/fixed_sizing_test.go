package layout

import (
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
)

func fixedSizingRoot(t *testing.T, style ComputedStyle, children ...*RenderNode) *LayoutNode {
	t.Helper()
	n := fixedElement(1, blockDisplay, children...)
	n.Style = &style
	trees, err := BuildLayoutTrees(n, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return trees.Fixed[0]
}

func anchoredFixedStyle() ComputedStyle {
	s := DefaultComputedStyle()
	s.Position = PositionFixed
	f := DefaultFixedStyle()
	f.Insets.Left, f.Insets.Top = lenPx(0), lenPx(0)
	s.Fixed = &f
	return s
}

func TestFixedSizingAxisConstraints(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		start, end, size, ms, me Length
		wantX, wantY             fixedAxisResult
	}{
		{"start anchored", lenPx(10), lenAuto(), lenPx(40), lenPx(5), lenPx(7), fixedAxisResult{10, 130, 40, 5, 7}, fixedAxisResult{10, 130, 40, 5, 7}},
		{"end anchored", lenAuto(), lenPx(20), lenPx(40), lenPx(5), lenPx(7), fixedAxisResult{120, 20, 40, 5, 7}, fixedAxisResult{120, 20, 40, 5, 7}},
		{"overconstrained", lenPx(10), lenPx(20), lenPx(40), lenPx(5), lenPx(7), fixedAxisResult{10, 130, 40, 5, 7}, fixedAxisResult{10, 130, 40, 5, 7}},
		{"auto margins center", lenPx(10), lenPx(20), lenPx(40), lenAuto(), lenAuto(), fixedAxisResult{10, 20, 40, 61, 61}, fixedAxisResult{10, 20, 40, 61, 61}},
		{"auto start margin", lenPx(10), lenPx(20), lenPx(40), lenAuto(), lenPx(7), fixedAxisResult{10, 20, 40, 115, 7}, fixedAxisResult{10, 20, 40, 115, 7}},
		{"auto end margin", lenPx(10), lenPx(20), lenPx(40), lenPx(5), lenAuto(), fixedAxisResult{10, 20, 40, 5, 117}, fixedAxisResult{10, 20, 40, 5, 117}},
		{"negative auto margins", lenPx(10), lenPx(20), lenPx(240), lenAuto(), lenAuto(), fixedAxisResult{10, 20, 240, 0, -78}, fixedAxisResult{10, 20, 240, -39, -39}},
		{"negative start auto margin", lenPx(10), lenPx(20), lenPx(240), lenAuto(), lenPx(7), fixedAxisResult{10, 20, 240, -85, 7}, fixedAxisResult{10, 20, 240, -85, 7}},
		{"negative end auto margin", lenPx(10), lenPx(20), lenPx(240), lenPx(5), lenAuto(), fixedAxisResult{10, 20, 240, 5, -83}, fixedAxisResult{10, 20, 240, 5, -83}},
		{"auto margins with auto end", lenPx(10), lenAuto(), lenPx(40), lenAuto(), lenAuto(), fixedAxisResult{10, 142, 40, 0, 0}, fixedAxisResult{10, 142, 40, 0, 0}},
		{"auto margins with auto start", lenAuto(), lenPx(20), lenPx(40), lenAuto(), lenAuto(), fixedAxisResult{132, 20, 40, 0, 0}, fixedAxisResult{132, 20, 40, 0, 0}},
		{"content size start anchored", lenPx(10), lenAuto(), lenAuto(), lenAuto(), lenAuto(), fixedAxisResult{10, 122, 60, 0, 0}, fixedAxisResult{10, 122, 60, 0, 0}},
		{"content size end anchored", lenAuto(), lenPx(20), lenAuto(), lenAuto(), lenAuto(), fixedAxisResult{112, 20, 60, 0, 0}, fixedAxisResult{112, 20, 60, 0, 0}},
		{"stretch", lenPx(10), lenPx(20), lenAuto(), lenPx(5), lenPx(7), fixedAxisResult{10, 20, 150, 5, 7}, fixedAxisResult{10, 20, 150, 5, 7}},
		{"stretch auto margins zero", lenPx(10), lenPx(20), lenAuto(), lenAuto(), lenAuto(), fixedAxisResult{10, 20, 162, 0, 0}, fixedAxisResult{10, 20, 162, 0, 0}},
		{"stretch minimum zero", lenPx(150), lenPx(80), lenAuto(), lenPx(5), lenPx(7), fixedAxisResult{150, 30, 0, 5, 7}, fixedAxisResult{150, 30, 0, 5, 7}},
		{"minimum zero auto margins", lenPx(150), lenPx(80), lenAuto(), lenAuto(), lenAuto(), fixedAxisResult{150, 80, 0, 0, -38}, fixedAxisResult{150, 80, 0, -19, -19}},
		{"explicit zero", lenPx(10), lenAuto(), lenPx(0), lenPx(0), lenPx(0), fixedAxisResult{10, 182, 0, 0, 0}, fixedAxisResult{10, 182, 0, 0, 0}},
		{"negative inset and margin", lenPx(-10), lenAuto(), lenPx(40), lenPx(-5), lenPx(7), fixedAxisResult{-10, 160, 40, -5, 7}, fixedAxisResult{-10, 160, 40, -5, 7}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := anchoredFixedStyle()
			s.Width, s.Fixed.Height = tc.size, tc.size
			s.Fixed.Insets = EdgeLengths{tc.start, tc.end, tc.end, tc.start}
			s.Margin = EdgeLengths{tc.ms, tc.me, tc.me, tc.ms}
			s.Padding = EdgeLengths{lenPx(2), lenPx(3), lenPx(3), lenPx(2)}
			s.Border = EdgeLengths{lenPx(1), lenPx(2), lenPx(2), lenPx(1)}
			root := fixedSizingRoot(t, s)
			before := copyTestTree(root)
			fixedCopy := *root.Style.Fixed
			before.Style.Fixed = &fixedCopy
			calls := 0
			measure := measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) { calls++; return IntrinsicWidths{30, 60}, nil })
			sizing, err := resolveFixedSizing(root, Rect{W: 200, H: 200}, 16, measure)
			if err != nil {
				t.Fatal(err)
			}
			pending := tc.size.Kind == LenAuto && (tc.start.Kind == LenAuto || tc.end.Kind == LenAuto)
			if sizing.HeightPending != pending || (pending && sizing.Vertical != (fixedAxisResult{})) {
				t.Fatal("incorrect pending height state")
			}
			if calls != map[bool]int{false: 0, true: 1}[pending] {
				t.Fatalf("measurement calls = %d", calls)
			}
			finished, err := sizing.withContentHeight(60)
			if err != nil {
				t.Fatal(err)
			}
			if finished.HeightPending || finished.Horizontal != tc.wantX || finished.Vertical != tc.wantY {
				t.Fatalf("got x=%+v y=%+v, want x=%+v y=%+v", finished.Horizontal, finished.Vertical, tc.wantX, tc.wantY)
			}
			if finished.Used.ContentWidth != tc.wantX.Size || finished.Used.Margin != (Edges{tc.wantY.MarginStart, tc.wantX.MarginEnd, tc.wantY.MarginEnd, tc.wantX.MarginStart}) {
				t.Fatalf("used values inconsistent: %+v", finished.Used)
			}
			for _, axis := range []fixedAxisResult{finished.Horizontal, finished.Vertical} {
				if axis.Start+axis.MarginStart+3+axis.Size+5+axis.MarginEnd+axis.End != 200 {
					t.Fatal("constraint equation does not balance")
				}
			}
			if !reflect.DeepEqual(root, before) || sizing.HeightPending != pending {
				t.Fatal("sizing mutated input or prior result")
			}
		})
	}
}

func TestFixedSizingPercentageAndFontBases(t *testing.T) {
	s := anchoredFixedStyle()
	s.Width, s.Fixed.Height = lenPct(.5), lenPct(.25)
	s.Fixed.Insets = EdgeLengths{lenAuto(), lenAuto(), lenPct(.1), lenPct(.1)}
	s.Margin, s.Padding, s.Border = edges(lenPct(.02)), edges(lenPct(.01)), edges(lenPx(1))
	root := fixedSizingRoot(t, s)
	got, err := resolveFixedSizing(root, Rect{X: 75, Y: -20, W: 300, H: 200}, 16, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Horizontal != (fixedAxisResult{30, 100, 150, 6, 6}) || got.Vertical != (fixedAxisResult{110, 20, 50, 6, 6}) || got.Used.Padding != (Edges{3, 3, 3, 3}) {
		t.Fatalf("wrong percentage bases: %+v", got)
	}
	local, err := resolveFixedSizing(root, Rect{W: 300, H: 200}, 16, nil)
	if err != nil || !reflect.DeepEqual(got, local) {
		t.Fatal("viewport origin affected local sizing")
	}
	for _, computed := range []float32{0, 20} {
		s = anchoredFixedStyle()
		s.FontSizePx, s.Width, s.Fixed.Height = computed, lenEm(3), lenEm(2)
		s.Fixed.Insets.Left, s.Fixed.Insets.Top = lenEm(2), lenEm(1)
		r, err := resolveFixedSizing(fixedSizingRoot(t, s), Rect{W: 300, H: 200}, 10, nil)
		if err != nil {
			t.Fatal(err)
		}
		font := float32(10)
		if computed != 0 {
			font = computed
		}
		if r.Horizontal.Size != 3*font || r.Vertical.Size != 2*font || r.Horizontal.Start != 2*font || r.Vertical.Start != font {
			t.Fatal("wrong em font basis")
		}
	}
	s = anchoredFixedStyle()
	s.Width, s.Fixed.Height = lenPct(.5), lenPct(.5)
	zero, err := resolveFixedSizing(fixedSizingRoot(t, s), Rect{}, 0, nil)
	if err != nil || zero.HeightPending || zero.Horizontal != (fixedAxisResult{}) || zero.Vertical != (fixedAxisResult{}) {
		t.Fatalf("zero viewport: %+v %v", zero, err)
	}
}

func TestFixedSizingShrinkToFit(t *testing.T) {
	for _, tc := range []struct{ viewport, want float32 }{{200, 90}, {80, 50}, {20, 30}, {0, 30}} {
		for _, right := range []bool{false, true} {
			s := anchoredFixedStyle()
			s.Fixed.Height = lenPx(10)
			s.Margin.Left, s.Margin.Right = lenPx(2), lenPx(3)
			s.Padding = edges(lenPx(3))
			s.Border = edges(lenPx(2))
			s.Fixed.Insets.Left = lenPx(15)
			if right {
				s.Fixed.Insets.Left, s.Fixed.Insets.Right = lenAuto(), lenPx(15)
			}
			root := fixedSizingRoot(t, s, newRenderText(2, "paragraph"))
			calls := 0
			measure := measureFunc(func(req InlineMeasureRequest) (IntrinsicWidths, error) {
				calls++
				if req.OwnerID != root.BoxID || req.Group != root.Children[0] {
					t.Fatal("wrong intrinsic group")
				}
				e, ok := req.Edges.Lookup(root.BoxID)
				if !ok || e.Margin.Left != 2 || e.Padding.Right != 3 || e.Border.Left != 2 {
					t.Fatal("missing root edges")
				}
				return IntrinsicWidths{30, 90}, nil
			})
			got, err := resolveFixedSizing(root, Rect{W: tc.viewport, H: 200}, 16, measure)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || got.Used.ContentWidth != tc.want {
				t.Fatalf("width=%g want=%g calls=%d", got.Used.ContentWidth, tc.want, calls)
			}
			if right && got.Horizontal.End != 15 || !right && got.Horizontal.Start != 15 {
				t.Fatal("lost definite inset")
			}
		}
	}
}

func TestFixedSizingHeightCompletion(t *testing.T) {
	s := anchoredFixedStyle()
	s.Width = lenPx(40)
	s.Fixed.Insets.Top, s.Fixed.Insets.Bottom = lenAuto(), lenPx(10)
	s.Margin.Top, s.Margin.Bottom = lenAuto(), lenPx(5)
	s.Padding.Top, s.Border.Bottom = lenPx(2), lenPx(3)
	root := fixedSizingRoot(t, s)
	pending, err := resolveFixedSizing(root, Rect{W: 100, H: 200}, 16, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Completion owns resolved values, not aliases to the original constraints.
	root.Style.Fixed.Insets.Bottom = lenPx(999)
	for _, height := range []float32{0, 30, 250} {
		got, err := pending.withContentHeight(height)
		if err != nil {
			t.Fatal(err)
		}
		if got.Vertical != (fixedAxisResult{180 - height, 10, height, 0, 5}) || !pending.HeightPending {
			t.Fatalf("completion: %+v", got)
		}
	}
	for _, height := range []float32{-1, float32(math.NaN()), float32(math.Inf(1))} {
		if got, err := pending.withContentHeight(height); err == nil || got != nil {
			t.Fatal("invalid natural height accepted")
		}
	}
	for _, height := range []Length{lenPx(0), lenPx(40), lenAuto()} {
		s = anchoredFixedStyle()
		s.Width = lenPx(40)
		s.Fixed.Height = height
		s.Fixed.Insets.Bottom = lenPx(10)
		prepared, err := resolveFixedSizing(fixedSizingRoot(t, s), Rect{W: 100, H: 200}, 16, nil)
		if err != nil {
			t.Fatal(err)
		}
		completed, err := prepared.withContentHeight(999)
		if err != nil || !reflect.DeepEqual(prepared, completed) {
			t.Fatal("natural overflow changed prescribed/stretch height")
		}
	}
}

func TestFixedSizingFailureAndReplay(t *testing.T) {
	s := anchoredFixedStyle()
	root := fixedSizingRoot(t, s, newRenderText(2, "text"))
	failure := errors.New("measurement failure")
	if got, err := resolveFixedSizing(root, Rect{W: 200, H: 100}, 16, measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) { return IntrinsicWidths{}, failure })); got != nil || !errors.Is(err, failure) {
		t.Fatalf("failure not preserved: %v", err)
	}
	measure := measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) { return IntrinsicWidths{20, 80}, nil })
	before := copyTestTree(root)
	want, err := resolveFixedSizing(root, Rect{W: 200, H: 100}, 16, measure)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := resolveFixedSizing(root, Rect{W: 200, H: 100}, 16, measure)
			if err != nil || !reflect.DeepEqual(want, got) {
				t.Errorf("concurrent replay: %v", err)
			}
		}()
	}
	wg.Wait()
	resized, err := resolveFixedSizing(root, Rect{W: 30, H: 50}, 16, measure)
	if err != nil || resized.Used.ContentWidth != 30 || want.Used.ContentWidth != 80 || !reflect.DeepEqual(root, before) {
		t.Fatal("resize mutated state")
	}
}
