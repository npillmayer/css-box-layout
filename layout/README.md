# layout

This package implements the core CSS box layout pipeline. It defines the layout tree (E), builds it from a styled render tree (D), and provides the entry points for resolving used values and running flow layout. The focus is correctness and clarity over optimization.

## Scope
- BuildLayoutTree: structural box tree construction (anonymous boxes, split+hoist).
- Used-values resolution (lengths, padding/border/margins).
- Flow layout entry points (block stacking + inline delegation).
- Shared types for layout nodes, geometry, and inline extent requests/results.

## Files (overview)
- `box.go`: core layout types (LayoutNode, BoxKind, geometry, edges).
- `boxid.go`: deterministic BoxID generation.
- `build_style.go`: typed style validation, snapshots, and anonymous inheritance.
- `flow.go`: Pass 1 helpers (flow items, normalizeBlockChildren, split+hoist).
- `layout.go`: public entry points for layout passes.
- `interfaces.go`: interfaces for inline layout and intrinsic measurement.
- `render.go`: minimal render-node stub for BuildLayoutTree (to be replaced by CSSDOM adapter).
- `stubs.go`: temporary types/placeholders used during early implementation.
- `*_test.go`: construction, identity, source/style, used-value, and flow tests.

## Notes
- CSSDOM creation, line breaking, and text shaping are external concerns and are integrated via interfaces.
- This package currently contains stubs while interfaces and adapters are finalized.

## Usage examples

Minimal build of the layout tree (structural pass only):

```go
root := &layout.RenderNode{
	ID: 1,
	HTML: &html.Node{Type: html.ElementNode, Data: "div"},
	Styles: map[string]string{"display": "block"},
}

tree, err := layout.BuildLayoutTree(root, layout.BuildOptions{})
if err != nil {
	// handle error
}
_ = tree
```

Flow layout entry point (requires inline/intrinsic adapters):

```go
used, err := layout.ResolveUsedValues(tree, layout.ResolveContext{
	ContainingBlock: layout.Rect{W: 800}, // fill in constraints
	FontSizePx:      16,
})
if err != nil {
	// handle error
}

res, err := layout.FlowLayout(
	tree,
	used,
	inlineLayouter,
	intrinsicMeasurer,
	layout.LayoutContext{ContainingBlock: layout.Rect{W: 800}},
	layout.LayoutOptions{},
)
if err != nil {
	// handle error
}
_ = res
```

## Supplying styles and text sources

The builder reads display from RenderNode.Styles and box values from the typed
RenderNode.Style field. It does not parse CSS lengths. For example:

```go
style := layout.DefaultComputedStyle()
style.Width = layout.Length{Kind: layout.LenPx, Value: 240}
style.FontSizePx = 16
root.Style = &style
```

A text node must carry its adapter-owned source reference:

```go
leaf := &layout.RenderNode{
    ID: 2,
    HTML: &html.Node{Type: html.TextNode, Data: "hello"},
    Text: &text.TextRef{
        Source: 7,
        Range: text.TextRange{Start: 100, End: 105},
    },
}
root.ChildrenNodes = []*layout.RenderNode{leaf}
```

Here `text` is `github.com/npillmayer/css-box-layout/text`. The adapter supplies
source 7 and validates the range against its bytes. Supplied ranges are copied
without rebasing; HTML.Data is not the storage contract. The builder drops empty
ranges and rejects reversed ranges or nonempty text without a reference.

Nil Style uses defaults; a non-nil zero style has explicit width 0px. FontSizePx
zero means inherit/fallback, not explicit CSS zero. Anonymous wrappers inherit
font size but not box edges. See [Tree-Type.md](../doc/Tree-Type.md) for the supported
subset and adapter responsibilities. Run `go test ./...` and `go vet ./...` for
validation.

## Inline adapter migration

Implement `LayoutInline(InlineRequest) (InlineResult, error)`. AtomicSizer is
removed. Core resolves and lays out atomic interiors before outer delegation;
`req.Atomics.Lookup(node.BoxID)` returns copied border-box dimensions for that
branch. Do not flatten its inner text or initiate sizing. Retain paragraphs by
Group.BoxID with OwnerID; local whatsit handles map to atomic BoxID.

Return full finite nonnegative Extent plus exactly one AtomicPlacement per
prepared item. X/Y denotes the border origin relative to OwnerID content, already
including margins/alignment. Negative positions are valid. Core rejects invalid
identity coverage or dimensions and applies validated placements without keeping
slice aliases. Extent-only adapters return `InlineResult{Extent: extent}`.

LayoutGeometry.RelativeTo explicitly identifies the content-coordinate parent
(root=0). Compose that chain once; do not reapply retained whatsit positions.
Fixed pixel atomic widths (including zero) and auto widths are supported. Percent
and em atomic widths still fail explicitly. For auto sizing, implement
`IntrinsicMeasurer.MeasureInline(InlineMeasureRequest) (IntrinsicWidths, error)`;
MaxContentWidth and InlineIntrinsic are removed. Core supplies nested intrinsic
border bounds by BoxID and a resolved edges-only view. The service returns ordered,
finite, nonnegative content bounds without publishing retained layout results.

Core selects bounded shrink-to-fit width, re-resolves descendants in a private
used-values table, then performs final layout once. UsedValues.WidthPending and
EdgesPending mark provisional values from ResolveUsedValues; they are never exposed
as final constraints. Percentage dependencies on unknown auto widths fail explicitly;
percentages with definite bases remain supported. Fixed-width flow needs no
intrinsic service. No speculative layout or cross-invocation cache is introduced. Real P wiring remains
external. Full tests, vet and race checks cover this bounded implementation.

## Snapshot ownership and geometry migration

`LayoutNode.Frame` and `LayoutNode.Content` are removed. Read geometry through
`res.Geometry[node.BoxID].Frame` and `.Content` instead. Each FlowLayout invocation
owns a fresh geometry table; success and failure leave the input tree, used values,
and previous results unchanged, provided callbacks honor the read-only contract.

`res.Root` shares the structural input rather than copying it. Keep that tree,
its styles, and published result tables immutable for their lifetime. Go does not
enforce this contract. Build a new tree for changed styles/structure. For resized
constraints, reuse the tree and call ResolveUsedValues again before FlowLayout;
changing LayoutContext alone does not recompute widths.

Adapters retain concrete paragraph results and paint per invocation, publishing
them with geometry only after complete success. Core tests exercise this ownership
pattern with a fake document adapter; real tytui publication remains integration
work. Resize, failed callbacks, atomic-callback immutability, and concurrent
read-only layouts have regression coverage. No caching is introduced.
