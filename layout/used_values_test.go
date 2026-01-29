package layout

import "testing"

func lenPx(v float32) Length     { return Length{Kind: LenPx, Value: v} }
func lenPct(v float32) Length    { return Length{Kind: LenPercent, Value: v} }
func lenEm(v float32) Length     { return Length{Kind: LenEm, Value: v} }
func lenAuto() Length            { return Length{Kind: LenAuto} }
func edges(v Length) EdgeLengths { return EdgeLengths{Top: v, Right: v, Bottom: v, Left: v} }

func TestResolveUsedValues_BasicEdgesAndWidth(t *testing.T) {
	tests := []struct {
		name     string
		node     *LayoutNode
		ctx      ResolveContext
		expected UsedValues
	}{
		{
			name: "padding_px_auto_width",
			node: &LayoutNode{
				BoxID: 1,
				Box:   BoxBlock,
				Style: &ComputedStyle{
					Width:   lenAuto(),
					Padding: edges(lenPx(10)),
				},
			},
			ctx: ResolveContext{ContainingBlock: Rect{W: 200}, FontSizePx: 16},
			expected: UsedValues{
				Padding:      Edges{Top: 10, Right: 10, Bottom: 10, Left: 10},
				ContentWidth: 200 - 20,
			},
		},
		{
			name: "padding_percent",
			node: &LayoutNode{
				BoxID: 1,
				Box:   BoxBlock,
				Style: &ComputedStyle{
					Width:   lenAuto(),
					Padding: edges(lenPct(0.10)),
				},
			},
			ctx: ResolveContext{ContainingBlock: Rect{W: 200}, FontSizePx: 16},
			expected: UsedValues{
				Padding:      Edges{Top: 20, Right: 20, Bottom: 20, Left: 20},
				ContentWidth: 200 - 40,
			},
		},
		{
			name: "border_px",
			node: &LayoutNode{
				BoxID: 1,
				Box:   BoxBlock,
				Style: &ComputedStyle{
					Width:  lenAuto(),
					Border: edges(lenPx(1)),
				},
			},
			ctx: ResolveContext{ContainingBlock: Rect{W: 200}, FontSizePx: 16},
			expected: UsedValues{
				Border:       Edges{Top: 1, Right: 1, Bottom: 1, Left: 1},
				ContentWidth: 200 - 2,
			},
		},
		{
			name: "width_auto_fill",
			node: &LayoutNode{
				BoxID: 1,
				Box:   BoxBlock,
				Style: &ComputedStyle{
					Width:   lenAuto(),
					Margin:  EdgeLengths{Left: lenPx(5), Right: lenPx(5)},
					Padding: EdgeLengths{Left: lenPx(10), Right: lenPx(10)},
					Border:  EdgeLengths{Left: lenPx(1), Right: lenPx(1)},
				},
			},
			ctx: ResolveContext{ContainingBlock: Rect{W: 200}, FontSizePx: 16},
			expected: UsedValues{
				Margin:       Edges{Left: 5, Right: 5},
				Padding:      Edges{Left: 10, Right: 10},
				Border:       Edges{Left: 1, Right: 1},
				ContentWidth: 168,
			},
		},
		{
			name: "width_fixed_px",
			node: &LayoutNode{
				BoxID: 1,
				Box:   BoxBlock,
				Style: &ComputedStyle{
					Width:  lenPx(120),
					Margin: EdgeLengths{Left: lenAuto(), Right: lenAuto()},
				},
			},
			ctx: ResolveContext{ContainingBlock: Rect{W: 200}, FontSizePx: 16},
			expected: UsedValues{
				Margin:       Edges{Left: 0, Right: 0},
				ContentWidth: 120,
			},
		},
		{
			name: "width_percent",
			node: &LayoutNode{
				BoxID: 1,
				Box:   BoxBlock,
				Style: &ComputedStyle{
					Width: lenPct(0.50),
				},
			},
			ctx: ResolveContext{ContainingBlock: Rect{W: 300}, FontSizePx: 16},
			expected: UsedValues{
				ContentWidth: 150,
			},
		},
		{
			name: "width_em",
			node: &LayoutNode{
				BoxID: 1,
				Box:   BoxBlock,
				Style: &ComputedStyle{
					Width: lenEm(2),
				},
			},
			ctx: ResolveContext{ContainingBlock: Rect{W: 300}, FontSizePx: 16},
			expected: UsedValues{
				ContentWidth: 32,
			},
		},
		{
			name: "width_auto_clamp",
			node: &LayoutNode{
				BoxID: 1,
				Box:   BoxBlock,
				Style: &ComputedStyle{
					Width:   lenAuto(),
					Padding: EdgeLengths{Left: lenPx(200), Right: lenPx(200)},
				},
			},
			ctx: ResolveContext{ContainingBlock: Rect{W: 300}, FontSizePx: 16},
			expected: UsedValues{
				Padding:      Edges{Left: 200, Right: 200},
				ContentWidth: 0,
			},
		},
		{
			name: "inline_level_width_zero",
			node: &LayoutNode{
				BoxID: 1,
				Box:   BoxInline,
				Style: &ComputedStyle{
					Width:   lenPx(200),
					Padding: EdgeLengths{Left: lenPx(10)},
				},
			},
			ctx: ResolveContext{ContainingBlock: Rect{W: 300}, FontSizePx: 16},
			expected: UsedValues{
				Padding:      Edges{Left: 10},
				ContentWidth: 0,
			},
		},
		{
			name: "text_node_width_zero",
			node: &LayoutNode{
				BoxID: 1,
				Box:   BoxText,
				Style: &ComputedStyle{
					Margin: EdgeLengths{Left: lenPx(5)},
				},
			},
			ctx: ResolveContext{ContainingBlock: Rect{W: 300}, FontSizePx: 16},
			expected: UsedValues{
				Margin:       Edges{Left: 5},
				ContentWidth: 0,
			},
		},
		{
			name: "inline_block_width_fixed",
			node: &LayoutNode{
				BoxID: 1,
				Box:   BoxInlineBlock,
				Style: &ComputedStyle{
					Width: lenPx(200),
				},
			},
			ctx: ResolveContext{ContainingBlock: Rect{W: 300}, FontSizePx: 16},
			expected: UsedValues{
				ContentWidth: 200,
			},
		},
		{
			name: "inline_block_width_auto",
			node: &LayoutNode{
				BoxID: 1,
				Box:   BoxInlineBlock,
				Style: &ComputedStyle{
					Width: lenAuto(),
				},
			},
			ctx: ResolveContext{ContainingBlock: Rect{W: 300}, FontSizePx: 16},
			expected: UsedValues{
				ContentWidth: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveUsedValues(tt.node, tt.ctx)
			if err != nil {
				t.Fatalf("ResolveUsedValues error: %v", err)
			}
			uv, ok := got[tt.node.BoxID]
			if !ok {
				t.Fatalf("missing used values for BoxID %d", tt.node.BoxID)
			}
			if uv.ContentWidth != tt.expected.ContentWidth {
				t.Fatalf("ContentWidth = %v, want %v", uv.ContentWidth, tt.expected.ContentWidth)
			}
			if uv.Margin != tt.expected.Margin {
				t.Fatalf("Margin = %+v, want %+v", uv.Margin, tt.expected.Margin)
			}
			if uv.Padding != tt.expected.Padding {
				t.Fatalf("Padding = %+v, want %+v", uv.Padding, tt.expected.Padding)
			}
			if uv.Border != tt.expected.Border {
				t.Fatalf("Border = %+v, want %+v", uv.Border, tt.expected.Border)
			}
		})
	}
}

func TestResolveUsedValues_ContextPropagation(t *testing.T) {
	parent := &LayoutNode{
		BoxID: 1,
		Box:   BoxBlock,
		Style: &ComputedStyle{
			Width: lenPx(200),
		},
	}
	child := &LayoutNode{
		BoxID: 2,
		Box:   BoxBlock,
		Style: &ComputedStyle{
			Padding: EdgeLengths{Left: lenPct(0.10)},
		},
	}
	parent.Children = []*LayoutNode{child}

	used, err := ResolveUsedValues(parent, ResolveContext{ContainingBlock: Rect{W: 200}, FontSizePx: 16})
	if err != nil {
		t.Fatalf("ResolveUsedValues error: %v", err)
	}
	if used[child.BoxID].Padding.Left != 20 {
		t.Fatalf("child padding-left = %v, want 20", used[child.BoxID].Padding.Left)
	}
}

func TestResolveUsedValues_InlineInheritsWidth(t *testing.T) {
	parent := &LayoutNode{
		BoxID: 1,
		Box:   BoxBlock,
		Style: &ComputedStyle{
			Width: lenPx(300),
		},
	}
	inlineChild := &LayoutNode{
		BoxID: 2,
		Box:   BoxInline,
		Style: &ComputedStyle{
			Padding: EdgeLengths{Left: lenPct(0.10)},
		},
	}
	parent.Children = []*LayoutNode{inlineChild}

	used, err := ResolveUsedValues(parent, ResolveContext{ContainingBlock: Rect{W: 300}, FontSizePx: 16})
	if err != nil {
		t.Fatalf("ResolveUsedValues error: %v", err)
	}
	if used[inlineChild.BoxID].Padding.Left != 30 {
		t.Fatalf("inline padding-left = %v, want 30", used[inlineChild.BoxID].Padding.Left)
	}
}
