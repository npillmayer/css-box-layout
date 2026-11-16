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
- `display.go`: typed display values, validation, and supported display profile.
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
	Display: layout.Display{
		Kind: layout.DisplayBox, Outer: layout.OuterBlock, Inner: layout.InnerFlow,
	},
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

The builder reads typed display from `RenderNode.Display` and box values from
`RenderNode.Style`. It does not parse CSS syntax or apply cascade/defaults.
Elements require an explicit display; text nodes leave it zero. Supported values:

| Kind | Outer | Inner | Result |
| --- | --- | --- | --- |
| DisplayBox | OuterBlock | InnerFlow | Ordinary block |
| DisplayBox | OuterBlock | InnerFlowRoot | Block with an independent block formatting context |
| DisplayBox | OuterInline | InnerFlow | Ordinary inline |
| DisplayBox | OuterInline | InnerFlowRoot | Atomic inline-block |
| DisplayNone | OuterNone | InnerNone | Omit subtree |

`Display.Validate()` checks structure. All four outer/inner pairs are supported
for non-replaced elements. Flex/grid/table/ruby/list-item/contents are outside
this core input subset; adapters must reject them before projection. `InnerFlow` does not choose
between inline and block children; normalization still makes that decision.

`BuildLayoutTree` requires a non-replaced element root with block flow or block
flow-root display. Nil input or an element with `DisplayNone` returns `(nil, nil)`. Inline,
inline-block, replaced, text, missing-HTML and other non-element roots return
`ErrInvalidRoot`; no root is silently blockified. The supplied root keeps its
existing containment behavior. Filter non-element/non-text descendants in the
adapter. Suppression skips descendant and box-style validation, but the hidden
element's own display input must be valid.

`Styles["display"]` is rejected with `ErrInvalidDisplay`, even when empty or
accompanied by typed display. Missing or malformed element display is also an
error. `Styles` remains available for adapter metadata and invalid-string diagnostics;
known box-value strings still fail validation. Errors carry the source NodeID
and can be classified with `errors.Is`.

Display migration is complete across w3c-dom, CSS core and tytui. Tytui uses
GetDisplay and typed RenderNode.Display. The unused string accessor and tree-view
interface are removed; legacy display strings remain explicitly rejected.

`LayoutNode.EstablishesBFC` records the independent block formatting context for
non-replaced block/inline flow-root containers. Block flow-root stays `BoxBlock`
with normal block widths and outer flow; it is not an atomic inline item.
`FCBlock` continues to describe internal layout, and ordinary/anonymous blocks
do not inherit the new flag. Supplied-root containment remains separate.

Flow-root contains its children's margins, including collapsed empty-child
chains, and cannot collapse its own top/bottom margins through itself, even at
zero height. Its own margins can still collapse with adjacent blocks and with
an eligible ordinary parent; sibling collapse inside it also remains active.
See [CSS Display](https://www.w3.org/TR/css-display-3/#valdef-display-flow-root)
and [CSS 2.2 margin rules](https://www.w3.org/TR/CSS22/box.html#collapsing-margins).
This implements containment for the current in-flow subset; floats, clearance,
positioning, and explicit/min/max heights remain outside the profile.

For example, to supply box lengths:

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

`InlineResult.AllowsMarginCollapse` defaults to false, including at zero extent.
Set it true only when the paragraph has no separating line boxes (applying CSS
empty-line rules), no atomic items, and no consumed height. Core rejects true
with positive height or atomics, then checks owner edges and containment itself.
An empty structural group alone is not sufficient: adapters may reserve space.
Existing keyed result literals remain conservative; consuming adapters must
explicitly opt in for eligible empty output. Core collapses sibling and eligible
parent/child margins, including through empty blocks; supplied root, flow-root
and inline-block boundaries contain descendant margins. Atomic dimensions include collapsed
internal layout. Existing adapters without permission still prevent collapsing
through their inline groups.

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


Explicit replaced leaves: RenderNode.Replaced supplies ReplacedContent with a
finite nonnegative content Height in px; Style.Width must be fixed nonnegative
px. Only inline/inline-block display and no children are accepted. Build snapshots
this value into a childless BoxInlineBlock. Intrinsic width includes normal box
edges; final layout uses the explicit height without a paragraph callback/strut.
Payloads remain adapter/renderer-owned. No intrinsic image or ratio sizing is
implied. See replaced_test.go; ordinary empty containers retain their old behavior.

### CSS whitespace and empty lines

`ComputedStyle.WhiteSpace` carries normal, nowrap, pre, pre-wrap, pre-line or
break-spaces. Zero is normal; nil styles inherit the parent's computed mode.
`BuildOptions.ReadText` provides the exact UTF-8 range identified by a TextRef.
Without a reader, HTML.Data must contain that exact range. Incorrect lengths,
invalid UTF-8 and read errors fail construction. Input is never mutated.

Core derives anonymous-block suppression from text, whitespace mode and inline
edges. Collapsed-only runs vanish from mixed flow; preserved spaces/newlines,
NBSP, decorated inlines and atomics remain. Font size alone is not a barrier.
Explicit empty blocks keep their principal boxes and paragraph groups; the inline
service must return no line/height for empty CSS content, allowing margin collapse.

`text.NormalizeNormal` implements the supported normal-whitespace transformation
with byte source maps. Other modes require corresponding inline services; accepting
a typed mode for box generation does not implement preserved-text line layout.
The former WhitespaceParticipation/transparent-wrapper permission API is removed.
