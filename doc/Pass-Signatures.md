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

Core passes are intended to be pure transforms with explicit dependencies.
Current FlowLayout mutates nodes; this remains an implementation/design gap.

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
    Frame Rect
    Content Rect
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
on failure, but node mirror mutation remains a task-11 gap. Atomic sizing still
returns border-box dimensions and intrinsic measurement content width; the later
inline-block fixes remain necessary. The legacy glyphing sketches are not this
public paragraph boundary.

## 5) Package organization and remaining decisions

Current organization: construction, resolution, flow, and integration interfaces
are all in `layout/`, which imports `text/`. A future split into `boxtree/`,
`flow/`, shared types, and integration interfaces is provisional. Shared type
ownership must be designed to avoid import cycles before splitting packages.

Checklist task 7 covers API/document alignment, input immutability, geometry
coverage, and numeric representation. Tasks 2 and 3 cover explicit auto-width
state and descendant resolution. Any optional geometry mirrors must preserve
the authoritative table and the agreed input ownership rules.
