package layout

import (
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
)

func definiteFixedStyle() ComputedStyle {
	s := anchoredFixedStyle()
	s.Width, s.Fixed.Height = lenPx(80), lenPx(30)
	s.Fixed.Insets.Left, s.Fixed.Insets.Top = lenPx(10), lenPx(20)
	return s
}

func TestResolveFixedGeometry(t *testing.T) {
	for _, tc := range []struct {
		name           string
		edit           func(*ComputedStyle)
		frame, content Rect
	}{
		{"explicit", func(*ComputedStyle) {}, Rect{10, 20, 80, 30}, Rect{10, 20, 80, 30}},
		{"page layout example", func(s *ComputedStyle) {
			s.Fixed.Insets.Left, s.Fixed.Insets.Right, s.Fixed.Insets.Top = lenAuto(), lenPx(20), lenPx(10)
			s.Padding, s.Border = edges(lenPx(3)), edges(lenPx(2))
		}, Rect{190, 10, 90, 40}, Rect{195, 15, 80, 30}},
		{"asymmetric edges", func(s *ComputedStyle) {
			s.Fixed.Height = lenPx(40)
			s.Margin = EdgeLengths{lenPx(2), lenPx(3), lenPx(4), lenPx(5)}
			s.Padding = EdgeLengths{lenPx(7), lenPx(11), lenPx(13), lenPx(17)}
			s.Border = EdgeLengths{lenPx(1), lenPx(2), lenPx(3), lenPx(4)}
		}, Rect{15, 22, 114, 64}, Rect{36, 30, 80, 40}},
		{"right bottom", func(s *ComputedStyle) {
			s.Fixed.Height = lenPx(40)
			s.Fixed.Insets = EdgeLengths{lenAuto(), lenPx(10), lenPx(20), lenAuto()}
			s.Margin = EdgeLengths{lenPx(2), lenPx(3), lenPx(4), lenPx(5)}
			s.Padding = EdgeLengths{lenPx(7), lenPx(11), lenPx(13), lenPx(17)}
			s.Border = EdgeLengths{lenPx(1), lenPx(2), lenPx(3), lenPx(4)}
		}, Rect{173, 112, 114, 64}, Rect{194, 120, 80, 40}},
		{"stretch", func(s *ComputedStyle) {
			s.Width, s.Fixed.Height = lenAuto(), lenAuto()
			s.Fixed.Insets = EdgeLengths{lenPx(30), lenPx(20), lenPx(40), lenPx(10)}
			s.Padding, s.Border = edges(lenPx(3)), edges(lenPx(2))
		}, Rect{10, 30, 270, 130}, Rect{15, 35, 260, 120}},
		{"overconstrained", func(s *ComputedStyle) {
			s.Fixed.Insets = EdgeLengths{lenPx(30), lenPx(20), lenPx(40), lenPx(10)}
		}, Rect{10, 30, 80, 30}, Rect{10, 30, 80, 30}},
		{"negative inset and margin", func(s *ComputedStyle) {
			s.Fixed.Insets.Left, s.Fixed.Insets.Top = lenPx(-10), lenPx(-20)
			s.Margin.Left, s.Margin.Top = lenPx(-5), lenPx(-7)
		}, Rect{-15, -27, 80, 30}, Rect{-15, -27, 80, 30}},
		{"auto margins", func(s *ComputedStyle) { s.Fixed.Insets = edges(lenPx(10)); s.Margin = edges(lenAuto()) }, Rect{110, 85, 80, 30}, Rect{110, 85, 80, 30}},
		{"auto margins overflow", func(s *ComputedStyle) {
			s.Fixed.Insets = edges(lenPx(10))
			s.Margin = edges(lenAuto())
			s.Width, s.Fixed.Height = lenPx(400), lenPx(300)
		}, Rect{10, -50, 400, 300}, Rect{10, -50, 400, 300}},
		{"negative stretch clamps", func(s *ComputedStyle) {
			s.Fixed.Insets = edges(lenPx(200))
			s.Width, s.Fixed.Height = lenAuto(), lenAuto()
		}, Rect{200, 200, 0, 0}, Rect{200, 200, 0, 0}},
		{"zero dimensions", func(s *ComputedStyle) { s.Width, s.Fixed.Height = lenPx(0), lenPx(0) }, Rect{10, 20, 0, 0}, Rect{10, 20, 0, 0}},
		{"percent bases", func(s *ComputedStyle) {
			s.Width, s.Fixed.Height = lenPct(.5), lenPct(.25)
			s.Fixed.Insets = EdgeLengths{lenAuto(), lenAuto(), lenPct(.1), lenPct(.1)}
			s.Margin, s.Padding, s.Border = edges(lenPct(.02)), edges(lenPct(.01)), edges(lenPx(1))
		}, Rect{36, 116, 158, 58}, Rect{40, 120, 150, 50}},
		{"fallback em", func(s *ComputedStyle) {
			s.Width, s.Fixed.Height = lenEm(4), lenEm(3)
			s.Fixed.Insets.Left, s.Fixed.Insets.Top = lenEm(1), lenEm(2)
			s.Padding = edges(lenEm(.5))
		}, Rect{10, 20, 50, 40}, Rect{15, 25, 40, 30}},
		{"computed em", func(s *ComputedStyle) {
			s.FontSizePx = 20
			s.Width, s.Fixed.Height = lenEm(4), lenEm(3)
			s.Fixed.Insets.Left, s.Fixed.Insets.Top = lenEm(1), lenEm(2)
			s.Padding = edges(lenEm(.5))
		}, Rect{20, 40, 100, 80}, Rect{30, 50, 80, 60}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := definiteFixedStyle()
			tc.edit(&s)
			before, fixedBefore := s, *s.Fixed
			viewport := Rect{1000, 2000, 300, 200}
			opts := FixedLayoutOptions{FontSizePx: 10}
			g, err := ResolveFixedGeometry(s, viewport, opts)
			want := LayoutGeometry{Frame: tc.frame, Content: tc.content}
			if err != nil || g != want {
				t.Fatalf("got %+v %v, want %+v", g, err, want)
			}
			local, err := ResolveFixedGeometry(s, Rect{W: 300, H: 200}, opts)
			if err != nil || local != g {
				t.Fatal("external viewport origin entered local geometry")
			}
			if s != before || *s.Fixed != fixedBefore {
				t.Fatal("geometry mutated input")
			}
			// Compare to the established tree route, which still lays out interiors.
			root := fixedSizingRoot(t, s, newRenderText(2, "content"))
			calls := 0
			r, err := FixedLayout([]*LayoutNode{root}, viewport, resultFunc(func(req InlineRequest) (InlineResult, error) {
				calls++
				return InlineResult{Extent: InlineExtent{W: req.ContentWidth, H: 50}}, nil
			}), measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) {
				t.Fatal("definite constraints requested measurement")
				return IntrinsicWidths{}, nil
			}), opts)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || r.Geometry[root.BoxID] != g {
				t.Fatalf("tree/direct disagreement: %+v vs %+v (calls %d)", r.Geometry[root.BoxID], g, calls)
			}
		})
	}
}

func TestResolveFixedGeometryDependencies(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*ComputedStyle)
		want error
	}{
		{"width", func(s *ComputedStyle) { s.Width = lenAuto() }, ErrFixedContentDependency},
		{"height", func(s *ComputedStyle) { s.Fixed.Height = lenAuto() }, ErrFixedContentDependency},
		{"both sizes", func(s *ComputedStyle) { s.Width, s.Fixed.Height = lenAuto(), lenAuto() }, ErrFixedContentDependency},
		{"bottom content height", func(s *ComputedStyle) {
			s.Fixed.Height = lenAuto()
			s.Fixed.Insets.Top, s.Fixed.Insets.Bottom = lenAuto(), lenPx(10)
		}, ErrFixedContentDependency},
		{"right content width", func(s *ComputedStyle) {
			s.Width = lenAuto()
			s.Fixed.Insets.Left, s.Fixed.Insets.Right = lenAuto(), lenPx(10)
		}, ErrFixedContentDependency},
		{"horizontal static position", func(s *ComputedStyle) { s.Fixed.Insets.Left = lenAuto() }, ErrUnsupportedPosition},
		{"vertical static position", func(s *ComputedStyle) { s.Fixed.Insets.Top = lenAuto() }, ErrUnsupportedPosition},
		{"nil fixed", func(s *ComputedStyle) { s.Fixed = nil }, ErrUnsupportedPosition},
		{"static", func(s *ComputedStyle) { s.Position = PositionStatic; s.Fixed = nil }, ErrUnsupportedPosition},
		{"unknown positioning", func(s *ComputedStyle) { s.Position = PositionKind(99) }, ErrUnsupportedPosition},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, viewport := range []Rect{{W: 300, H: 200}, {}} {
				s := definiteFixedStyle()
				tc.edit(&s)
				g, err := ResolveFixedGeometry(s, viewport, FixedLayoutOptions{})
				if g != (LayoutGeometry{}) || !errors.Is(err, tc.want) {
					t.Fatalf("got %+v %v, want %v", g, err, tc.want)
				}
			}
		})
	}
	// A nonnil zero FixedStyle is explicit zero, not auto; zero viewport is definite.
	s := ComputedStyle{Position: PositionFixed, Fixed: &FixedStyle{}}
	if g, err := ResolveFixedGeometry(s, Rect{}, FixedLayoutOptions{}); err != nil || g != (LayoutGeometry{}) {
		t.Fatalf("explicit zero rejected: %+v %v", g, err)
	}
}

func TestResolveFixedGeometryInvalidInput(t *testing.T) {
	nan, inf := float32(math.NaN()), float32(math.Inf(1))
	for _, tc := range []struct {
		name string
		edit func(*ComputedStyle, *Rect, *FixedLayoutOptions)
	}{
		{"negative viewport", func(_ *ComputedStyle, v *Rect, _ *FixedLayoutOptions) { v.W = -1 }},
		{"nonfinite viewport", func(_ *ComputedStyle, v *Rect, _ *FixedLayoutOptions) { v.H = inf }},
		{"nonfinite origin", func(_ *ComputedStyle, v *Rect, _ *FixedLayoutOptions) { v.X = nan }},
		{"bad fallback font", func(_ *ComputedStyle, _ *Rect, o *FixedLayoutOptions) { o.FontSizePx = -1 }},
		{"bad computed font", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.FontSizePx = nan }},
		{"bad width", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Width = lenPx(-1) }},
		{"nonfinite width", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Width.Value = nan }},
		{"bad width kind", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Width.Kind = LengthKind(99) }},
		{"bad height", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Fixed.Height = lenPx(-1) }},
		{"bad inset kind", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Fixed.Insets.Top.Kind = LengthKind(99) }},
		{"nonfinite inset", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Fixed.Insets.Left.Value = inf }},
		{"nonfinite unused auto payload", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Fixed.Insets.Right.Value = nan }},
		{"auto padding", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Padding.Left = lenAuto() }},
		{"negative border", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Border.Top = lenPx(-1) }},
		{"nonfinite margin", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Margin.Bottom.Value = inf }},
		{"bad whitespace", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.WhiteSpace = WhiteSpace(99) }},
		{"em needs font", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Width = lenEm(1) }},
		{"zero em needs font", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Margin.Left = lenEm(0) }},
		{"percent overflow", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Width = lenPct(math.MaxFloat32) }},
		{"inset overflow", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Fixed.Insets.Top = lenPct(math.MaxFloat32) }},
		{"edge overflow", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) { s.Padding.Left = lenPct(math.MaxFloat32) }},
		{"width sum overflow", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) {
			s.Width = lenPx(math.MaxFloat32)
			s.Padding.Right = lenPx(math.MaxFloat32)
		}},
		{"height sum overflow", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) {
			s.Fixed.Height = lenPx(math.MaxFloat32)
			s.Padding.Bottom = lenPx(math.MaxFloat32)
		}},
		{"origin overflow", func(s *ComputedStyle, _ *Rect, _ *FixedLayoutOptions) {
			s.Fixed.Insets.Left = lenPx(math.MaxFloat32)
			s.Margin.Left = lenPx(math.MaxFloat32)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, viewport, opts := definiteFixedStyle(), Rect{W: 300, H: 200}, FixedLayoutOptions{}
			tc.edit(&s, &viewport, &opts)
			g, err := ResolveFixedGeometry(s, viewport, opts)
			if g != (LayoutGeometry{}) || !errors.Is(err, ErrInvalidFixedGeometry) || errors.Is(err, ErrFixedContentDependency) || errors.Is(err, ErrUnsupportedPosition) {
				t.Fatalf("invalid input returned %+v, %v", g, err)
			}
			if tc.name == "bad height" && !errors.Is(err, ErrInvalidPosition) {
				t.Fatal("underlying validation cause was lost")
			}
		})
	}
}

func TestResolveFixedGeometryReplayAndConcurrency(t *testing.T) {
	s := definiteFixedStyle()
	s.Width, s.Fixed.Height = lenAuto(), lenAuto()
	s.Fixed.Insets = edges(lenPx(10))
	before, fixedBefore := s, *s.Fixed
	first, err := ResolveFixedGeometry(s, Rect{W: 300, H: 200}, FixedLayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g, err := ResolveFixedGeometry(s, Rect{W: 100, H: 80}, FixedLayoutOptions{})
			want := LayoutGeometry{Frame: Rect{10, 10, 80, 60}, Content: Rect{10, 10, 80, 60}}
			if err != nil || g != want {
				t.Errorf("concurrent resize: %+v %v", g, err)
			}
		}()
	}
	wg.Wait()
	if _, err := ResolveFixedGeometry(s, Rect{W: -1}, FixedLayoutOptions{}); err == nil {
		t.Fatal("expected error")
	}
	again, err := ResolveFixedGeometry(s, Rect{W: 300, H: 200}, FixedLayoutOptions{})
	if err != nil || again != first || s != before || *s.Fixed != fixedBefore {
		t.Fatal("replay/failure changed input or previous output")
	}
	s.Fixed.Insets.Top = lenPx(99)
	if first.Frame != (Rect{10, 10, 280, 180}) {
		t.Fatal("output retained an input alias")
	}
}

func TestFixedTreePreparationStillRejectsVerticalOverflowBeforeMeasurement(t *testing.T) {
	s := anchoredFixedStyle()
	s.Fixed.Height = lenPx(math.MaxFloat32)
	s.Padding.Bottom = lenPx(math.MaxFloat32)
	root := fixedSizingRoot(t, s, newRenderText(2, "x"))
	before := copyTestTree(root)
	calls := 0
	got, err := resolveFixedSizing(root, Rect{W: 300, H: 200}, 0, measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) { calls++; return IntrinsicWidths{10, 20}, nil }))
	if got != nil || err == nil || calls != 0 || !reflect.DeepEqual(root, before) {
		t.Fatalf("validation moved after measurement: %+v %v calls=%d", got, err, calls)
	}
}
