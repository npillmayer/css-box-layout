package layout

import (
	"reflect"
	"testing"
)

// Compare trees and geometry independently of allocation IDs: removing a source
// subtree may renumber boxes, but must not change their normal-flow behavior.
func canonicalFlow(n *LayoutNode) (*LayoutNode, map[BoxID]BoxID) {
	ids := map[BoxID]BoxID{}
	walkPositionTree(n, func(box *LayoutNode) { ids[box.BoxID] = BoxID(len(ids) + 1) })
	copy := copyTestTree(n)
	walkPositionTree(copy, func(box *LayoutNode) { box.BoxID = ids[box.BoxID] })
	return copy, ids
}

func TestFixedDiversionPreservesNormalFlow(t *testing.T) {
	for _, kind := range []string{"inline run", "mixed margins", "auto atomic", "only fixed", "collapsible whitespace"} {
		for _, display := range []Display{blockDisplay, flowRootDisplay, inlineDisplay, inlineBlockDisplay} {
			t.Run(kind+"/"+display.String(), func(t *testing.T) {
				fixture := func(includeFixed bool) *RenderNode {
					// This subtree would fail normal atomic-width validation if leaked,
					// and its size/margins would visibly change its parent's layout.
					inside := newRenderElement(92, inlineBlockDisplay, newRenderText(93, "fixed text"))
					inside.Style = &ComputedStyle{Width: lenPct(.5)}
					fixed := fixedElement(90, display, inside, fixedElement(94, blockDisplay))
					fixed.Style.Width = lenPx(1000)
					fixed.Style.Margin = edges(lenPx(1000))
					children := func(a, b *RenderNode) []*RenderNode {
						var nodes []*RenderNode
						if a != nil {
							nodes = append(nodes, a)
						}
						if includeFixed {
							nodes = append(nodes, fixed)
						}
						if b != nil {
							nodes = append(nodes, b)
						}
						return nodes
					}
					var nodes []*RenderNode
					switch kind {
					case "inline run":
						nodes = []*RenderNode{newRenderElement(2, inlineDisplay, children(newRenderText(3, "before"), newRenderText(4, "after"))...)}
					case "mixed margins":
						a := marginTestBlock(2, 13, 8, true)
						b := marginTestBlock(3, 20, 7, true)
						nodes = []*RenderNode{marginTestBlock(4, 5, 9, false, children(a, b)...)}
					case "auto atomic":
						atomic := newRenderElement(2, inlineBlockDisplay, children(newRenderText(3, "inside"), newRenderText(4, "tail"))...)
						atomic.Style = &ComputedStyle{Width: lenAuto(), Padding: edges(lenPx(2))}
						nodes = []*RenderNode{newRenderText(5, "before"), atomic, newRenderText(6, "after")}
					case "only fixed":
						nodes = children(nil, nil)
					case "collapsible whitespace":
						nodes = []*RenderNode{marginTestBlock(2, 13, 8, true)}
						nodes = append(nodes, children(newRenderText(3, " "), newRenderText(4, "\t"))...)
						nodes = append(nodes, marginTestBlock(5, 20, 7, true))
					}
					root := newRenderElement(1, blockDisplay, nodes...)
					s := DefaultComputedStyle()
					s.Width = lenPx(120)
					root.Style = &s
					return root
				}
				base, err := BuildLayoutTree(fixture(false), BuildOptions{})
				if err != nil {
					t.Fatal(err)
				}
				trees, err := BuildLayoutTrees(fixture(true), BuildOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if len(trees.Fixed) != 2 {
					t.Fatalf("fixed roots = %d", len(trees.Fixed))
				}
				wantTree, _ := canonicalFlow(base)
				gotTree, _ := canonicalFlow(trees.Flow)
				if !reflect.DeepEqual(gotTree, wantTree) {
					t.Fatal("fixed diversion changed normal-flow tree")
				}

				type observation struct {
					geometry        LayoutGeometryTable
					calls, measured []BoxID
					texts           map[BoxID][]BoxID
				}
				run := func(root *LayoutNode) observation {
					_, ids := canonicalFlow(root)
					used, err := ResolveUsedValues(root, ResolveContext{ContainingBlock: Rect{W: 120}, FontSizePx: 16})
					if err != nil {
						t.Fatal(err)
					}
					obs := observation{geometry: LayoutGeometryTable{}, texts: map[BoxID][]BoxID{}}
					measure := measureFunc(func(req InlineMeasureRequest) (IntrinsicWidths, error) {
						obs.measured = append(obs.measured, ids[req.OwnerID])
						var width float32
						walkPositionTree(req.Group, func(n *LayoutNode) {
							if n.Box == BoxText {
								width += float32(n.Text.Range.End-n.Text.Range.Start) * 5
							}
						})
						return IntrinsicWidths{MinContent: width, MaxContent: width}, nil
					})
					adapter := newAtomicFake(t)
					result, err := FlowLayout(root, used, adapter, measure, LayoutContext{}, LayoutOptions{})
					if err != nil {
						t.Fatal(err)
					}
					for id, g := range result.Geometry {
						g.RelativeTo = ids[g.RelativeTo]
						obs.geometry[ids[id]] = g
					}
					for _, id := range adapter.calls {
						obs.calls = append(obs.calls, ids[id])
					}
					for group, texts := range adapter.texts {
						for _, id := range texts {
							obs.texts[ids[group]] = append(obs.texts[ids[group]], ids[id])
						}
					}
					return obs
				}
				want, got := run(base), run(trees.Flow)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("normal flow changed:\ngot %+v\nwant %+v", got, want)
				}
				if kind == "auto atomic" && len(got.measured) == 0 {
					t.Fatal("test did not exercise intrinsic sizing")
				}
				if kind == "mixed margins" && got.geometry[1].Content.H != 62 {
					// Parent top collapses to max(5,13), sibling gap is 20,
					// parent bottom collapses to max(9,7), and both lines are 10px.
					t.Fatalf("margin fixture height = %g, want 62", got.geometry[1].Content.H)
				}
			})
		}
	}
}
