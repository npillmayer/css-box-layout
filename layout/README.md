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

Implement `LayoutInline(req layout.InlineRequest) (layout.InlineExtent, error)`.
Use `req.Group.BoxID` as the typed paragraph retention key and `req.OwnerID` for
placement through the owning block's content geometry. `req.ContentWidth` is
CSS-pixel content width; `req.Used.Lookup(id)` exposes resolved value copies.
Treat the group and caller inputs as read-only.

Return the full finite, nonnegative layout extent, including paragraph-owned
trailing space. H determines flow height; W does not resize the containing block.
Zero width and empty results are valid; empty groups may reserve nonzero height.
Do not return lines or a payload. `layout.LineBox`, `LinesByBlock`, and
`LayoutResult.Lines` are removed. Paragraph lines/baselines/glyphs belong in the
adapter's concrete result table, which must include anonymous groups too.

Create that table per invocation and publish it with successful document geometry
only. Discard partial adapter state after errors. Nested coordinates now use parent-content-relative placement. Core snapshot
purity remains separately tracked; nil LayoutResult on error does
not yet guarantee unchanged input-node mirrors. Atomic inline sizing remains
unfinished. The extent-only tests use normal block flow and fake typed paragraphs.
