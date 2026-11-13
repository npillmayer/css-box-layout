# Pass Boundaries and Signatures

Reviewed against `layout/` on 2026-11-02. Signatures below describe the current
Go API; they are not a claim that implementation meets every requirement.
`doc/CSS-Algo-Overview.md` is the authoritative algorithm contract.
Open changes are tracked in `doc/Current-Checklist.md`.

## 1) Pass ownership

- BuildLayoutTree owns structural construction, anonymous wrappers, split/hoist,
  and B1-B4 normalization. It does not resolve used values or compute geometry.
- ResolveUsedValues owns edges and known content widths in a BoxID-keyed table.
  It does not change structure or position boxes.
- FlowLayout owns block stacking, inline delegation, and result geometry.
  It does not change structure. Auto inline-block sizing requires a still-open
  design for descendant resolution; current boundaries do not solve this by
  themselves (checklist tasks 2 and 3).

Core passes use explicit dependencies. FlowLayout leaves its tree and used
values unchanged and owns a fresh geometry table. LayoutResult.Root shares the
read-only structural tree; changed styles/structure require a new tree.

## 2) Current shared types

All layout types and interfaces currently live in package `layout`; text
references live in package `text`. Geometry and lengths use `float32`.

```go
type BoxID uint64
type NodeID uint64
type Rect struct { X, Y, W, H float32 }
type Edges struct { Top, Right, Bottom, Left float32 }

type LayoutNode struct {
    BoxID BoxID
    NodeID NodeID
    Box BoxKind
    FC FormattingContextKind
    Style *ComputedStyle
    Children []*LayoutNode
    Text text.TextRef
}

type UsedValues struct {
    Margin Edges
    Padding Edges
    Border Edges
    ContentWidth float32
    WidthPending bool
    EdgesPending bool
}
type UsedValuesTable map[BoxID]UsedValues

type LayoutGeometry struct { RelativeTo BoxID; Frame, Content Rect }
type LayoutGeometryTable map[BoxID]LayoutGeometry

type LayoutResult struct {
    Root *LayoutNode
    Geometry LayoutGeometryTable
}
```

`text.TextRef` contains `Source text.TextSourceID` and `Range text.TextRange`;
Start and End are `uint64` byte offsets. It is a value field used for BoxText.
Box kinds are BoxBlock, BoxInline, BoxText, BoxAnonymousBlock,
BoxAnonymousInline, and BoxInlineBlock.

IsBlockLevel classifies outer block flow and excludes inline-blocks.
IsBlockContainer includes inline-blocks for internal layout and width resolution.

## 3) Current entry points

```go
func BuildLayoutTree(root *RenderNode, opts BuildOptions) (*LayoutNode, error)

func ResolveUsedValues(root *LayoutNode, ctx ResolveContext) (UsedValuesTable, error)

func FlowLayout(
    root *LayoutNode,
    used UsedValuesTable,
    inline InlineLayouter,
    intrinsic IntrinsicMeasurer,
    ctx LayoutContext,
    opts LayoutOptions,
) (*LayoutResult, error)
```

`ComputeLayoutWithConstraints` has the same signature as FlowLayout and forwards
to it as a compatibility wrapper. BuildLayoutTree uses the RenderNode stub;
`doc/Tree-Type.md` describes the current typed Style/Text projection contract
and the unfinished read-only adapter proposal. RenderNode.Style and Text are
snapshotted during construction; external source/paint lookup remains adapter-owned.

```go
type ResolveContext struct {
    ContainingBlock Rect
    FontSizePx float32
    Policy ResolvePolicy
}
type LayoutContext struct {
    ContainingBlock Rect
    Policy LayoutPolicy
}
```

BuildOptions, LayoutOptions, ResolvePolicy, and LayoutPolicy are currently empty.
BuildOptions, LayoutOptions, and FlowLayout's context are unused by the current
implementation. Their intended roles need review; do not infer supported
behavior from their presence. Future features may require interface changes,
not just policy fields.

## 4) Inline integration - task 2A implemented

Fixed CSS-pixel atomic widths (including zero) and auto widths are supported.
Percent/em widths on atomic boxes remain rejected; descendant percentages work
with definite bases. Unknown-auto-width percentage dependencies fail explicitly.
Core prepares atomic subtrees before outer delegation without sizing callbacks or
external reentrancy. The intrinsic service is used for auto width only.

### Prepared atomic items and placement

The following interfaces are implemented. The older AtomicSizer and extent-only method signature are removed.

```go
type InlineExtent struct { W, H float32 }
type PreparedAtomic struct {
    BoxID BoxID
    BorderWidth, BorderHeight float32
}
type PreparedAtomicsView interface {
    Lookup(BoxID) (PreparedAtomic, bool)
}
type InlineRequest struct {
    Group *LayoutNode
    OwnerID BoxID
    ContentWidth float32
    Used UsedValuesView
    Atomics PreparedAtomicsView
}
type AtomicPlacement struct {
    BoxID BoxID
    X, Y float32
}
type InlineResult struct {
    Extent InlineExtent
    Atomics []AtomicPlacement
}
type InlineLayouter interface {
    LayoutInline(InlineRequest) (InlineResult, error)
}
```

One read-only view serves one group. Lookup uses the atomic branch's existing
BoxID and returns a copy; items inside an atomic boundary belong to inner groups.
The adapter extracts ordinary inline text but replaces atomic branches with ordered
whatsits. Local handles map back to BoxID; source mapping, item encoding, retained
paragraphs, and position storage remain adapter-owned. Missing preparation is an
error, not an invitation to initiate layout.

Extent is finite/nonnegative; H includes paragraph trailing space, W does not
resize the owner. Every prepared item requires exactly one placement, including
zero-sized items. Unknown, foreign, duplicate or missing IDs and non-finite
coordinates fail with nil LayoutResult. Negative coordinates are valid. Core
validates the entire placement answer before applying it and retains no slice
alias. Paragraph errors remain wrapped with group/owner identity.

Placement X/Y is the border-box origin relative to request.OwnerID content, after
margin/alignment effects. Core adds no further margin, preserves dimensions, and
sets Content origin by adding own border/padding. RelativeTo names the geometry
coordinate parent: owner for atomics, containing block for stacked children, zero
for root. Descendant-local geometry is not translated; compose each parent content
origin once. No core paragraph lines, glyphs or opaque payloads are returned.

Trees, styles, used tables and prior results remain unchanged. Publish geometry
and invocation-local typed results together only after whole-document success.
The fake adapter uses explicit inter-item breaks, indivisible whatsits, overflow
on an empty line, and bottom-margin-edge baseline alignment. These are bounded
test policies, not full CSS baseline support or real P integration.

## Intrinsic measurement

Implemented under tasks 2B/3B. MaxContentWidth and InlineIntrinsic are removed;
external providers implement the measurement-only operation below. Fixed-width
flow does not require a measurer.

```go
type IntrinsicWidths struct {
    MinContent float32
    MaxContent float32
}
type IntrinsicAtomic struct {
    BoxID BoxID
    BorderWidths IntrinsicWidths
}
type IntrinsicAtomicsView interface {
    Lookup(BoxID) (IntrinsicAtomic, bool)
}
type BoxEdges struct {
    Margin  Edges
    Padding Edges
    Border  Edges
}
type ResolvedEdgesView interface {
    Lookup(BoxID) (BoxEdges, bool)
}
type InlineMeasureRequest struct {
    Group   *LayoutNode
    OwnerID BoxID
    Edges   ResolvedEdgesView
    Atomics IntrinsicAtomicsView
}
type IntrinsicMeasurer interface {
    MeasureInline(InlineMeasureRequest) (IntrinsicWidths, error)
}
```

Widths are CSS pixels, finite and nonnegative, with MinContent <= MaxContent.
MeasureInline returns group content bounds, excluding the owner's edges/margins.
Each IntrinsicAtomic carries border-box bounds including its own padding/borders
but excluding margins. The measurer adds that atomic item's horizontal margins
from Edges exactly once, not its padding/borders again. It handles ordinary inline
span edges within the group. Core accounts for owner edges when constructing an
atomic contribution and combines internal block contributions (maximum horizontal
contribution in the supported normal-flow subset, not vertical-stack summation).

There is one read-only atomic view per group; node.BoxID is the lookup key and
returned BoxID matches it. Core recursively measures atomic boundaries before
outer measurement, without reentrant external calls. Fixed-width items have equal
border bounds; auto items derive their bounds from recursively measured content.
Nested interiors belong to separate requests, not the outer mixed input. Local
whatsit handles may map to BoxID exactly as during final layout.

Min-content measurement uses the supported minimum-content breaking rules and
atomic minimum contributions. Max-content suppresses optional wrapping, preserves
mandatory breaks and uses atomic maximum contributions. At either bound an atomic
item is indivisible to the outer paragraph. Text semantics and typography inputs
must match final layout; source/style lookup is explicit adapter-owned state.

There is no available-width field and no fabricated infinite constraint. Edges
exposes only independently resolved values, never a provisional ContentWidth.
Lookup returns copies; views and measurements are invocation-scoped. Missing
items/edges, invalid bounds and unsupported dependencies fail explicitly. No
heights, baselines, geometry, placements or retained paragraph payloads are
produced or published by measurement. Internal typography scratch work is allowed
but cannot replace accepted layout results.

Core selects content width using min(max(minContent, availableContent), maxContent).
Available content width is the containing block's content width minus the atomic
box's horizontal margins, padding and borders, clamped to zero; it is not remaining
line space. Then task 3B records the selected width in invocation-local used values,
re-resolves descendants, and performs one accepted layout. Intrinsic min width may
force overflow. Empty content may have zero bounds; zero is not unresolved state.

This adopts bounded shrink-to-fit instead of the earlier max-content-only plan.
Percentage dependencies needing an unknown auto width are rejected initially,
not evaluated against zero. Percentages with a definite basis, including descendant
resolution after width selection, remain a distinct supported operation. The
UsedValues.WidthPending and EdgesPending flags distinguish provisional values
from resolved zero. ResolveUsedValues preserves resolution context internally;
FlowLayout copies the table, selects widths, re-resolves descendants, and finishes
all late resolution before any LayoutInline call. Measurement receives only
resolved edges, not pending content widths. Invocation-local content bounds are
reused for nested auto items, but there is no cross-invocation cache. Full CSS intrinsic sizing, speculative layout
trials/caching and full baseline behavior remain outside this slice.

## 5) Package organization and remaining decisions

Current organization: construction, resolution, flow, and integration interfaces
are all in `layout/`, which imports `text/`. A future split into `boxtree/`,
`flow/`, shared types, and integration interfaces is provisional. Shared type
ownership must be designed to avoid import cycles before splitting packages.

Checklist task 7 covers API/document alignment, input immutability, geometry
coverage, and numeric representation. Tasks 2 and 3 cover explicit auto-width
state and descendant resolution. Geometry exists only in LayoutResult.Geometry;
LayoutNode.Frame/Content have been removed. Published trees, styles, and result
tables must remain read-only for their lifetime, not only during a layout call.
