# layout

This package implements the core CSS box layout pipeline. It defines the layout tree (E), builds it from a styled render tree (D), and provides the entry points for resolving used values and running flow layout. The focus is correctness and clarity over optimization.

## Scope
- BuildLayoutTree: structural box tree construction (anonymous boxes, split+hoist).
- Used-values resolution (lengths, padding/border/margins).
- Flow layout entry points (block stacking + inline delegation).
- Shared types for layout nodes, geometry, and line boxes.

## Files (overview)
- `box.go`: core layout types (LayoutNode, BoxKind, geometry, edges).
- `boxid.go`: deterministic BoxID generation.
- `flow.go`: Pass 1 helpers (flow items, normalizeBlockChildren, split+hoist).
- `layout.go`: public entry points for layout passes.
- `interfaces.go`: interfaces for inline layout and intrinsic measurement.
- `render.go`: minimal render-node stub for BuildLayoutTree (to be replaced by CSSDOM adapter).
- `stubs.go`: temporary types/placeholders used during early implementation.
- `*_test.go`: unit tests for pass-1 behavior and invariants.

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
res, err := layout.ComputeLayoutWithConstraints(
	tree,
	inlineLayouter,
	intrinsicMeasurer,
	layout.ContainingBlock{}, // fill in constraints
	layout.LayoutOptions{},
)
if err != nil {
	// handle error
}
_ = res
```
