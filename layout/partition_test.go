package layout

import (
	"errors"
	"maps"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/npillmayer/css-box-layout/text"
)

func copyRenderProjection(n *RenderNode) *RenderNode {
	if n == nil {
		return nil
	}
	c := *n
	c.Styles = maps.Clone(n.Styles)
	if n.HTML != nil {
		h := *n.HTML
		h.Attr = slices.Clone(h.Attr)
		c.HTML = &h
	}
	if n.Style != nil {
		s := *n.Style
		if s.Fixed != nil {
			f := *s.Fixed
			s.Fixed = &f
		}
		c.Style = &s
	}
	if n.Text != nil {
		r := *n.Text
		c.Text = &r
	}
	if n.Replaced != nil {
		r := *n.Replaced
		c.Replaced = &r
	}
	if n.ChildrenNodes != nil {
		c.ChildrenNodes = make([]*RenderNode, len(n.ChildrenNodes))
		for i, child := range n.ChildrenNodes {
			c.ChildrenNodes[i] = copyRenderProjection(child)
		}
	}
	return &c
}

func partitionFixture() (original, residual, branch *RenderNode) {
	before, after := newRenderText(2, "Before "), newRenderText(3, " after.")
	content := newRenderText(5, "café\u00a0soft\u00adhyphen")
	content.Text = &text.TextRef{Source: 42, Range: text.TextRange{Start: 100, End: 100 + uint64(len(content.HTML.Data))}}
	content.Style = &ComputedStyle{FontSizePx: 36, WhiteSpace: WhiteSpacePre}
	atomic := newRenderElement(6, inlineBlockDisplay, newRenderText(7, "\n"))
	branch = newRenderElement(4, flowRootDisplay, content, atomic, newRenderElement(8, blockDisplay))
	style := DefaultComputedStyle()
	style.FontSizePx = 24
	style.WhiteSpace = WhiteSpacePre
	style.Width = lenPct(.75)
	style.Margin.Left = lenPx(10)
	style.Padding.Left = lenPx(8)
	style.Border.Left = lenPx(2)
	branch.Style = &style
	emptied := newRenderElement(9, blockDisplay, branch)
	emptied.Style = &ComputedStyle{Width: lenAuto(), FontSizePx: 80, Padding: edges(lenPx(16)), Border: edges(lenPx(1))}
	original = newRenderElement(1, blockDisplay, before, emptied, after)
	original.Style = &ComputedStyle{Width: lenAuto(), FontSizePx: 16, WhiteSpace: WhiteSpaceNormal}
	// Adapter filtering happens before normalization. The engine neither routes
	// names nor clones source ancestors into a detached branch.
	residual = copyRenderProjection(original)
	residual.ChildrenNodes[1].ChildrenNodes = nil
	return
}

func TestBuildPartitionedProjectionsPreserveSourcesAndContext(t *testing.T) {
	original, residual, branch := partitionFixture()
	beforeOriginal, beforeResidual, beforeBranch := copyRenderProjection(original), copyRenderProjection(residual), copyRenderProjection(branch)
	whole, err := BuildLayoutTree(original, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rest, err := BuildLayoutTree(residual, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	detached, err := BuildLayoutTree(branch, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, beforeOriginal) || !reflect.DeepEqual(residual, beforeResidual) || !reflect.DeepEqual(branch, beforeBranch) {
		t.Fatal("construction mutated a render projection")
	}
	// Source correspondence survives normalization; anonymous nodes have no
	// invented source ID. TextRef coordinates need not begin at zero.
	refs := map[NodeID]text.TextRef{}
	var walk func(*LayoutNode, func(*LayoutNode))
	walk = func(n *LayoutNode, f func(*LayoutNode)) {
		f(n)
		for _, child := range n.Children {
			walk(child, f)
		}
	}
	walk(detached, func(n *LayoutNode) {
		if n.Box == BoxAnonymousBlock || n.Box == BoxAnonymousInline {
			if n.NodeID != 0 {
				t.Fatal("invented anonymous source")
			}
			if n.Style.FontSizePx != 24 || n.Style.WhiteSpace != WhiteSpacePre {
				t.Fatal("anonymous owner lost source context")
			}
		}
		if n.Box == BoxText {
			refs[n.NodeID] = n.Text
		}
	})
	if !reflect.DeepEqual(refs, map[NodeID]text.TextRef{5: *branch.ChildrenNodes[0].Text, 7: *branch.ChildrenNodes[1].ChildrenNodes[0].Text}) {
		t.Fatalf("changed decoded ranges: %+v", refs)
	}
	if !detached.EstablishesBFC || *detached.Style != *branch.Style {
		t.Fatal("branch root edges/font/whitespace changed")
	}
	var sourceBranch *LayoutNode
	walk(whole, func(n *LayoutNode) {
		if n.NodeID == 4 {
			sourceBranch = n
		}
	})
	// Ignore allocation IDs: source styles remain identical when the same branch
	// is detached from the differently styled source parent.
	walk(sourceBranch, func(n *LayoutNode) {
		if n.NodeID == 4 && *n.Style != *detached.Style {
			t.Fatal("detachment changed style")
		}
	})
	walk(rest, func(n *LayoutNode) {
		if n.NodeID >= 4 && n.NodeID <= 8 {
			t.Fatal("filtered material survived in residual")
		}
	})
	empty := rest.Children[1]
	if empty.NodeID != 9 || empty.Style.Padding.Left.Value != 16 || empty.Style.Border.Left.Value != 1 || len(empty.Children) != 1 || empty.Children[0].Box != BoxAnonymousInline || len(empty.Children[0].Children) != 0 || empty.Children[0].Style.FontSizePx != 80 {
		t.Fatal("emptied decorated ancestor lost")
	}
	// Rebuilding the same source has a separate BoxID domain and separate values.
	again, err := BuildLayoutTree(branch, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if again == detached || again.Style == detached.Style || again.BoxID != detached.BoxID || !reflect.DeepEqual(again, detached) {
		t.Fatal("independent builds lost deterministic local identity or isolation")
	}
	detachedBefore := copyTestTree(detached)
	branch.Style.FontSizePx = 99
	branch.ChildrenNodes[0].Text.Range.End++
	if !reflect.DeepEqual(detached, detachedBefore) {
		t.Fatal("output borrows mutable adapter values")
	}
	// A later failed build cannot alter any earlier output.
	failure := errors.New("late read failure")
	failed, err := BuildLayoutTree(branch, BuildOptions{ReadText: func(text.TextRef) (string, error) { return "", failure }})
	var buildErr *BuildError
	if failed != nil || !errors.Is(err, failure) || !errors.As(err, &buildErr) || buildErr.RootID != 4 || buildErr.NodeID != 5 {
		t.Fatal("failed independent build lost identity", err)
	}
	if !reflect.DeepEqual(detached, detachedBefore) || !reflect.DeepEqual(again, detachedBefore) {
		t.Fatal("failed build changed prior outputs")
	}
}

func TestBuildFilteredInlineAdjacency(t *testing.T) {
	first, last := newRenderText(2, "Before "), newRenderText(5, " after.")
	taken := newRenderElement(3, blockDisplay, newRenderText(4, "routed"))
	original := newRenderElement(1, blockDisplay, first, taken, last)
	original.Style = &ComputedStyle{Width: lenAuto(), FontSizePx: 22}
	filtered := *original
	filtered.ChildrenNodes = []*RenderNode{first, last}
	whole, err := BuildLayoutTree(original, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	root, err := BuildLayoutTree(&filtered, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(whole.Children) != 3 || len(root.Children) != 1 || root.Children[0].Box != BoxAnonymousInline {
		t.Fatal("filtering must precede normalization")
	}
	group := root.Children[0]
	if group.Style.FontSizePx != 22 || len(group.Children) != 2 || group.Children[0].NodeID != 2 || group.Children[1].NodeID != 5 || group.Children[0].Text != *first.Text || group.Children[1].Text != *last.Text {
		t.Fatal("adjacency lost owner/text context")
	}
	if len(original.ChildrenNodes) != 3 || original.ChildrenNodes[1] != taken {
		t.Fatal("filter/build mutated original ancestry")
	}
}

func TestBuildPartitionsConcurrently(t *testing.T) {
	original, residual, branch := partitionFixture()
	before := copyRenderProjection(original)
	var wg sync.WaitGroup
	for range 6 {
		for _, input := range []*RenderNode{residual, branch} {
			wg.Go(func() {
				root, err := BuildLayoutTree(input, BuildOptions{})
				if err != nil {
					t.Error(err)
					return
				}
				if root.NodeID != input.ID || *root.Style != *input.Style {
					t.Error("independent construction lost root snapshot")
				}
			})
		}
	}
	wg.Wait()
	if !reflect.DeepEqual(original, before) {
		t.Fatal("concurrent read-only builds changed source")
	}
}
