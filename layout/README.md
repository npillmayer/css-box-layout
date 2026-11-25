# layout

This package implements the core CSS box layout pipeline. It defines the layout tree (E), builds it from a styled render tree (D), and provides the entry points for resolving used values and running flow layout. The focus is correctness and clarity over optimization.

## Scope
- BuildLayoutTree: structural box tree construction (anonymous boxes, split+hoist).
- InlineGroups: pure enumeration of normalized groups, owners and atomic boundaries.
- Used-values resolution (lengths, padding/border/margins).
- Standalone intrinsic border-box measurement without final layout or geometry.
- Flow layout entry points (block stacking + inline delegation).
- Finite-height structural fragments and exact continuation through paragraph callbacks.
- Shared types for layout nodes, geometry, and inline extent requests/results.

## Files (overview)
- `box.go`: core layout types (LayoutNode, BoxKind, geometry, edges).
- `boxid.go`: deterministic BoxID generation.
- `build_style.go`: typed style validation, snapshots, and anonymous inheritance.
- `build_error.go`: structured source/root locations and preserved error causes.
- `display.go`: typed display values, validation, and supported display profile.
- `flow.go`: Pass 1 helpers (flow items, normalizeBlockChildren, split+hoist).
- `groups.go`: read-only normalized group enumeration for preparation/indexing.
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

`MeasureIntrinsic(tree, resolveContext, intrinsicMeasurer)` exposes the same
recursive intrinsic pass used by auto sizing. It returns CSS-pixel border-box
bounds excluding the supplied root's margins. Definite widths contribute their
prescribed width plus padding/borders; auto widths recurse. The context supplies
a definite percentage basis for the root and a fallback font size. No final
heights, used-value table or geometry are produced. Each call owns fresh scratch;
callbacks borrow read-only views and errors return zero widths with causes intact.
The tree must be normalized and obey the existing normal-flow atomic profile.

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
known box-value strings still fail validation. Construction errors carry structured
source identities and can be classified with `errors.Is`.

### Construction diagnostics and independent trees

`BuildLayoutTree` and `BuildLayoutTrees` return `*BuildError` on failure, with no
partial tree/result. `RootID` is the supplied render root's source NodeID;
`HasNode` and `NodeID` identify the offending source when one applies. Check
`HasNode` rather than comparing NodeID with zero: callers may supply zero IDs.
An allocation-wide failure has only the tree boundary, not an invented source
for an anonymous box. Display/style/text/replaced/fixed diagnostics use the
relevant source; nested failures are not relabeled as their outer ancestors.

```go
var construction *layout.BuildError
if errors.As(err, &construction) {
    if construction.HasNode {
        // Map construction.NodeID to the adapter's original source record.
    } else {
        // Report the input branch identified by construction.RootID.
    }
}
// errors.Is(err, layout.ErrInvalidDisplay) and errors.As for a ReadText
// callback's error still work through BuildError.Unwrap().
```

The error retains IDs and its cause, not input DOM/render/layout nodes. The
caller's cause may itself retain application data. This contract applies to
construction only; direct value validators and geometry/layout errors keep their
existing error types. Diagnostic prose is not an API to parse.

Adapters may filter `RenderNode.ChildrenNodes` into independent projections and
call the existing builder on each block root. Filter before normalization so
residual inline runs become adjacent and empty decorated ancestors remain.
Snapshot effective root fonts/whitespace and other source styles before
detaching: the builder does not rediscover the original cascade. It preserves
source NodeIDs and TextRefs, snapshots typed values and leaves inputs unchanged.
Named channels and cross-tree ownership remain adapter responsibilities; no
channel callback or forest builder is required.

BoxIDs are unique across flow/fixed roots within one `BuildLayoutTrees` call,
not across independent calls. Identical builds can reuse numerical IDs; edits
can change allocation. Use `(caller tree identity, BoxID)` for retained boxes,
and original NodeIDs/TextRefs for source correspondence. Anonymous boxes have
NodeID zero; split inline fragments can share a source NodeID. Tests in
`partition_test.go` cover detached source context, filtering, copied values,
failed later builds and concurrent read-only construction.

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

## Enumerating normalized groups

`InlineGroups(root) iter.Seq[InlineGroup]` visits actual `BoxAnonymousInline`
groups in structural preorder. Each descriptor contains `Group`, its nearest
block-container `Owner`, immediate `Atomics` in inline order, and the nearest
enclosing `ParentAtomic` (nil outside an inline-block). Owners may be anonymous
blocks. Ordinary inline elements are not separate groups.

```go
for group := range layout.InlineGroups(tree) {
    // Retain identities with the caller's tree/snapshot identity.
    fmt.Println(group.Group.BoxID, group.Owner.BoxID, len(group.Atomics))
}
```

The outer group precedes groups inside its atomics. Its `Atomics` list stops at
each inline-block boundary; nested interiors are enumerated separately. Empty
groups are included. Replaced leaves appear as atomics but have no interior
group. The iterator does not reconstruct runs omitted during normalization.
See the executable [example](groups_example_test.go).

Supply a normalized block-container tree or subtree. Nil yields nothing;
ancestors outside the supplied root are ignored. An inline-block root encloses
its own interior groups. This is a traversal, not a malformed-tree or cycle
validator, and performs no text processing, style resolution or sizing.

All node pointers borrow the input tree: nodes, styles and child slices must
remain read-only while descriptors are used. Each returned `Atomics` slice is
independent caller-owned storage; changing that slice does not change traversal.
The iterator is repeatable after early termination and supports concurrent reads
of an immutable tree. BoxIDs remain local to their construction call; enumeration
neither assigns IDs nor establishes identity across independent trees.

## Inline adapter migration

Implement `LayoutInline(InlineRequest) (InlineResult, error)`. AtomicSizer is
removed. Core resolves and lays out atomic interiors before outer delegation;
`req.Atomics.Lookup(node.BoxID)` returns copied border-box dimensions for that
branch. Do not flatten its inner text or initiate sizing. Retain paragraphs by
Group.BoxID with OwnerID; local whatsit handles map to atomic BoxID.

Return full finite nonnegative Extent plus exactly one AtomicPlacement per
realized item. X/Y denotes the border origin relative to OwnerID content, already
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


### Fixed boxes: independent layout against a viewport

Set `ComputedStyle.Position = PositionFixed` and call `BuildLayoutTrees` to obtain
`LayoutTrees{Flow, Fixed}`. Fixed roots (including nested fixed elements) are
separate from normal flow before normalization and share the build's BoxID space.
Use `trees.Flow` with the existing resolver and FlowLayout. Call the fixed branch
independently, supplying its viewport and services:

```go
fixed, err := layout.FixedLayout(trees.Fixed, viewport, inline, intrinsic,
    layout.FixedLayoutOptions{FontSizePx: 16})
```

FixedLayout resolves root constraints and descendant widths internally, then
lays out each interior once. Content-based height completes bottom anchoring;
explicit/stretch height remains fixed even if its contents overflow. Root
Frame/Content coordinates are viewport-local (`RelativeTo == 0`); add the returned
Viewport origin once externally. Descendants stay relative to parent content.
The result owns its root slice and geometry table, sharing read-only nodes.
On error, discard the invocation's adapter results; no partial geometry is returned.

Optional `ComputedStyle.Fixed` holds `FixedStyle{Insets, Height}`. Nil means auto;
start from `DefaultFixedStyle()` to override selected constraints. Explicit zero
values remain zero. Width and box edges stay on ComputedStyle. Position and fixed
constraints are not inherited, and input styles are copied. Fixed roots currently
support non-replaced elements using the supported display pairs. Each axis needs
at least one non-auto inset; static-position fallback is unsupported. Viewport
W/H must be finite and nonnegative; zero is definite. The fallback font size must
be nonnegative, and em lengths need a positive effective font size. No page,
repetition, clipping or paint-order rules are applied.

`BuildLayoutTree` reports `ErrFixedLayoutRequired` instead of discarding fixed
roots. ResolveUsedValues and FlowLayout reject fixed participants before callbacks.
Malformed fixed values use `ErrInvalidPosition`; unsupported schemes or fixed
replaced elements use `ErrUnsupportedPosition`. See `ExampleBuildLayoutTrees` in
[position_example_test.go](position_example_test.go) for construction and
`ExampleFixedLayout` for independent calls with invocation-owned adapter results.

### Fixed geometry without content (BOX-01)

`ResolveFixedGeometry(style, viewport, opts)` accepts `ComputedStyle` directly
and returns one `LayoutGeometry`. It shares typed constraint preparation, axis
equations and rectangle construction with `FixedLayout`, but needs no DOM,
layout tree, BoxIDs, intrinsic measurer or inline layouter. See
[`ExampleResolveFixedGeometry`](fixed_geometry_example_test.go) for a complete
example using default constructors and the resulting border/content rectangles.

The style must use `PositionFixed`, with at least one definite inset per axis.
An explicit content size or opposing-inset stretch resolves without content;
auto size with one definite inset is a content dependency, even for an empty box
or a zero-size viewport. Px, percentages with viewport bases, and em with a
positive effective font size follow the existing fixed contract. Zero sizes,
negative margins/insets, auto margins, overconstraints and finite overflow remain
valid core inputs. Callers enforce stricter receiving-frame or bounds policies.

Errors are classifiable with `errors.Is`: `ErrInvalidFixedGeometry` for malformed
input, unresolved font context or arithmetic overflow; `ErrUnsupportedPosition`
for non-fixed positioning or both-auto static-position fallback;
`ErrFixedContentDependency` for unresolved content sizing. Wrapped validation
causes are preserved. Every failure returns a zero geometry value.

The result is detached and viewport-local (`RelativeTo == 0`). Apply the external
viewport origin once during composition. Inputs and prior results remain
unchanged across concurrent calls and resize/replay. `FixedLayout` retains its
existing content measurement, interior layout and callback lifecycle.

## Inspect normalized source boundaries

Use `BuildLayoutTreesWithProvenance` when source positions must survive anonymous
wrapping, inline splitting and block hoisting:

```go
trees, provenance, err := layout.BuildLayoutTreesWithProvenance(renderRoot, layout.BuildOptions{})
if err != nil {
    return err
}
for _, source := range provenance.Sources() {
    if source.Before.Disposition == layout.BoundaryRetained {
        position := source.Before.Position
        fmt.Println(source.Source, position.Root, position.Box, position.Group, position.Order)
    }
}
_ = trees
```

Each input `NodeID` must be unique within this build, including suppressed
subtrees; zero is permitted. Source records follow input preorder. Before/after
positions enclose the source's emitted flow, including hoisted blocks. Retained
occurrences list boxes actually bearing that source ID, so a split inline can
have several occurrences and a hoisted-only inline can have none. Generated
anonymous boxes have structural positions but no invented source identities.

`Boundaries()` returns depth-first before/after positions for every box. Order
is one-based and restarts within each root; compare it only for positions in the
same root and build. Flow precedes fixed roots in the returned inventory. These
positions describe structure, not legal line/page cuts. `Group` identifies a
containing anonymous inline group; atomic interiors have their own groups.
Existing `TextRef` byte ranges describe retained text without text processing.

Check each endpoint's disposition. `BoundaryOmitted` means normalization dropped
its emitted endpoint, such as a whitespace-only mixed-flow run; `BoundaryEmpty`
means the source emitted no boxes, such as zero-length text; `BoundarySuppressed`
means display suppression. These have no fabricated position or nearest-box
fallback. Even an empty inline may be retained in an inline-only group but omitted
between blocks. Choosing affinity or mapping to processed text is the caller's
separate responsibility. Sources filtered out before this build are absent.

All returned records and slices are independent value copies without mutable
node pointers. Qualify identities by your build/tree snapshot. Errors return no
trees or provenance and retain `BuildError` source locations and original causes.

## Fit a structural fragment and resume

Create a `StructuralFitter` from a normalized block root and a `ParagraphFitter`.
The supported profile is static auto-width blocks with zero margins, borders and
padding, containing nested blocks or ordinary inline/text groups. Specified block
widths, nonzero/unresolved edges, positioned/replaced boxes and inline atomics
return `ErrUnsupportedFit`.

```go
fitter, err := layout.NewStructuralFitter(tree, paragraphFitter)
if err != nil {
    return err
}
var choices []layout.FitAlternative
status, err := fitter.Enumerate(ctx, layout.FitRequest{
    Width: 320, Height: 480, // finite, nonnegative CSS pixels
}, func(a layout.FitAlternative) bool {
    choices = append(choices, a)
    return true
})
if err != nil {
    return err
}
if status != layout.FitExhausted {
    // Interrupted: choices are valid snapshots, but enumeration is incomplete.
    return fmt.Errorf("fitting interrupted")
}
if len(choices) == 0 {
    // No legal progressing prefix fits these dimensions.
    return nil
}
selected := choices[0] // apply the client's selection policy here
fmt.Println(selected.Extent(), selected.Progress(), selected.Complete())
next := layout.FitRequest{
    Width: 240, Height: 480, Continuation: selected.Continuation(),
}
_ = next // pass to the same fitter; the accepted prefix remains unchanged
```

`ParagraphFitter.EnumerateParagraph` receives copied group/owner/root identities,
receiving dimensions, a logical `From` and the selected opaque checkpoint. Supply
all legal whole-line prefixes, including shorter choices. Partial choices must
advance their width-independent `ParagraphPosition` and provide a checkpoint;
complete choices consume the remaining group and have no checkpoint. Each choice
includes its exact extent and matching output. An empty group may complete with
zero extent and unchanged paragraph position; consuming it still advances the
structural cursor. Oversized prefixes are filtered as non-fit.

Checkpoint and output types implement `FitPayload.Clone`. Return an independent
snapshot, or the same value only when immutable. Core clones on receipt, output
inspection and checkpoint resumption. Do not retain mutable callback scratch in
a clone. The service owns text preparation and line breaking; core does not
reconstruct checkpoints from offsets. See the runnable
[changed-width example](fit_example_test.go).

Enumeration is depth first. Within each paragraph, complete choices come first,
then descending logical ends, with provider order preserved for ties. Extensions
precede their shorter block prefix. Per-group choices are collected and frozen
before traversal. `IncludeEmpty` adds an unchanged, non-progress alternative last;
an already completed or stopped cursor is reported even without that option.
`Paragraphs()` returns copied outputs at `(0,Y)` in the receiving rectangle.
`Containers()` describes first/last fragments; continuation inspection lists
open ancestors, their next children and pending opening/closing facts.

Use `fitter.Boundary(boxID, edge)` for an optional `FitRequest.Stop`. Block and
anonymous-group boundaries are supported; cuts inside inline material return
`ErrUnsupportedFit`. A trailing child's after-cut includes closing ancestors.
Boundaries and continuations belong to exactly one fitter, even when another
fitter has the same numerical BoxIDs. `End().Compare` compares progress only
within that identity; positions are not continuation authority.

A false visitor or voluntary service stop returns `FitStopped`. Context errors,
`MaxSteps` limits, malformed callback results and backend failures return
`FitFailed` with an error. Only `FitExhausted` certifies full enumeration; an empty
exhaustive result is ordinary non-fit. `FitError` identifies the box and preserves
causes for `errors.Is`/`errors.As`. Limits and cancellation are cooperative between
structural steps and paragraph yields/calls. Results retain no source nodes or
paragraph service; concurrent use requires a concurrency-safe service.
