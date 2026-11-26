package layout

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

type fitTestPayload struct {
	Group BoxID
	End   uint64
	Bytes []byte
}

func (p *fitTestPayload) Clone() FitPayload {
	q := *p
	q.Bytes = slices.Clone(p.Bytes)
	return &q
}

type fitTestService func(context.Context, ParagraphFitRequest, func(ParagraphAlternative) bool) (FitEnumeration, error)

func (f fitTestService) EnumerateParagraph(ctx context.Context, req ParagraphFitRequest, yield func(ParagraphAlternative) bool) (FitEnumeration, error) {
	return f(ctx, req, yield)
}

// Every byte is one indivisible unit, Width units fit a line, and every line
// occupies one pixel. Empty groups advance structurally at zero height. The
// service intentionally reuses/mutates payloads immediately after each yield.
func fitTestParagraphs(material map[BoxID]string) fitTestService {
	return func(ctx context.Context, req ParagraphFitRequest, yield func(ParagraphAlternative) bool) (FitEnumeration, error) {
		if req.From != (ParagraphPosition{}) {
			c, ok := req.Checkpoint.(*fitTestPayload)
			if !ok || c.Group != req.Group || c.End != req.From.Offset {
				return FitFailed, fmt.Errorf("lost checkpoint association")
			}
			// A borrowed checkpoint must not be able to change a retained cut.
			c.End = math.MaxUint64
		}
		body := material[req.Group]
		start := int(req.From.Offset)
		if len(body) > start && req.Width < 1 {
			return FitExhausted, nil
		}
		capacity := max(1, int(req.Width))
		lines := 0
		for end := start; ; {
			if end < len(body) {
				end = min(end+capacity, len(body))
				lines++
			}
			out := &fitTestPayload{Group: req.Group, End: uint64(end), Bytes: []byte(body[start:end])}
			a := ParagraphAlternative{End: ParagraphPosition{Offset: uint64(end)}, Complete: end == len(body),
				Extent: InlineExtent{W: float32(min(end-start, capacity)), H: float32(lines)}, Output: out}
			var checkpoint *fitTestPayload
			if !a.Complete {
				checkpoint = &fitTestPayload{Group: req.Group, End: uint64(end)}
				a.Checkpoint = checkpoint
			}
			ok := yield(a)
			for i := range out.Bytes {
				out.Bytes[i] = '?'
			}
			if checkpoint != nil {
				checkpoint.End = math.MaxUint64
			}
			if !ok {
				return FitStopped, nil
			}
			if a.Complete {
				return FitExhausted, nil
			}
		}
	}
}

func fitTestTree(t *testing.T) (*LayoutNode, []InlineGroup, map[BoxID]string) {
	t.Helper()
	root := buildGroupFixture(t, newRenderElement(1, blockDisplay,
		newRenderElement(2, blockDisplay,
			newRenderElement(3, blockDisplay, newRenderText(4, "abc")),
			newRenderElement(5, blockDisplay,
				newRenderElement(6, blockDisplay, newRenderText(7, "de")),
				newRenderElement(8, blockDisplay))),
		newRenderElement(9, blockDisplay, newRenderText(10, "fgh"))))
	groups := slices.Collect(InlineGroups(root))
	material := map[BoxID]string{}
	for i, body := range []string{"abc", "de", "", "fgh"} {
		material[groups[i].Group.BoxID] = body
	}
	return root, groups, material
}

func collectFits(t *testing.T, f *StructuralFitter, req FitRequest) []FitAlternative {
	t.Helper()
	var out []FitAlternative
	status, err := f.Enumerate(context.Background(), req, func(a FitAlternative) bool { out = append(out, a); return true })
	if err != nil || status != FitExhausted {
		t.Fatalf("enumeration = %v, %v", status, err)
	}
	return out
}

func fitText(a FitAlternative) string {
	var out strings.Builder
	for _, p := range a.Paragraphs() {
		out.Write(p.Output.(*fitTestPayload).Bytes)
	}
	return out.String()
}

func TestStructuralFitEveryCutConservesMaterial(t *testing.T) {
	root, _, material := fitTestTree(t)
	before := copyTestTree(root)
	f, err := NewStructuralFitter(root, fitTestParagraphs(material))
	if err != nil {
		t.Fatal(err)
	}
	all := collectFits(t, f, FitRequest{Width: 2, Height: 10, IncludeEmpty: true})
	if len(all) != 7 {
		t.Fatalf("got %d alternatives, want every line/block cut and empty", len(all))
	}
	for i, want := range []string{"abcdefgh", "abcdefg", "abcde", "abcde", "abc", "ab", ""} {
		a := all[i]
		if got := fitText(a); got != want {
			t.Fatalf("alternative %d = %q, want %q", i, got, want)
		}
		if a.Progress() != (i < 6) || a.Complete() != (i == 0) {
			t.Fatalf("progress/completion %d", i)
		}
		if i > 0 {
			if c, ok := all[i-1].End().Compare(a.End()); !ok || c <= 0 {
				t.Fatalf("logical order %d", i)
			}
		}
		// Choosing any alternative, including empty, must permit a new width
		// without moving or reflowing that accepted prefix.
		suffix := collectFits(t, f, FitRequest{Width: 1, Height: 20, Continuation: a.Continuation()})
		if len(suffix) == 0 || !suffix[0].Complete() || fitText(a)+fitText(suffix[0]) != "abcdefgh" {
			t.Fatalf("cut %d did not conserve material", i)
		}
		if got := fitText(a); got != want {
			t.Fatalf("resume mutated prefix %d: %q", i, got)
		}
		// Mutating inspection data must change neither another accessor nor
		// the continuation used on the next speculative attempt.
		for _, p := range a.Paragraphs() {
			for j := range p.Output.(*fitTestPayload).Bytes {
				p.Output.(*fitTestPayload).Bytes[j] = '!'
			}
		}
		info := a.Continuation().Info()
		for j := range info.Open {
			info.Open[j].NextChild = 999
		}
		collectFits(t, f, FitRequest{Width: 3, Height: 20, Continuation: a.Continuation()})
		if fitText(a) != want {
			t.Fatalf("inspection mutated prefix %d", i)
		}
	}
	if !reflect.DeepEqual(before, root) {
		t.Fatal("fitting changed source tree")
	}
	// Repeated shortest-prefix acceptance must terminate, not replay a group.
	var cursor FitContinuation
	var pieces string
	for step := 0; step < 20; step++ {
		choices := collectFits(t, f, FitRequest{Width: 1, Height: 10, Continuation: cursor})
		a := choices[len(choices)-1]
		if !a.Progress() {
			t.Fatal("non-advancing suffix")
		}
		pieces += fitText(a)
		cursor = a.Continuation()
		if a.Complete() {
			break
		}
	}
	if !cursor.Info().Complete || pieces != "abcdefgh" {
		t.Fatalf("multiple resumes: %q, %+v", pieces, cursor.Info())
	}
}

func TestStructuralFitNestedContinuationAndStops(t *testing.T) {
	root, groups, material := fitTestTree(t)
	f, err := NewStructuralFitter(root, fitTestParagraphs(material))
	if err != nil {
		t.Fatal(err)
	}
	partial := collectFits(t, f, FitRequest{Width: 2, Height: 1})[0]
	info := partial.Continuation().Info()
	if info.Next != groups[0].Group.BoxID || len(info.Open) != 3 {
		t.Fatalf("partial cursor: %+v", info)
	}
	for _, c := range info.Open {
		if c.NextChild != 0 || c.OpeningPending || !c.ClosingPending {
			t.Fatalf("pending state: %+v", c)
		}
	}
	stop, err := f.Boundary(groups[1].Owner.BoxID, BoundaryAfter)
	if err != nil {
		t.Fatal(err)
	}
	a := collectFits(t, f, FitRequest{Width: 2, Height: 20, Continuation: partial.Continuation(), Stop: stop})[0]
	if !a.StopReached() || a.Complete() || fitText(a) != "cde" {
		t.Fatalf("stop: %q", fitText(a))
	}
	for _, c := range a.Containers() {
		if c.Box == groups[0].Owner.BoxID && (c.First || !c.Last) {
			t.Fatalf("resumed last: %+v", c)
		}
		if c.Box == groups[1].Owner.BoxID && (!c.First || !c.Last) {
			t.Fatalf("contained whole child: %+v", c)
		}
		if c.Box == root.BoxID && (c.First || c.Last) {
			t.Fatalf("middle root: %+v", c)
		}
	}
	beforeGroup, _ := f.Boundary(groups[0].Group.BoxID, BoundaryBefore)
	empty := collectFits(t, f, FitRequest{Width: 0, Height: 0, Stop: beforeGroup})
	if len(empty) != 1 || empty[0].Progress() || !empty[0].StopReached() || len(empty[0].Containers()) != 0 {
		t.Fatal("stop before first material")
	}
	for _, c := range empty[0].Continuation().Info().Open {
		if !c.OpeningPending {
			t.Fatal("empty stop consumed opening")
		}
	}
	full := collectFits(t, f, FitRequest{Width: 10, Height: 10, Continuation: empty[0].Continuation()})[0]
	if !full.Complete() || fitText(full) != "abcdefgh" {
		t.Fatal("resume at opening")
	}
	for _, c := range full.Containers() {
		if !c.First || !c.Last {
			t.Fatalf("whole container: %+v", c)
		}
	}
	// Boundary ordinals use normalized traversal, not BoxID arithmetic.
	input := newRenderElement(1, blockDisplay, newRenderText(2, "abc"))
	_, provenance, err := BuildLayoutTreesWithProvenance(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Compare against an independently normalized identical input.
	plain := buildGroupFixture(t, input)
	g, err := NewStructuralFitter(plain, fitTestParagraphs(nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range provenance.Boundaries() {
		cut, err := g.Boundary(b.Box, b.Edge)
		if err == nil && cut.Info() != b {
			t.Fatalf("boundary: %+v != %+v", cut.Info(), b)
		}
	}
	// Every supported structural stop conserves the full input on resumption,
	// including before empty groups and after a last child closes its ancestors.
	var checkStops func(*LayoutNode)
	checkStops = func(n *LayoutNode) {
		if IsBlockLevel(n.Box) || n.Box == BoxAnonymousInline {
			for _, edge := range []BoundaryEdge{BoundaryBefore, BoundaryAfter} {
				stop, err := f.Boundary(n.BoxID, edge)
				if err != nil {
					t.Fatal(err)
				}
				prefix := collectFits(t, f, FitRequest{Width: 2, Height: 20, Stop: stop})[0]
				suffix := collectFits(t, f, FitRequest{Width: 1, Height: 20, Continuation: prefix.Continuation()})[0]
				if !prefix.StopReached() || !suffix.Complete() || fitText(prefix)+fitText(suffix) != "abcdefgh" {
					t.Fatalf("stop box %d edge %v lost material", n.BoxID, edge)
				}
			}
		}
		for _, child := range n.Children {
			checkStops(child)
		}
	}
	checkStops(root)
}

func TestStructuralFitZeroHeightAndNonfit(t *testing.T) {
	root := buildGroupFixture(t, newRenderElement(1, blockDisplay))
	f, _ := NewStructuralFitter(root, fitTestParagraphs(nil))
	empty := collectFits(t, f, FitRequest{})
	if len(empty) != 1 || !empty[0].Progress() || !empty[0].Complete() || empty[0].Extent() != (InlineExtent{}) {
		t.Fatal("empty group must advance at zero height")
	}
	done := collectFits(t, f, FitRequest{Continuation: empty[0].Continuation()})
	if len(done) != 1 || done[0].Progress() || !done[0].Complete() {
		t.Fatal("already completed")
	}
	root = buildGroupFixture(t, newRenderElement(1, blockDisplay, newRenderText(2, "abcd")))
	group := slices.Collect(InlineGroups(root))[0].Group.BoxID
	f, _ = NewStructuralFitter(root, fitTestParagraphs(map[BoxID]string{group: "abcd"}))
	for _, h := range []float32{0, .999} {
		if got := collectFits(t, f, FitRequest{Width: 2, Height: h}); len(got) != 0 {
			t.Fatal("oversized prefix accepted")
		}
		a := collectFits(t, f, FitRequest{Width: 2, Height: h, IncludeEmpty: true})
		if len(a) != 1 || a[0].Progress() {
			t.Fatal("no-fit changed cursor")
		}
	}
	if a := collectFits(t, f, FitRequest{Width: 2, Height: 1}); len(a) != 1 || fitText(a[0]) != "ab" || a[0].Extent().H != 1 {
		t.Fatal("exact height")
	}
	if a := collectFits(t, f, FitRequest{Width: 2, Height: 2}); len(a) != 2 || !a[0].Complete() {
		t.Fatal("retry after non-fit")
	}
	// Backend reports an indivisible oversized object; core must filter it.
	f, _ = NewStructuralFitter(root, fitTestService(func(_ context.Context, _ ParagraphFitRequest, yield func(ParagraphAlternative) bool) (FitEnumeration, error) {
		if !yield(ParagraphAlternative{Complete: true, End: ParagraphPosition{Offset: 4}, Extent: InlineExtent{W: 3, H: 1}, Output: &fitTestPayload{}}) {
			return FitStopped, nil
		}
		return FitExhausted, nil
	}))
	if len(collectFits(t, f, FitRequest{Width: 2, Height: 10})) != 0 {
		t.Fatal("oversized width")
	}
	// Nonempty material can also advance at zero height. Extent is never a
	// substitute for logical progress or completion.
	f, _ = NewStructuralFitter(root, fitTestService(func(_ context.Context, req ParagraphFitRequest, yield func(ParagraphAlternative) bool) (FitEnumeration, error) {
		a := ParagraphAlternative{End: ParagraphPosition{Offset: 4}, Complete: true, Output: &fitTestPayload{Bytes: []byte("abcd")}}
		if !yield(a) {
			return FitStopped, nil
		}
		return FitExhausted, nil
	}))
	zero := collectFits(t, f, FitRequest{})
	if len(zero) != 1 || !zero[0].Progress() || !zero[0].Complete() || fitText(zero[0]) != "abcd" {
		t.Fatal("zero-height nonempty material")
	}
}

func TestStructuralFitRequestQualification(t *testing.T) {
	root, groups, material := fitTestTree(t)
	f, _ := NewStructuralFitter(root, fitTestParagraphs(material))
	other, _ := NewStructuralFitter(root, fitTestParagraphs(material))
	a := collectFits(t, f, FitRequest{Width: 2, Height: 1})[0]
	foreign := collectFits(t, other, FitRequest{Width: 2, Height: 1})[0]
	stop, _ := f.Boundary(groups[0].Group.BoxID, BoundaryBefore)
	foreignStop, _ := other.Boundary(root.BoxID, BoundaryAfter)
	for name, req := range map[string]FitRequest{
		"foreign cursor": {Continuation: foreign.Continuation()}, "foreign stop": {Stop: foreignStop},
		"behind partial": {Continuation: a.Continuation(), Stop: stop},
		"negative width": {Width: -1}, "negative height": {Height: -1},
		"nan": {Width: float32(math.NaN())}, "infinity": {Height: float32(math.Inf(1))},
	} {
		t.Run(name, func(t *testing.T) {
			status, err := f.Enumerate(context.Background(), req, func(FitAlternative) bool { t.Fatal("invalid request yielded"); return true })
			if status != FitFailed || !errors.Is(err, ErrInvalidFit) {
				t.Fatalf("%v %v", status, err)
			}
		})
	}
	if _, err := f.Boundary(groups[0].Group.Children[0].BoxID, BoundaryBefore); !errors.Is(err, ErrUnsupportedFit) {
		t.Fatal(err)
	}
	if _, err := f.Boundary(99999, BoundaryBefore); !errors.Is(err, ErrInvalidFit) {
		t.Fatal(err)
	}
	if _, ok := a.End().Compare(foreign.End()); ok {
		t.Fatal("foreign positions comparable")
	}
}

func TestStructuralFitInterruptionsAndLateErrors(t *testing.T) {
	root, groups, material := fitTestTree(t)
	f, _ := NewStructuralFitter(root, fitTestParagraphs(material))
	var retained FitAlternative
	status, err := f.Enumerate(context.Background(), FitRequest{Width: 2, Height: 10}, func(a FitAlternative) bool { retained = a; return false })
	if status != FitStopped || err != nil || !retained.Complete() {
		t.Fatalf("early stop %v %v", status, err)
	}
	for limit := uint64(1); limit < 12; limit++ {
		status, err = f.Enumerate(context.Background(), FitRequest{Width: 2, Height: 10, MaxSteps: limit}, func(FitAlternative) bool { return true })
		if status != FitFailed || !errors.Is(err, ErrFitLimit) {
			t.Fatalf("limit %d = %v %v", limit, status, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	status, err = f.Enumerate(ctx, FitRequest{}, func(FitAlternative) bool { t.Fatal("cancel yielded"); return true })
	if status != FitFailed || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel %v %v", status, err)
	}
	boom := errors.New("late paragraph failure")
	base := fitTestParagraphs(material)
	// Fail only after one full alternative has been published: the first
	// group offers two complete bindings, the second of which fails downstream.
	calls := 0
	late := fitTestService(func(ctx context.Context, req ParagraphFitRequest, yield func(ParagraphAlternative) bool) (FitEnumeration, error) {
		if req.Group == groups[0].Group.BoxID {
			return base(ctx, req, func(a ParagraphAlternative) bool {
				if !yield(a) {
					return false
				}
				if a.Complete {
					return yield(a)
				}
				return true
			})
		}
		if req.Group == groups[1].Group.BoxID {
			calls++
			if calls > 1 {
				return FitFailed, boom
			}
		}
		return base(ctx, req, yield)
	})
	g, _ := NewStructuralFitter(root, late)
	var earlier []FitAlternative
	status, err = g.Enumerate(context.Background(), FitRequest{Width: 2, Height: 10}, func(a FitAlternative) bool { earlier = append(earlier, a); return true })
	var located *FitError
	if status != FitFailed || !errors.Is(err, boom) || !errors.As(err, &located) || located.Box != groups[1].Group.BoxID || len(earlier) == 0 {
		t.Fatalf("late error %v %v; outputs %d", status, err, len(earlier))
	}
	if fitText(earlier[0]) != "abcdefgh" || fitText(retained) != "abcdefgh" {
		t.Fatal("late error invalidated earlier output")
	}
	// Cancellation during a synchronous service call is observed on return.
	ctx, cancel = context.WithCancel(context.Background())
	g, _ = NewStructuralFitter(root, fitTestService(func(context.Context, ParagraphFitRequest, func(ParagraphAlternative) bool) (FitEnumeration, error) {
		cancel()
		return FitExhausted, nil
	}))
	status, err = g.Enumerate(ctx, FitRequest{Width: 2, Height: 10}, func(FitAlternative) bool { return true })
	if status != FitFailed || !errors.Is(err, context.Canceled) {
		t.Fatalf("late cancel %v %v", status, err)
	}
}

func TestStructuralFitProtocol(t *testing.T) {
	root := buildGroupFixture(t, newRenderElement(1, blockDisplay))
	for name, a := range map[string]ParagraphAlternative{
		"no progress":         {Checkpoint: &fitTestPayload{}, Output: &fitTestPayload{}},
		"no checkpoint":       {End: ParagraphPosition{Offset: 1}, Output: &fitTestPayload{}},
		"complete checkpoint": {Complete: true, Checkpoint: &fitTestPayload{}, Output: &fitTestPayload{}},
		"no output":           {Complete: true},
		"negative extent":     {Complete: true, Extent: InlineExtent{H: -1}, Output: &fitTestPayload{}},
		"infinite extent":     {Complete: true, Extent: InlineExtent{W: float32(math.Inf(1))}, Output: &fitTestPayload{}},
	} {
		t.Run(name, func(t *testing.T) {
			f, _ := NewStructuralFitter(root, fitTestService(func(_ context.Context, _ ParagraphFitRequest, yield func(ParagraphAlternative) bool) (FitEnumeration, error) {
				if !yield(a) {
					return FitStopped, nil
				}
				return FitExhausted, nil
			}))
			status, err := f.Enumerate(context.Background(), FitRequest{Width: 2, Height: 2}, func(FitAlternative) bool { t.Fatal("invalid result published"); return true })
			if status != FitFailed || !errors.Is(err, ErrFitProtocol) {
				t.Fatalf("%v %v", status, err)
			}
		})
	}
	for _, result := range []FitEnumeration{FitStopped, FitFailed, 0} {
		f, _ := NewStructuralFitter(root, fitTestService(func(context.Context, ParagraphFitRequest, func(ParagraphAlternative) bool) (FitEnumeration, error) {
			return result, nil
		}))
		status, err := f.Enumerate(context.Background(), FitRequest{}, func(FitAlternative) bool { return true })
		if result == FitStopped {
			if status != FitStopped || err != nil {
				t.Fatal(status, err)
			}
		} else if status != FitFailed || !errors.Is(err, ErrFitProtocol) {
			t.Fatal(status, err)
		}
	}
}

func TestStructuralFitProfileAndConcurrentReuse(t *testing.T) {
	rootPositioned := buildGroupFixture(t, newRenderElement(1, blockDisplay, newRenderText(2, "a")))
	style := DefaultComputedStyle()
	style.Position = PositionFixed
	rootPositioned.Style = &style
	if f, err := NewStructuralFitter(rootPositioned, fitTestParagraphs(nil)); f != nil || !errors.Is(err, ErrUnsupportedFit) {
		t.Fatalf("profile %v %v", f, err)
	}
	root, _, material := fitTestTree(t)
	f, err := NewStructuralFitter(root, fitTestParagraphs(material))
	if err != nil {
		t.Fatal(err)
	}
	// Snapshot structure is independent even of later caller mutation. Normal
	// callers keep normalized trees immutable, but results never borrow them.
	root.Children = nil
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var all []FitAlternative
			status, err := f.Enumerate(context.Background(), FitRequest{Width: 2, Height: 10}, func(a FitAlternative) bool { all = append(all, a); return true })
			if err != nil || status != FitExhausted || len(all) != 6 || fitText(all[0]) != "abcdefgh" {
				t.Errorf("concurrent output: %v, %v", status, err)
			}
		}()
	}
	wg.Wait()
}
