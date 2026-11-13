package layout

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func copyTestTree(n *LayoutNode) *LayoutNode {
	if n == nil {
		return nil
	}
	c := *n
	if n.Style != nil {
		s := *n.Style
		c.Style = &s
	}
	if n.Children != nil {
		c.Children = make([]*LayoutNode, len(n.Children))
		for i, child := range n.Children {
			c.Children[i] = copyTestTree(child)
		}
	}
	return &c
}

func copyTestGeometry(g LayoutGeometryTable) LayoutGeometryTable {
	c := make(LayoutGeometryTable, len(g))
	for id, v := range g {
		c[id] = v
	}
	return c
}

func snapshotFixture(t *testing.T) *LayoutNode {
	t.Helper()
	note := newRenderElement(3, "block", newRenderText(4, "inside"))
	note.Style = &ComputedStyle{Width: lenPct(.5)}
	root, err := BuildLayoutTree(newRenderElement(1, "block", newRenderText(2, "before"), note, newRenderText(5, "after")), BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestFlowLayout_ResizeAndFailedPublication(t *testing.T) {
	root := snapshotFixture(t)
	original := copyTestTree(root)
	failure := errors.New("paragraph failed after earlier group succeeded")
	type document struct {
		layout     *LayoutResult
		paragraphs map[BoxID]testRetainedParagraph
		paint      map[BoxID]string
	}
	var published *document
	var requested []float32
	var failedRetained int
	run := func(width float32, fail bool) error {
		used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: width}})
		if err != nil {
			return err
		}
		usedBefore := make(UsedValuesTable, len(used))
		for id, u := range used {
			usedBefore[id] = u
		}
		pending := &document{paragraphs: make(map[BoxID]testRetainedParagraph), paint: make(map[BoxID]string)}
		calls := 0
		result, err := FlowLayout(root, used, inlineFunc(func(req InlineRequest) (InlineExtent, error) {
			i := calls
			calls++
			requested = append(requested, req.ContentWidth)
			if fail && i == 1 {
				return InlineExtent{}, failure
			}
			height := []float32{24, 36, 12}[i]
			if i == 1 && req.ContentWidth == 90 {
				height = 48
			}
			if fail {
				// Expose writes made before the later callback fails.
				height = 99
			}
			pending.paragraphs[req.Group.BoxID] = testRetainedParagraph{req.OwnerID, testParagraph{body: height}}
			pending.paint[req.Group.BoxID] = fmt.Sprintf("width=%g", width)
			return InlineExtent{W: req.ContentWidth, H: height}, nil
		}), nil, LayoutContext{}, LayoutOptions{})
		if !reflect.DeepEqual(used, usedBefore) {
			t.Error("flow mutated used values")
		}
		if !reflect.DeepEqual(root, original) {
			t.Error("layout mutated the structural input")
		}
		if err != nil {
			failedRetained = len(pending.paragraphs)
			if result != nil {
				t.Error("failed run returned partial result")
			}
			return err // Discard pending; never replace the published snapshot on failure.
		}
		pending.layout = result
		published = pending
		return nil
	}
	if err := run(240, false); err != nil {
		t.Fatal(err)
	}
	first := published
	geometryBefore := copyTestGeometry(first.layout.Geometry)
	if first.layout.Root != root {
		t.Fatal("immutable input tree should be shared")
	}
	if first.layout.Geometry[root.BoxID].Content.H != 72 || first.layout.Geometry[root.Children[2].BoxID].Frame.Y != 60 {
		t.Fatal("incorrect initial layout")
	}
	if err := run(180, false); err != nil {
		t.Fatal(err)
	}
	second := published
	if second.layout.Geometry[root.BoxID].Content.H != 84 || second.layout.Geometry[root.Children[2].BoxID].Frame.Y != 72 {
		t.Fatal("resize did not propagate height")
	}
	if !reflect.DeepEqual(requested, []float32{240, 120, 240, 180, 90, 180}) {
		t.Fatalf("resize requests=%v", requested)
	}
	if !reflect.DeepEqual(first.layout.Geometry, geometryBefore) || !reflect.DeepEqual(first.layout.Root, original) {
		t.Error("resize altered first snapshot")
	}
	for group, p := range first.paragraphs {
		if first.paint[group] != "width=240" || second.paint[group] != "width=180" {
			t.Error("paint snapshots alias")
		}
		if p.owner == root.Children[1].BoxID && (p.result.body != 36 || second.paragraphs[group].result.body != 48) {
			t.Error("typed paragraph snapshots changed or were not resized")
		}
	}
	secondBefore := copyTestGeometry(second.layout.Geometry)
	if err := run(160, true); !errors.Is(err, failure) {
		t.Fatalf("lost failure: %v", err)
	}
	if failedRetained != 1 || published != second {
		t.Fatal("partial result replaced successful publication")
	}
	if !reflect.DeepEqual(first.layout.Geometry, geometryBefore) || !reflect.DeepEqual(second.layout.Geometry, secondBefore) || !reflect.DeepEqual(root, original) {
		t.Error("failed run corrupted old snapshots")
	}
	// A failed adapter run must not poison later layouts of the same input.
	if err := run(240, false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(published.layout.Geometry, geometryBefore) {
		t.Fatal("rerun after failure changed deterministic result")
	}
}

func TestFlowLayout_AtomicCallbackDoesNotMutateInput(t *testing.T) {
	root := &LayoutNode{BoxID: 1, Box: BoxBlock, Children: []*LayoutNode{{BoxID: 2, Box: BoxAnonymousInline, Children: []*LayoutNode{{BoxID: 3, Box: BoxInlineBlock, Children: []*LayoutNode{{BoxID: 4, Box: BoxAnonymousInline}}}}}}}
	original := copyTestTree(root)
	used := UsedValuesTable{1: {ContentWidth: 200}, 2: {}, 3: {ContentWidth: 80}, 4: {}}
	failure := errors.New("failure after atomic sizing")
	for _, fail := range []bool{false, true} {
		result, err := FlowLayout(root, used, inlineFunc(func(req InlineRequest) (InlineExtent, error) {
			if req.OwnerID == 3 {
				return InlineExtent{W: 80, H: 10}, nil
			}
			w, h, err := req.Atomic.SizeInlineBlock(req.Group.Children[0], req.ContentWidth)
			if err != nil {
				return InlineExtent{}, err
			}
			if fail {
				return InlineExtent{}, failure
			}
			return InlineExtent{W: w, H: h}, nil
		}), nil, LayoutContext{}, LayoutOptions{})
		if fail {
			if !errors.Is(err, failure) || result != nil {
				t.Fatal("atomic failure not propagated")
			}
		} else if err != nil || result.Geometry[3].Frame.H != 10 {
			t.Fatalf("atomic layout failed: %v", err)
		}
		if !reflect.DeepEqual(root, original) {
			t.Error("atomic callback mutated input")
		}
	}
}

func TestFlowLayout_ConcurrentReadOnlyInputs(t *testing.T) {
	root := snapshotFixture(t)
	original := copyTestTree(root)
	used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 240}})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan *LayoutResult, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := FlowLayout(root, used, fakeInlineLayouter{extent: InlineExtent{W: 10, H: 10}}, nil, LayoutContext{}, LayoutOptions{})
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
		if r.Geometry[root.BoxID].Content.H != 30 {
			t.Error("concurrent layout changed result")
		}
	}
	if !reflect.DeepEqual(root, original) {
		t.Error("concurrent layouts mutated input")
	}
}
