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
- FlowLayout owns block stacking, inline delegation, and result geometry/lines.
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
type LinesByBlock map[BoxID][]LineBox

type LayoutResult struct {
    Root *LayoutNode
    Geometry LayoutGeometryTable
    Lines LinesByBlock
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
`doc/Tree-Type.md` describes the unfinished read-only adapter proposal.

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
type InlineLayouter interface {
    LayoutInline(inlineRoot *LayoutNode, maxWidth float32,
        atomic AtomicSizer) ([]LineBox, error)
}
type AtomicSizer interface {
    SizeInlineBlock(node *LayoutNode, maxWidth float32) (w, h float32, err error)
}
type IntrinsicMeasurer interface {
    MaxContentWidth(node *LayoutNode) (float32, error)
}
```

The inline root is BoxAnonymousInline. Line frames are relative to the owning
block content box; the inline layouter supplies stacked Y coordinates. Atomic
sizing returns border-box dimensions; intrinsic measurement returns content
width. Resolved edges must be available before the corresponding line breaking.
The inline interface currently receives no used-values table explicitly;
defining how the adapter consumes resolved inline edges remains part of the
integration design. Do not rely on hidden mutable global state.

## 5) Package organization and remaining decisions

Current organization: construction, resolution, flow, and integration interfaces
are all in `layout/`, which imports `text/`. A future split into `boxtree/`,
`flow/`, shared types, and integration interfaces is provisional. Shared type
ownership must be designed to avoid import cycles before splitting packages.

Checklist task 7 covers API/document alignment, input immutability, geometry
coverage, and numeric representation. Tasks 2 and 3 cover explicit auto-width
state and descendant resolution. Any optional geometry mirrors must preserve
the authoritative table and the agreed input ownership rules.
