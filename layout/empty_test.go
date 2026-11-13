package layout

import "testing"

func TestBuildLayoutTree_EmptyContainers(t *testing.T) {
	for _, display := range []string{"block", "inline-block"} {
		for _, contents := range []string{"no_children", "empty_text", "hidden_children"} {
			t.Run(display+"/"+contents, func(t *testing.T) {
				var children []*RenderNode
				switch contents {
				case "empty_text":
					children = []*RenderNode{newRenderText(3, "")}
				case "hidden_children":
					children = []*RenderNode{newRenderElement(3, "none", newRenderText(4, "hidden"))}
				}
				input := newRenderElement(1, "block", newRenderElement(2, display, children...))
				build := func() *LayoutNode {
					t.Helper()
					root, err := BuildLayoutTree(input, BuildOptions{})
					if err != nil {
						t.Fatal(err)
					}
					if len(root.Children) != 1 {
						t.Fatalf("root has %d children, want 1", len(root.Children))
					}
					container := root.Children[0]
					if display == "inline-block" {
						if container.Box != BoxAnonymousInline || len(container.Children) != 1 {
							t.Fatal("inline-block must participate in an outer inline context")
						}
						container = container.Children[0]
						if container.Box != BoxInlineBlock {
							t.Fatal("expected atomic inline-block")
						}
					}
					if len(container.Children) != 1 {
						t.Fatalf("empty container has %d children, want one anonymous inline root", len(container.Children))
					}
					inline := container.Children[0]
					if inline.Box != BoxAnonymousInline || inline.FC != FCInline || len(inline.Children) != 0 {
						t.Fatal("expected an empty anonymous inline formatting context")
					}
					if inline.BoxID == container.BoxID || inline.NodeID != 0 {
						t.Fatal("anonymous inline needs its own box identity and no source node")
					}
					return inline
				}
				if first, second := build(), build(); first.BoxID != second.BoxID {
					t.Fatal("empty wrapper identity is not deterministic")
				}
			})
		}
	}
}

type emptyInlineRecorder struct {
	calls int
	root  *LayoutNode
	width float32
}

func (r *emptyInlineRecorder) LayoutInline(req InlineRequest) (InlineResult, error) {
	r.calls++
	r.root, r.width = req.Group, req.ContentWidth
	return InlineResult{}, nil
}

func TestFlowLayout_BuiltEmptyRoot(t *testing.T) {
	root, err := BuildLayoutTree(newRenderElement(1, "block"), BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 200}})
	if err != nil {
		t.Fatal(err)
	}
	inline := &emptyInlineRecorder{}
	result, err := FlowLayout(root, used, inline, nil, LayoutContext{}, LayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if inline.calls != 1 || inline.root == nil || inline.root.Box != BoxAnonymousInline || len(inline.root.Children) != 0 || inline.width != 200 {
		t.Fatalf("empty root was not delegated at its content width: %+v", inline)
	}
	if _, ok := used[inline.root.BoxID]; !ok {
		t.Fatal("empty inline root is missing used values")
	}
	geometry := result.Geometry[root.BoxID]
	if geometry.Content.H != 0 || geometry.Frame.H != 0 || geometry.Content.W != 200 {
		t.Fatalf("unexpected empty root geometry: %+v", geometry)
	}
}
