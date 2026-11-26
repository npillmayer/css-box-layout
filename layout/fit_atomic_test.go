package layout

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"
)

// One atomic per line. Ordinary interior text consumes one ten-pixel line;
// its output records the exact width under which it was realized.
func fitAtomicParagraphs(inspect func(ParagraphFitRequest)) fitTestService {
	return func(_ context.Context, req ParagraphFitRequest, yield func(ParagraphAlternative) bool) (FitEnumeration, error) {
		if inspect != nil {
			inspect(req)
		}
		if len(req.Atomics) == 0 {
			out := &fitTestPayload{Bytes: []byte(fmt.Sprintf("at-%g", req.Width))}
			ok := yield(ParagraphAlternative{Complete: true, End: ParagraphPosition{Offset: 1}, Extent: InlineExtent{W: req.Width, H: 10}, Output: out})
			out.Bytes[0] = '!'
			if !ok {
				return FitStopped, nil
			}
			return FitExhausted, nil
		}
		var placements []AtomicPlacement
		var height, width float32
		for i, item := range req.Atomics {
			if item.Consumed {
				continue
			}
			placements = append(placements, AtomicPlacement{BoxID: item.BoxID, Y: height})
			height += item.BorderHeight
			width = max(width, item.BorderWidth)
			a := ParagraphAlternative{Complete: i == len(req.Atomics)-1, End: ParagraphPosition{Offset: uint64(i + 1)}, Extent: InlineExtent{width, height}, Atomics: placements, Output: &fitTestPayload{Bytes: []byte(fmt.Sprintf("through-%d", i+1))}}
			if !a.Complete {
				a.Checkpoint = &fitTestPayload{End: uint64(i + 1)}
			}
			if !yield(a) {
				return FitStopped, nil
			}
		}
		return FitExhausted, nil
	}
}

func twoFitAtomics(t *testing.T) (*LayoutNode, []InlineGroup) {
	t.Helper()
	a := newRenderElement(2, inlineBlockDisplay, newRenderText(3, "first"))
	b := newRenderElement(4, inlineBlockDisplay, newRenderText(5, "second"))
	s := DefaultComputedStyle()
	s.Width = lenPct(.5)
	a.Style = &s
	b.Style = &s
	root := buildGroupFixture(t, newRenderElement(1, blockDisplay, a, b))
	return root, slices.Collect(InlineGroups(root))
}

func TestStructuralFitAtomicContinuationLocksAcceptedInterior(t *testing.T) {
	root, groups := twoFitAtomics(t)
	before := copyTestTree(root)
	counts := map[BoxID]int{}
	resuming := false
	service := fitAtomicParagraphs(func(req ParagraphFitRequest) {
		counts[req.Owner]++
		if req.Group == groups[0].Group.BoxID && resuming {
			if len(req.Atomics) != 2 || !req.Atomics[0].Consumed || req.Atomics[1].Consumed || req.Atomics[0].BorderWidth != 50 || req.Atomics[1].BorderWidth != 40 {
				t.Fatalf("locked/new inputs %+v", req.Atomics)
			}
			u, _ := req.Used.Lookup(req.Atomics[0].BoxID)
			if u.ContentWidth != 50 {
				t.Fatal("locked used values", u)
			}
		}
	})
	f, err := NewStructuralFitter(root, service)
	if err != nil {
		t.Fatal(err)
	}
	prefix := collectFits(t, f, FitRequest{Width: 100, Height: 10})[0]
	if prefix.Complete() || len(prefix.Atomics()) != 1 {
		t.Fatal("atomic split")
	}
	first := prefix.Atomics()[0]
	if first.Item().BorderWidth != 50 || first.Item().BorderHeight != 10 || string(first.Paragraphs()[0].Output.(*fitTestPayload).Bytes) != "at-50" {
		t.Fatal("prefix interior", first.Item(), first.Paragraphs())
	}
	resuming = true
	suffix := collectFits(t, f, FitRequest{Width: 80, Height: 10, Continuation: prefix.Continuation()})[0]
	if !suffix.Complete() || len(suffix.Atomics()) != 1 || suffix.Atomics()[0].Item().BorderWidth != 40 || string(suffix.Atomics()[0].Paragraphs()[0].Output.(*fitTestPayload).Bytes) != "at-40" {
		t.Fatal("suffix realization")
	}
	if counts[groups[1].Owner.BoxID] != 1 || counts[groups[2].Owner.BoxID] != 2 {
		t.Fatal("consumed atomic re-realized", counts)
	}
	if _, ok := suffix.Geometry()[first.Item().BoxID]; ok {
		t.Fatal("consumed prefix republished")
	}
	// The input, inspection copies and a failed subsequent call cannot affect
	// already accepted geometry/paragraph output.
	g := first.Geometry()
	delete(g, first.Item().BoxID)
	u := first.UsedValues()
	delete(u, first.Item().BoxID)
	p := first.Paragraphs()
	p[0].Output.(*fitTestPayload).Bytes[0] = '?'
	if string(first.Paragraphs()[0].Output.(*fitTestPayload).Bytes) != "at-50" || first.Geometry()[first.Item().BoxID].Frame.W != 50 {
		t.Fatal("mutable atomic inspection")
	}
	if !reflect.DeepEqual(before, root) {
		t.Fatal("source tree changed")
	}
	if _, err := f.Boundary(groups[1].Group.BoxID, BoundaryBefore); !errors.Is(err, ErrUnsupportedFit) {
		t.Fatal("atomic split permitted", err)
	}
	// Losing the fitter and caller tree leaves frozen inspection self-contained.
	f = nil
	root = nil
	if first.Paragraphs()[0].Extent.W != 50 || suffix.Atomics()[0].Geometry()[groups[2].Owner.BoxID].Frame.W != 40 {
		t.Fatal("retained atomic lifetime")
	}
}

func TestStructuralFitOversizedAtomicRetriesAndZeroSizeProgress(t *testing.T) {
	for _, size := range []struct{ w, h float32 }{{120, 20}, {0, 0}} {
		a := newRenderElement(2, inlineBlockDisplay)
		s := DefaultComputedStyle()
		s.Width = lenPx(size.w)
		a.Style = &s
		a.Replaced = &ReplacedContent{Height: size.h}
		root := buildGroupFixture(t, newRenderElement(1, blockDisplay, a))
		f, err := NewStructuralFitter(root, fitAtomicParagraphs(nil))
		if err != nil {
			t.Fatal(err)
		}
		all := collectFits(t, f, FitRequest{Width: 100, Height: 30, IncludeEmpty: true})
		if size.w > 100 && (len(all) != 1 || all[0].Progress()) {
			t.Fatal("oversized atomic consumed")
		}
		full := collectFits(t, f, FitRequest{Width: 140, Height: 30, Continuation: all[len(all)-1].Continuation()})[0]
		if !full.Progress() || !full.Complete() || len(full.Atomics()) != 1 || full.Atomics()[0].Item().BorderWidth != size.w || full.Atomics()[0].Item().BorderHeight != size.h {
			t.Fatal("retry/zero atomic", full.Extent(), full.Atomics())
		}
	}
}

func TestStructuralFitNestedAtomicsAndFailureIsolation(t *testing.T) {
	root, bySource := recursiveFixture(t)
	boom := errors.New("interior backend failed")
	failing := false
	base := fitAtomicParagraphs(nil)
	service := fitTestService(func(ctx context.Context, req ParagraphFitRequest, yield func(ParagraphAlternative) bool) (FitEnumeration, error) {
		if failing && req.Owner == bySource[5].BoxID {
			return FitFailed, boom
		}
		return base(ctx, req, yield)
	})
	f, err := NewStructuralFitter(root, service)
	if err != nil {
		t.Fatal(err)
	}
	a := collectFits(t, f, FitRequest{Width: 100, Height: 500})[0]
	if !a.Complete() || len(a.Atomics()) != 1 {
		t.Fatal("nested completion")
	}
	outer := a.Atomics()[0]
	innerID := bySource[5].BoxID
	if outer.Geometry()[innerID].RelativeTo != bySource[4].BoxID || outer.Geometry()[innerID].Frame.W != 12 || len(outer.Paragraphs()) != 4 {
		t.Fatal("nested geometry", outer.Geometry(), outer.Paragraphs())
	}
	old := a.Geometry()
	failing = true
	status, err := f.Enumerate(context.Background(), FitRequest{Width: 80, Height: 500}, func(FitAlternative) bool { t.Fatal("failed interior published"); return true })
	if status != FitFailed || !errors.Is(err, boom) || !reflect.DeepEqual(old, a.Geometry()) {
		t.Fatal("failure isolation", status, err)
	}
}

func TestStructuralFitAtomicPlacementProtocol(t *testing.T) {
	for _, mode := range []string{"missing", "duplicate", "skip", "consumed", "nan", "overflow"} {
		t.Run(mode, func(t *testing.T) {
			root, groups := twoFitAtomics(t)
			base := fitAtomicParagraphs(nil)
			bad := false
			service := fitTestService(func(ctx context.Context, req ParagraphFitRequest, yield func(ParagraphAlternative) bool) (FitEnumeration, error) {
				if !bad || req.Group != groups[0].Group.BoxID {
					return base(ctx, req, yield)
				}
				a := ParagraphAlternative{Complete: true, End: ParagraphPosition{Offset: 2}, Extent: InlineExtent{W: 1, H: 1}, Output: &fitTestPayload{}, Atomics: []AtomicPlacement{{BoxID: req.Atomics[0].BoxID}, {BoxID: req.Atomics[1].BoxID}}}
				switch mode {
				case "missing":
					a.Atomics = nil
				case "duplicate":
					a.Atomics[1] = a.Atomics[0]
				case "skip":
					a.Complete = false
					a.Checkpoint = &fitTestPayload{}
					a.Atomics = a.Atomics[1:]
				case "nan":
					a.Atomics[0].X = float32(math.NaN())
				case "overflow":
					a.Atomics[0].Y = 100
				}
				if !yield(a) {
					return FitStopped, nil
				}
				return FitExhausted, nil
			})
			f, err := NewStructuralFitter(root, service)
			if err != nil {
				t.Fatal(err)
			}
			prefix := collectFits(t, f, FitRequest{Width: 100, Height: 10})[0]
			bad = true
			req := FitRequest{Width: 100, Height: 20}
			if mode == "consumed" {
				req.Continuation = prefix.Continuation()
			}
			status, err := f.Enumerate(context.Background(), req, func(FitAlternative) bool { t.Fatal("invalid placement published"); return true })
			if mode == "overflow" {
				if status != FitExhausted || err != nil {
					t.Fatal(status, err)
				}
			} else if status != FitFailed || !errors.Is(err, ErrFitProtocol) {
				t.Fatal(status, err)
			}
			if prefix.Atomics()[0].Item().BorderWidth != 50 {
				t.Fatal("failed retry corrupted prefix")
			}
		})
	}
}

func TestStructuralFitAutoAtomicsAndInterruptedInteriors(t *testing.T) {
	root, groups := twoFitAtomics(t)
	for _, g := range groups[1:] {
		g.Owner.Style.Width = Length{Kind: LenAuto}
	}
	measurements := 0
	measure := measureFunc(func(InlineMeasureRequest) (IntrinsicWidths, error) {
		measurements++
		return IntrinsicWidths{20, 100}, nil
	})
	f, err := NewStructuralFitterWithOptions(root, fitAtomicParagraphs(nil), StructuralFitOptions{Intrinsic: measure})
	if err != nil {
		t.Fatal(err)
	}
	prefix := collectFits(t, f, FitRequest{Width: 90, Height: 10})[0]
	suffix := collectFits(t, f, FitRequest{Width: 60, Height: 10, Continuation: prefix.Continuation()})[0]
	if measurements != 3 || prefix.Atomics()[0].Item().BorderWidth != 90 || suffix.Atomics()[0].Item().BorderWidth != 60 {
		t.Fatal("shrink-to-fit/reuse", measurements)
	}
	without, _ := NewStructuralFitter(root, fitAtomicParagraphs(nil))
	status, err := without.Enumerate(context.Background(), FitRequest{Width: 100, Height: 30}, func(FitAlternative) bool { t.Fatal("missing intrinsic service accepted"); return true })
	if status != FitFailed || !errors.Is(err, ErrInvalidFit) {
		t.Fatal(status, err)
	}
	for _, g := range groups[1:] {
		g.Owner.Style.Width = lenPx(20)
	}
	for _, cancel := range []bool{false, true} {
		ctx, stop := context.WithCancel(context.Background())
		service := fitTestService(func(context.Context, ParagraphFitRequest, func(ParagraphAlternative) bool) (FitEnumeration, error) {
			if cancel {
				stop()
			}
			return FitStopped, nil
		})
		f, _ := NewStructuralFitter(root, service)
		status, err := f.Enumerate(ctx, FitRequest{Width: 100, Height: 30}, func(FitAlternative) bool { t.Fatal("interrupted interior accepted"); return true })
		stop()
		if cancel {
			if status != FitFailed || !errors.Is(err, context.Canceled) {
				t.Fatal(status, err)
			}
		} else if status != FitStopped || err != nil {
			t.Fatal(status, err)
		}
	}
}

func TestStructuralFitAtomicProvenanceOrdinals(t *testing.T) {
	a := newRenderElement(3, inlineBlockDisplay, newRenderElement(4, blockDisplay, newRenderText(5, "inside")))
	s := DefaultComputedStyle()
	s.Width = lenPx(30)
	a.Style = &s
	input := newRenderElement(1, blockDisplay, newRenderElement(2, inlineDisplay, a), newRenderElement(6, blockDisplay, newRenderText(7, "tail")))
	trees, provenance, err := BuildLayoutTreesWithProvenance(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	f, err := NewStructuralFitter(trees.Flow, fitAtomicParagraphs(nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range provenance.Boundaries() {
		if got := f.program.all[fitEdge{b.Box, b.Edge}]; got != b {
			t.Fatalf("provenance %+v != %+v", got, b)
		}
	}
}

func TestStructuralFitEmptyPrefixBesideAtomic(t *testing.T) {
	a := newRenderElement(2, inlineBlockDisplay)
	s := DefaultComputedStyle()
	s.Width = lenPx(5)
	a.Style, a.Replaced = &s, &ReplacedContent{Height: 10}
	root := buildGroupFixture(t, newRenderElement(1, blockDisplay, a))
	service := fitTestService(func(_ context.Context, req ParagraphFitRequest, yield func(ParagraphAlternative) bool) (FitEnumeration, error) {
		if req.From.Offset == 0 {
			if !yield(ParagraphAlternative{End: ParagraphPosition{Offset: 1}, Checkpoint: &fitTestPayload{}, Output: &fitTestPayload{}, AllowsMarginCollapse: true}) {
				return FitStopped, nil
			}
		}
		if !yield(ParagraphAlternative{End: ParagraphPosition{Offset: 2}, Complete: true, Extent: InlineExtent{5, 10}, Atomics: []AtomicPlacement{{BoxID: req.Atomics[0].BoxID}}, Output: &fitTestPayload{}}) {
			return FitStopped, nil
		}
		return FitExhausted, nil
	})
	f, err := NewStructuralFitter(root, service)
	if err != nil {
		t.Fatal(err)
	}
	prefix := collectFits(t, f, FitRequest{Width: 20, Height: 0})[0]
	if !prefix.Progress() || prefix.Extent().H != 0 || len(prefix.Atomics()) != 0 {
		t.Fatal("empty logical prefix did not fit")
	}
	suffix := collectFits(t, f, FitRequest{Width: 20, Height: 10, Continuation: prefix.Continuation()})[0]
	if !suffix.Complete() || len(suffix.Atomics()) != 1 {
		t.Fatal("atomic after empty prefix lost")
	}
}
