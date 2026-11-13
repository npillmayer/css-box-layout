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
}
type UsedValuesTable map[BoxID]UsedValues

type LayoutGeometry struct { Frame, Content Rect }
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

Current `IsBlockLevel` includes BoxInlineBlock: it must not be used unchanged
as an outer-flow classification predicate for B3 validation. Inline-blocks are
atomic inline participants and internal block containers.

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

## 4) Inline integration

```go
type InlineExtent struct { W, H float32 }
type UsedValuesView interface {
    Lookup(BoxID) (UsedValues, bool)
}
type InlineRequest struct {
    Group *LayoutNode
    OwnerID BoxID
    ContentWidth float32
    Used UsedValuesView
    Atomic AtomicSizer
}
type InlineLayouter interface {
    LayoutInline(InlineRequest) (InlineExtent, error)
}
type AtomicSizer interface {
    SizeInlineBlock(node *LayoutNode, maxWidth float32) (w, h float32, err error)
}
type IntrinsicMeasurer interface {
    MaxContentWidth(node *LayoutNode) (float32, error)
}
```

Group is BoxAnonymousInline; Group.BoxID and OwnerID identify existing boxes.
ContentWidth is definite CSS-pixel width, including zero. Extent W/H are finite,
nonnegative overall layout dimensions, not ink bounds. H sets owner content
height, including paragraph trailing space; W does not change allocated width.
Owner edges and margins are accounted for separately. Empty groups may reserve
height. Used.Lookup returns copies; callbacks must not mutate Group or input data.

This replaces the previous line-return API. LayoutResult.Lines, layout.LineBox,
and LinesByBlock are removed. A concrete paragraph result stays in an adapter-owned
typed table keyed by Group.BoxID with OwnerID. Publish that table with successful
geometry as a document snapshot; core carries no opaque payload. Baselines and
precise glyph positions are private to the retained paragraph result. The adapter
owns conversion into CSS-pixel extents; its exact DU/point rounding policy is an
integration task, not a new baseline convention in the core.

Missing used values and invalid widths/edges fail before callbacks. Invalid extents
fail, and callback errors are wrapped with group/owner identity. Flow returns nil
on failure without mutating inputs or earlier results, provided callbacks honor
the read-only contract. Atomic sizing is specified to return border-box dimensions
and intrinsic measurement content width; the later inline-block fixes remain
necessary. Glyph representations and line-breaking results belong to external
typography services, not to a separate package in this module.

### Concrete task-2A proposal (not implemented)

The signatures above remain the current Go API. The following proposed replacement
records the agreed scheduling and BoxID association; it is not implemented code.

```go
type PreparedAtomic struct {
    BoxID        BoxID
    BorderWidth  float32
    BorderHeight float32
}

type PreparedAtomicsView interface {
    Lookup(BoxID) (PreparedAtomic, bool)
}

type InlineRequest struct {
    Group        *LayoutNode
    OwnerID      BoxID
    ContentWidth float32
    Used         UsedValuesView
    Atomics      PreparedAtomicsView // replaces AtomicSizer
}

type AtomicPlacement struct {
    BoxID BoxID
    X, Y  float32
}

type InlineResult struct {
    Extent  InlineExtent
    Atomics []AtomicPlacement
}

type InlineLayouter interface {
    LayoutInline(InlineRequest) (InlineResult, error)
}

// Proposed extension; the current geometry type has no RelativeTo field.
type LayoutGeometry struct {
    RelativeTo BoxID
    Frame      Rect
    Content    Rect
}
```

There is one prepared view per group/request, not per atomic node. Core finishes
atomic subtrees before outer delegation and indexes each measured item by that
node's existing BoxID. The adapter associates a branch using
`req.Atomics.Lookup(node.BoxID)`, not a node pointer, NodeID, or rope offset.
Lookup returns copies; returned BoxID matches the key. The view includes atomic
boundaries reached through ordinary inline descendants, but not items inside
those boundaries. Empty groups have an empty view. Data is invocation-scoped and
read-only; missing entries are errors. Border dimensions are finite/nonnegative;
margins are separate in Used.Lookup. Core does not expose retained paragraph data.

The adapter skips atomic contents when extracting text, but inserts an ordered
whatsit anchor. It may use BoxID directly or map a local handle back to BoxID.
Rope placeholders, attachment tables, mixed-item representation, and internal
position storage remain adapter choices; source mapping is explicit. Core never
interprets those representations. P need not import core types.

The output deliberately refines the earlier extent-only contract: extent plus
placements of core-owned atomic boxes, never paragraph lines/glyphs or opaque
payloads. AtomicPlacement.X/Y is the border-box origin in CSS pixels relative to
request.OwnerID's content origin, after margin/alignment effects. No second margin
addition occurs in core. Dimensions cannot be changed by the adapter. Exactly one
placement per prepared item (including zero-sized items) is required; reject
missing, duplicate, foreign-group or unknown IDs and non-finite coordinates.
Negative coordinates are permitted. Validate the full result before applying it.

RelativeTo names the geometry coordinate parent, not necessarily the structural
parent: atomic entries use request.OwnerID, stacked blocks use their containing
block, and the root uses zero. Core sets Frame.X/Y to the accepted placement and
Content.X/Y to that origin plus own border/padding, preserving dimensions and all
descendant-local rectangles. This makes placement part of authoritative geometry;
retain no slice alias to adapter output and never reapply it during composition.
Build final geometry privately and publish only after complete success.

Core recursion finishes inner external calls before outer delegation; adapter
reentrancy is not required. Checklist 2A/3A tracks implementation. P's concrete
whatsit API and conversion remain integration work. The proposed initial bounded
alignment policy is bottom margin edge to the line baseline, without exporting
an inner paragraph baseline; confirm that fixture policy before coding. Full CSS
baseline alignment, auto sizing and speculative trials remain follow-up work.

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
