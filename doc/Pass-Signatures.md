# Pass Boundaries and Signatures (Provisional)

This document freezes the **pass boundaries** and **function signatures** for the core pipeline. It also proposes a **provisional package layout** aligned with those passes. External APIs will adapt to these signatures, not the other way around.

Authoritative algorithm contract: `doc/CSS-Algo-Overview.md`.

---

## 1) Pass boundaries (what each pass owns)

### Pass 1: BuildLayoutTree (D -> E)
Owns:
- Structural box tree construction.
- Anonymous box insertion for mixed inline/block runs.
- Split + hoist when inline contains block.
- B1-B4 invariants on the resulting tree.

Does not own:
- Used values resolution.
- Geometry.
- Line breaking.

### Pass 2: ResolveUsedValues (E -> used values)
Owns:
- Resolving margins/padding/borders and used widths.
- Producing a `UsedValuesTable` keyed by `BoxId`.

Does not own:
- Structural tree changes.
- Geometry or positioning.

### Pass 3: FlowLayout (E + used values -> F)
Owns:
- Block layout (vertical stacking, no margin collapsing yet).
- Inline layout delegation to the inline layouter.
- Producing layout geometry and line boxes.

Does not own:
- Structural tree changes.
- Used values resolution.

---

## 2) Shared types (minimal, stable)

The following types are shared across passes. Keep them small and stable.

Identity:
- `type BoxId uint64` (unique per box)
- `type NodeId uint64` (source DOM/render id; may be shared across fragments)

Geometry:
- `type Rect struct { X, Y, W, H float64 }`
- `type Edges struct { Top, Right, Bottom, Left float64 }`

Text:
- `type TextRef struct { Source TextSourceId; Start, End uint64 }`

Box kinds:
- `type BoxKind enum { BoxBlock, BoxInline, BoxText, BoxAnonymousBlock, BoxAnonymousInline, BoxInlineBlock }`
- `func IsBlockLevel(kind BoxKind) bool`

Structural node:
- `type LayoutNode struct {
    BoxId   BoxId
    NodeId  NodeId
    Kind    BoxKind
    Style   *ComputedStyle
    Children []*LayoutNode
    Text    *TextRef
  }`

Tables:
- `type UsedValuesTable map[BoxId]UsedValues`
- `type LayoutGeometryTable map[BoxId]LayoutGeometry`
- `type LinesByBlock map[BoxId][]LineBox`

Inline layout interfaces:
- `type InlineLayouter interface { LayoutInline(inlineRoot *LayoutNode, maxWidth float32, atomic AtomicSizer) ([]LineBox, error) }`
- `type AtomicSizer interface { SizeInlineBlock(node *LayoutNode, maxWidth float32) (w, h float32, err error) }`

---

## 3) Pass signatures (canonical)

### BuildLayoutTree
```
func BuildLayoutTree(root *RenderNode) (*LayoutNode, error)
```
Notes:
- Returns a structurally normalized `LayoutNode` tree (E).
- Enforces B1-B4 invariants.

### ResolveUsedValues
```
func ResolveUsedValues(root *LayoutNode, ctx ResolveContext) (UsedValuesTable, error)
```
Notes:
- `ResolveContext` includes containing block metrics and policy toggles.
- No structural changes to the tree.

### FlowLayout
```
func FlowLayout(
    root *LayoutNode,
    used UsedValuesTable,
    inline interfaces.InlineLayouter,
    intrinsic interfaces.IntrinsicMeasurer,
    ctx LayoutContext,
) (LayoutResult, error)
```
Notes:
- `LayoutResult` includes geometry table and line boxes.
- No structural changes to the tree.

---

## 4) Context structs (explicit inputs)

All pass signatures must include explicit inputs to avoid hidden dependencies.

```
type ResolveContext struct {
    ContainingBlock Rect
    Policy ResolvePolicy
}

type LayoutContext struct {
    ContainingBlock Rect
    Policy LayoutPolicy
}
```

Policy structs are versioned toggles for deferred semantics (margin collapsing, line-height policy, bidi).

---

## 5) Result types

```
type UsedValues struct {
    Margin  Edges
    Padding Edges
    Border  Edges
    ContentWidth float64
    // other used values as needed
}

type LayoutGeometry struct {
    Frame   Rect
    Content Rect
}

type LayoutResult struct {
    Root *LayoutNode
    Geometry LayoutGeometryTable
    Lines LinesByBlock
}
```

---

## 6) Provisional package structure (draft)

Packages:
- `boxtree/`: BuildLayoutTree + invariants validation.
- `layout/`: used values resolution and shared layout tables/types if needed.
- `flow/`: flow layout + inline delegation + geometry validation.
- `interfaces/`: inline layouter, atomic sizer, intrinsic measurer.
- `text/`: text interfaces (strings/ropes adapters).

Allowed dependencies (acyclic):

boxtree -> layout/types
layout -> interfaces
flow -> layout, interfaces
text -> (no deps on layout/flow)

ASCII view:

boxtree  --> layout
flow     --> layout --> interfaces
flow     --> interfaces
text     (independent)

Validation functions live with their pass (boxtree/ and flow/).

---

## 7) Success criteria (Step 3 sign-off)

- Each pass has a fixed signature with explicit inputs/outputs.
- No implicit global dependencies.
- Package graph is acyclic and maps cleanly to pass boundaries.
- Signatures can accept deferred features by extending `Policy` structs only.
- External interfaces are expected to adapt to these signatures.
