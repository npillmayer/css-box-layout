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
func BuildLayoutTrees(root *RenderNode, opts BuildOptions) (*LayoutTrees, error)

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
`doc/Tree-Type.md` describes the current typed Display/Style/Text projection contract
without a string-style adapter interface. RenderNode.Style and Text are
snapshotted during construction; external source/paint lookup remains adapter-owned.

`RenderNode.Display` is a `Display{Kind, Outer, Inner}` value. Elements require
an explicit pair or subtree suppression; text leaves it zero. Legacy display
strings are rejected. The root must be a non-replaced block flow or block
flow-root element; nil and display:none yield no tree. `Display.Validate() error`
validates structure. Non-replaced block/inline flow-root containers carry
`LayoutNode.EstablishesBFC bool`; this contains descendant margins without
changing the box's outer participation or width rules. The flag is separate
from `FCBlock` and supplied-root containment, and is not inherited by wrappers.
Errors wrap `ErrInvalidDisplay`, `ErrUnsupportedDisplay` or `ErrInvalidRoot` and
identify the source NodeID. The tytui adapter uses this typed contract.

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

BuildOptions.ReadText supplies referenced source text during either builder.
LayoutOptions, ResolvePolicy, and LayoutPolicy are currently empty; FlowLayout's
context is unused by the current implementation. Do not infer supported behavior
from their presence. Future features may require interface changes,
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
    AllowsMarginCollapse bool
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

AllowsMarginCollapse defaults to false. True certifies no separating line boxes
under CSS empty-line rules and requires zero H and no atomic items. The core
rejects contradictory declarations, then combines permission with owner edges
and supplied-root/flow-root/inline-block containment. This eligibility contract is
implemented and used by collapsed placement. The tytui adapter grants permission
only for line-free zero-height output without atomics. Empty CSS-processed
paragraphs now produce exactly that output; principal block boxes remain.

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

<a id="fixed-layout-api"></a>
## 6) Fixed-layout client boundary (construction implemented)

PR 1 implements LayoutTrees and BuildLayoutTrees below. The separate resolver
and FixedLayout API are still proposals for PRs 2 and 3, not available Go API.
The [roadmap](Exec-Roadmap.md#fixed-layout-plan) records that sequence. All types
live in package `layout`.

```go
type LayoutTrees struct {
    Flow  *LayoutNode
    Fixed []*LayoutNode // fixed roots in source preorder; nested fixed roots diverted too
}

func BuildLayoutTrees(root *RenderNode, opts BuildOptions) (*LayoutTrees, error)
```

PositionKind supports PositionStatic (zero) and PositionFixed. ComputedStyle.Fixed
is an optional `*FixedStyle{Insets EdgeLengths, Height Length}`; it is accepted only
with PositionFixed. Width and ordinary box edges remain on ComputedStyle. Nil
Fixed means auto insets/height; DefaultFixedStyle supplies explicit auto defaults.
A non-nil zero record means explicit zero constraints. Both the style and fixed
record are snapshotted. Build validates length kinds, finiteness, and nonnegative
non-auto height, but does not resolve or reject both-auto axes. Negative insets
are valid. Legacy positioning/inset/height strings and positioned text are rejected.

ErrInvalidPosition classifies malformed positioning input; ErrUnsupportedPosition
classifies unknown schemes and fixed replaced roots. ErrFixedLayoutRequired guards
BuildLayoutTree against lost branches and both normal-flow pass entry points
against fixed participants, before measurement/final callbacks. Errors include
source NodeID during construction or BoxID during flow-input validation.

Proposed sizing/placement API, **not implemented**:

```go
type FixedLayoutOptions struct {
    FontSizePx float32 // explicit fallback where computed font size is absent
}

type FixedLayoutResult struct {
    Roots    []*LayoutNode
    Viewport Rect
    Geometry LayoutGeometryTable
}

func FixedLayout(
    roots []*LayoutNode,
    viewport Rect,
    inline InlineLayouter,
    intrinsic IntrinsicMeasurer,
    opts FixedLayoutOptions,
) (*FixedLayoutResult, error)
```

Construction and migration (implemented):

- Build both branches in one traversal with one BoxID allocator. Preserve source
  identity and inherited styles before diversion; allocate no duplicate subtree.
  Normalize each fixed interior independently, after diverting nested fixed boxes.
- Keep the existing supplied-root restrictions for ordinary flow. Permit an
  eligible non-replaced fixed root as a fixed-only input; its Flow result is nil.
  Nil/display:none input yields empty branches. Suppressed descendants stay absent.
- BuildLayoutTree preserves existing normal-flow behavior and returns an error
  if a fixed branch would be lost. Use BuildLayoutTrees for fixed input. Both
  ResolveUsedValues and FlowLayout reject fixed participants in hand-built input.
- Construction profile: non-replaced fixed roots with supported flow/flow-root
  interiors. Blockify their outer display without inserting them into parent flow.
  Position remains distinct from BoxKind; fixed roots set EstablishesBFC even
  when their original display was ordinary flow. Anonymous descendants stay static.

Sizing and invocation (proposed):

- FixedLayout orchestrates a dedicated internal fixed sizing resolver. Do not
  make clients manufacture UsedValuesTable entries or call the normal-flow width
  solver on fixed roots. Normal descendants can reuse existing resolution helpers
  after the fixed root's content width is final.
- Viewport coordinates must be finite and W/H nonnegative; zero is definite.
  Insets/dimensions use the appropriate axis; margin/padding percentages retain
  their existing width basis. Use a finite positive fallback FontSizePx when a
  computed font size needed for resolution is absent; do not invent a font size.
- Finalize fixed-root width and descendant widths before LayoutInline. Reuse
  IntrinsicMeasurer for shrink-to-fit, including nested atomic preparation, without
  publishing paragraph results during measurement. Determine content-based height
  from one final interior layout before resolving bottom anchoring. A prescribed
  root content height remains prescribed even when interior material overflows.
- In the first proposed profile, require at least one non-auto inset per axis.
  Reject a box needing static-position fallback, rather than assuming (0,0).
  Every fixed root, including one extracted from another fixed subtree, uses the
  supplied viewport. Ancestor-established fixed containing blocks are deferred;
  the adapter must reject unsupported inputs that depend on them.
- Validate branch shape, IDs and supported styles before final callbacks. Keep all
  derived values private. On any sizing/callback/geometry error, return nil result;
  the client discards this invocation's retained paragraphs. No partial root set
  is published. Empty roots return a successful empty result for a valid viewport.

Coordinates and lifetime (proposed):

- Fixed-root Frame and Content coordinates are viewport-local; RelativeTo is 0
  for each fixed root. The result's Viewport supplies the external origin. Add its
  X/Y once when mapping to client coordinates. Descendants retain the existing
  RelativeTo/parent-content convention; never translate every descendant directly.
- FixedLayoutResult distinguishes this reference space from LayoutResult's normal
  root space. Neither the normal-flow root nor a synthetic page is the parent.
  Geometry coverage inside each fixed subtree matches existing FlowLayout coverage;
  this proposal does not add line/text rectangles.
- Copy the input root slice for the result; structural nodes remain shared and
  read-only. Use a fresh geometry table per call. Calls with different rectangles
  share no derived state; identical calls are deterministic given the same services.
- Inline adapters retain paragraphs by group BoxID with OwnerID, as today. Each
  invocation owns its retained state. A client choosing to combine normal and
  fixed output coordinates publication itself; neither branch invokes the other.
  Root order is deterministic traversal order, not a promise about paint order.

Provisional client sequence (error handling elided):

```go
trees, err := layout.BuildLayoutTrees(renderRoot, buildOptions)
// The client may independently resolve/layout trees.Flow through the existing API.
fixed, err := layout.FixedLayout(
    trees.Fixed, viewport, inline, intrinsic,
    layout.FixedLayoutOptions{FontSizePx: 16},
)
// On success, retain fixed together with this call's adapter paragraph results.
```

This endpoint performs no clipping, painting, repetition, pagination, register
consumption, or page composition. Placement outside the viewport remains geometry
for the client to handle. There is no page index in this API.
