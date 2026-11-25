package layout

import (
	"errors"
	"github.com/npillmayer/css-box-layout/text"
	"reflect"
	"sync"
	"testing"
)

func provenanceSource(t *testing.T, p *NormalizationProvenance, id NodeID) SourceBoundaries {
	t.Helper()
	for _, s := range p.Sources() {
		if s.Source == id {
			return s
		}
	}
	t.Fatalf("missing source %d", id)
	return SourceBoundaries{}
}

func TestNormalizationProvenanceSplitsAndHoists(t *testing.T) {
	input := newRenderElement(1, blockDisplay,
		newRenderElement(2, inlineDisplay, newRenderText(3, "before"), newRenderElement(4, blockDisplay, newRenderText(5, "block")), newRenderText(6, "after")),
		newRenderElement(7, inlineDisplay, newRenderElement(8, blockDisplay)))
	before := copyRenderProjection(input)
	plain, err := BuildLayoutTrees(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	trees, p, err := BuildLayoutTreesWithProvenance(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plain, trees) || !reflect.DeepEqual(before, input) {
		t.Fatal("provenance changed normalization/input")
	}
	split := provenanceSource(t, p, 2)
	if len(split.Occurrences) != 2 || split.Before.Disposition != BoundaryRetained || split.After.Disposition != BoundaryRetained || split.Before.Position.Box != split.Occurrences[0] || split.After.Position.Box != split.Occurrences[1] {
		t.Fatal(split)
	}
	block := provenanceSource(t, p, 4)
	if !(split.Before.Position.Order < block.Before.Position.Order && block.After.Position.Order < split.After.Position.Order) {
		t.Fatal("hoisted order")
	}
	if split.Before.Position.Group == 0 || split.After.Position.Group == 0 || split.Before.Position.Group == split.After.Position.Group {
		t.Fatal("anonymous groups")
	}
	hoistedOnly := provenanceSource(t, p, 7)
	child := provenanceSource(t, p, 8)
	if len(hoistedOnly.Occurrences) != 0 || hoistedOnly.Before != child.Before || hoistedOnly.After != child.After {
		t.Fatal("hoisted-only source boundaries", hoistedOnly)
	}
	for i, b := range p.Boundaries() {
		if b.Order != uint64(i+1) || b.Root != trees.Flow.BoxID {
			t.Fatal("structural ordinal", b)
		}
	}
	// Values and occurrence slices are independent of the index and other builds.
	copy := p.Sources()
	copy[1].Occurrences[0] = 0
	positions := p.Boundaries()
	positions[0].Box = 0
	_, again, err := BuildLayoutTreesWithProvenance(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Sources(), again.Sources()) || !reflect.DeepEqual(p.Boundaries(), again.Boundaries()) {
		t.Fatal("mapping aliased or nondeterministic")
	}
}

func TestNormalizationProvenanceEmptyOmittedAndSuppressed(t *testing.T) {
	hidden := newRenderElement(8, Display{Kind: DisplayNone}, newRenderElement(9, inlineDisplay))
	input := newRenderElement(0, blockDisplay,
		newRenderText(1, " "), newRenderElement(2, blockDisplay),
		newRenderElement(3, inlineDisplay), newRenderText(4, "\t"),
		newRenderElement(5, blockDisplay, newRenderElement(6, inlineDisplay), newRenderText(7, "")), hidden)
	trees, p, err := BuildLayoutTreesWithProvenance(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []NodeID{1, 3, 4} {
		s := provenanceSource(t, p, id)
		if s.Before.Disposition != BoundaryOmitted || s.After.Disposition != BoundaryOmitted || s.Before.Position != (StructuralBoundary{}) || len(s.Occurrences) != 0 {
			t.Fatal("omitted", s)
		}
	}
	for _, id := range []NodeID{8, 9} {
		s := provenanceSource(t, p, id)
		if s.Before.Disposition != BoundarySuppressed || s.After.Disposition != BoundarySuppressed {
			t.Fatal("suppressed", s)
		}
	}
	empty := provenanceSource(t, p, 7)
	if empty.Before.Disposition != BoundaryEmpty || empty.After.Disposition != BoundaryEmpty {
		t.Fatal(empty)
	}
	anchor := provenanceSource(t, p, 6)
	if anchor.Before.Disposition != BoundaryRetained || anchor.Before.Position.Group == 0 || len(anchor.Occurrences) != 1 {
		t.Fatal("retained empty inline", anchor)
	}
	root := provenanceSource(t, p, 0)
	if root.Before.Position.Box != trees.Flow.BoxID || len(root.Occurrences) != 1 {
		t.Fatal("zero source identity", root)
	}
	suppressed, sp, err := BuildLayoutTreesWithProvenance(hidden, BuildOptions{})
	if err != nil || suppressed.Flow != nil || len(sp.Sources()) != 2 || len(sp.Boundaries()) != 0 {
		t.Fatal("suppressed root", err)
	}
}

func TestNormalizationProvenancePartialOmissionAndAtomics(t *testing.T) {
	input := newRenderElement(1, blockDisplay, newRenderElement(2, inlineDisplay, newRenderText(3, " "), newRenderElement(4, blockDisplay), newRenderText(5, "tail")), newRenderElement(6, inlineBlockDisplay, newRenderText(7, "inside")))
	_, p, err := BuildLayoutTreesWithProvenance(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s := provenanceSource(t, p, 2)
	if s.Before.Disposition != BoundaryOmitted || s.After.Disposition != BoundaryRetained || len(s.Occurrences) != 1 {
		t.Fatal("discarded first fragment must not move boundary", s)
	}
	atomic, inside := provenanceSource(t, p, 6), provenanceSource(t, p, 7)
	if atomic.Before.Position.Group == 0 || inside.Before.Position.Group == 0 || atomic.Before.Position.Group == inside.Before.Position.Group {
		t.Fatal("atomic interior group")
	}
}

func TestNormalizationProvenanceFixedAndFailures(t *testing.T) {
	fixed := fixedElement(2, blockDisplay, newRenderText(3, "fixed"), fixedElement(4, blockDisplay))
	input := newRenderElement(1, blockDisplay, fixed, newRenderText(5, "flow"))
	trees, p, err := BuildLayoutTreesWithProvenance(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []NodeID{2, 4} {
		s := provenanceSource(t, p, id)
		if s.Before.Position.Root != trees.Fixed[i].BoxID || s.Before.Position.Order != 1 || len(s.Occurrences) != 1 {
			t.Fatal("fixed root", s)
		}
	}
	cause := errors.New("read failure")
	trees, p, err = BuildLayoutTreesWithProvenance(input, BuildOptions{ReadText: func(_ text.TextRef) (string, error) { return "", cause }})
	var located *BuildError
	if trees != nil || p != nil || !errors.Is(err, cause) || !errors.As(err, &located) || located.RootID != 1 || located.NodeID != 3 {
		t.Fatal("partial publication/location", err)
	}
	duplicate := newRenderElement(1, blockDisplay, newRenderElement(1, inlineDisplay))
	trees, p, err = BuildLayoutTreesWithProvenance(duplicate, BuildOptions{})
	if trees != nil || p != nil || !errors.As(err, &located) || located.NodeID != 1 {
		t.Fatal("ambiguous source IDs", err)
	}
	trees, p, err = BuildLayoutTreesWithProvenance(nil, BuildOptions{})
	if err != nil || trees.Flow != nil || len(p.Sources()) != 0 || len(p.Boundaries()) != 0 {
		t.Fatal("nil build", err)
	}
	var zero *NormalizationProvenance
	if zero.Sources() != nil || zero.Boundaries() != nil {
		t.Fatal("nil provenance")
	}

}

func TestNormalizationProvenanceConcurrentCopies(t *testing.T) {
	input := newRenderElement(1, blockDisplay, newRenderElement(2, inlineDisplay, newRenderText(3, "text")))
	_, p, err := BuildLayoutTreesWithProvenance(input, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := p.Sources()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				_, q, err := BuildLayoutTreesWithProvenance(input, BuildOptions{})
				if err != nil || !reflect.DeepEqual(want, q.Sources()) {
					t.Error("independent build")
				}
				s := p.Sources()
				s[0].Occurrences[0] = 0
				b := p.Boundaries()
				b[0].Order = 0
			}
		}()
	}
	wg.Wait()
	if !reflect.DeepEqual(want, p.Sources()) {
		t.Fatal("concurrent mutation")
	}
}

func TestNormalizationProvenanceKeepsTextCoordinates(t *testing.T) {
	leaf := newRenderText(2, "café")
	leaf.Text = &text.TextRef{Source: 91, Range: text.TextRange{Start: 40, End: 45}}
	root := newRenderElement(1, blockDisplay, leaf)
	calls := 0
	trees, p, err := BuildLayoutTreesWithProvenance(root, BuildOptions{ReadText: func(ref text.TextRef) (string, error) {
		calls++
		if ref != *leaf.Text {
			t.Fatal("changed source text range")
		}
		return "café", nil
	}})
	if err != nil || calls != 1 {
		t.Fatal("extra source read", calls, err)
	}
	s := provenanceSource(t, p, 2)
	node := trees.Flow.Children[0].Children[0]
	if node.Text != *leaf.Text || s.Before.Position.Box != node.BoxID || s.After.Position.Box != node.BoxID || s.Before.Position.Group != trees.Flow.Children[0].BoxID {
		t.Fatal("text/group correspondence")
	}
}
