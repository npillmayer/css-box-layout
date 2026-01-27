# CSS Layout Engine — Current State Summary (Project Outline)

This document summarizes what we have specified so far for the **CSS box-tree construction and layout algorithm**, assuming a **DOM with computed styles** already exists. It is intended to be both a refresher and a build-oriented outline suitable for implementation assistants (e.g., Codex / Claude Code).

---

## 1. Scope and Assumptions

### Assumed complete / available
- A **DOM** annotated with **computed styles** (CSSDOM).
- A **line-breaking module** exists (eventually glyph-based), but we treat it as a **black box** for the layout core.
- **Bidi / RTL** deferred (LTR only).
- **Text positions** are `uint64` byte offsets; `len = End - Start` holds.
- Coordinates of boxes are **relative to parent content box**.
- Adjacent text-node merging postponed.
- Mapping back to DOM/text for selection not required now.

### Current goal
Implement the pipeline stages:
- **D → E**: Build a correct **layout box tree** (anonymous boxes, split+hoist).
- **E → F**: Compute block geometry and delegate inline formatting to a layouter.
- Include **margins / padding / borders** because they affect **available line width** and sizing.
- Postpone **margin collapsing**.

---

## 2. Data-Structure Stages (Conceptual)

- **D**: Render tree / CSSDOM (DOM + computed styles).
- **E**: Layout box tree (`LayoutNode`) with correct anonymous wrappers and inline-block structure.
- **F**: Final geometry (`Frame`, `Content`) + line boxes per owning block.

---

## 3. Core Layout Node Model (E + geometry fields for F)

### Box kinds
- `BoxBlock`
- `BoxInline` (purely structural for now)
- `BoxText` (leaf)
- `BoxAnonymousBlock`
- `BoxAnonymousInline` (root of inline formatting context)
- `BoxInlineBlock` (atomic inline; contains a block formatting context internally)

### LayoutNode (minimal)
- `ID NodeID`  
  - Real boxes use DOM/render IDs  
  - Anonymous boxes have `ID = 0`
  - Split inline elements may produce multiple `BoxInline` nodes with the **same ID**
- `Box BoxKind`
- `Style *ComputedStyle`
- `Children []*LayoutNode`
- `Text TextRef` (only for `BoxText`)
- Used edges (populated during layout): `Margin`, `Padding`, `Border`
- Geometry (relative): `Frame Rect` (border box), `Content Rect` (content box)

### Text addressing
- `TextRef{ Source TextSourceID, Range: [Start,End) }`
- `Start/End` are `uint64` byte offsets; empty ranges are dropped at build time.

---

## 4. Structural Invariants Produced by BuildLayoutTree

### B1. Block-container normalization
Every **block container** (`BoxBlock`, `BoxAnonymousBlock`, `BoxInlineBlock`) has children in exactly one shape:
1) **Block-only**: all children are block-level boxes  
2) **Inline-only**: exactly one child of kind `BoxAnonymousInline`

### B2. Inline formatting contexts are explicit
Inline formatting always starts at `BoxAnonymousInline`.

### B3. Inline subtree purity
No block-level boxes appear under a `BoxAnonymousInline`.  
If source contains blocks inside inline elements, they are handled by **split+hoist**.

### B4. BoxText is leaf
`BoxText` has no children.

---

## 5. D → E: Box Tree Construction Algorithm (Flow Items + Split/Hoist)

### Key technique
Builders return sequences of **FlowItems**:
- `FlowInline(node)`
- `FlowBlock(node)`

This avoids parent-pointer rewrites and makes split+hoist deterministic.

### Core functions
- `buildBlockContainer(renderNode, boxKind) -> LayoutNode`
- `buildInlineFlow(renderNode) -> []FlowItem`
- `normalizeBlockChildren(flowItems) -> []*LayoutNode`

### normalizeBlockChildren (enforces B1)
Given a block container’s collected flow items:
- Only blocks → children = blocks
- Only inlines → children = `[BoxAnonymousInline(inlineChildren)]`
- Mixed → wrap each maximal inline run into:
  - `BoxAnonymousBlock(ID=0)`
    - `BoxAnonymousInline(ID=0)` containing inline nodes
  - keep block nodes as direct children between these wrappers

### Split + hoist (inline contains block)
`buildInlineFlow(display:inline)` builds children flow.
If any `FlowBlock` occurs:
- The inline element is **split into multiple inline fragments**:
  - each maximal inline run gets wrapped into a `BoxInline` node (same ID/style)
  - block items bubble up as `FlowBlock` to the nearest block container
This achieves hoisting and preserves inline structure where possible.

### Inline-block handling in BuildLayoutTree
`display:inline-block` becomes `BoxInlineBlock` and is treated as:
- **inline-level atomic** in its parent inline context
- **block container** internally (apply block normalization rules to its children)

### ID policy
- Inserted anonymous boxes: `ID = 0`
- Split inline elements: multiple nodes may share the same `ID`

---

## 6. E → F: Layout Algorithm Skeleton (Block Layout + Inline Delegation)

### Public API (conceptual)
- `BuildLayoutTree(renderRoot) -> *LayoutNode`
- `ComputeLayoutWithConstraints(root, inlineLayouter, intrinsicMeasurer, containingBlock) -> LayoutResult`

### LayoutResult
- `Root *LayoutNode` (same tree with geometry filled)
- `LinesByBlock map[NodeID][]LineBox`  
  - stored only for `ID != 0` (anonymous blocks use ID=0)

### Coordinate convention (frozen)
- `Frame` and `Content` coordinates are **relative to parent content box**
- Root origin fixed at `(0,0)`

### Inline layouter contract (black box)
Called only for inline-only block containers (those with a single `BoxAnonymousInline` child):
- Input: `(anonymousInlineRoot, maxWidth = owner.Content.W, atomicSizer)`
- Output: `[]LineBox` where:
  - line `Frame` is relative to owning block content box
  - `Frame.Y` already stacked (inline layouter sets vertical positions)
  - `Frame.H` is used line height
  - payload is opaque (glyph fragments etc.)

Span-level `line-height` is ignored; `BoxInline` is structural only.

### Block layout recursion
For a block container `B`:
1. Resolve used edges (margins/padding/border) and used widths (see below)
2. If inline-only:
   - call inline layouter with `B.Content.W`
   - store `LinesByBlock[B.ID]` if `B.ID != 0`
   - `B.Content.H = extent(lines)` where `extent = last.Y + last.H`
3. If block-only:
   - layout children with available width = `B.Content.W`
   - stack vertically with margins (no collapsing):
     - `child.Frame.Y = y + child.Margin.Top`
     - `y += child.Margin.Top + child.Frame.H + child.Margin.Bottom`
   - `B.Content.H = y`
4. Set `B.Frame.H = B.Content.H + paddingTop+paddingBottom + borderTop+borderBottom`

---

## 7. Used-Value Resolution (Lengths → px) and Box Metrics

### We require a resolve step
Computed styles include lengths in px/%/em/auto. Font sizes are already computed.

### Shared helper (used by both normal layout and intrinsic measurement)
`ResolveUsedEdges(node, availableWidth) -> (MarginAutoFlags)`
- Resolves:
  - `Padding` (auto treated as 0 defensively)
  - `Margin` (vertical auto → 0; horizontal auto flags recorded)
  - `Border` widths are known floats (given)
- Returns `LeftAuto/RightAuto` for margin auto handling in horizontal geometry.

### Horizontal geometry computation (simplified CSS-like)
`computeHorizontalGeometry(node, availableWidth, flags)` sets:
- Used `Content.W`
- `Frame.W = Content.W + padL+padR + borderL+borderR`
- `Frame.X = marginLeft`
- `Content.X = Frame.X + borderL + padL`

Rules:
- If `width:auto`: auto margins become 0; content fills remaining width.
- If width definite: distribute remainder to auto margins (centering if both auto).

### Important outcome
Inline layouter receives `maxWidth = owner.Content.W`, which correctly excludes padding/border.

---

## 8. Inline-Block Sizing (Atomic Inline)

### High-level rule (milestone approximation)
For `inline-block` with `width:auto`:
- `usedContentW = min(remainingLineWidth, maxContentWidthApprox)`
- Reserved width on the line uses **border-box width**:
  - `reservedW = usedContentW + padL+padR + borderL+borderR`

### AtomicSizer bridge
Inline layouter calls:
- `SizeAtomicInline(node, maxWidthRemaining) -> (w, h)`

Implementation:
1) `maxContent = intrinsic.MaxContentWidth(node)` (content-box)
2) `usedContentW = min(maxWidthRemaining, maxContent)`
3) Layout inline-block’s internal contents with availableWidth = usedContentW
4) Return `(node.Frame.W, node.Frame.H)` (border-box reservation)

---

## 9. Intrinsic Measurement: MaxContentWidth (Best-Effort, Pluggable)

We need `MaxContentWidth(inlineBlock)` to approximate shrink-to-fit later.

### Key idea
Use the **inline layouter itself** as the measurement oracle:
- Ask it to lay out with an extremely large width (`HUGE`) so it does not wrap.
- Take the resulting line width as max-content width.

### Unified intrinsic algorithm
`MaxContentWidth(n)` (content-box):
- If width is definite (px/em): return it
- Else:
  - If inline-only internal structure:
    - `lines = inline.LayoutInline(anonymousInline, HUGE, intrinsicAtomicSizer)`
    - return `max(line.Frame.W)`
  - If block-only:
    - return max over children of (child max-content content width + child pad/border)
- Memoize by node pointer, not ID (IDs not unique; anonymous are 0)

### IntrinsicAtomicSizer
When intrinsic measurement encounters nested inline-blocks:
- returns border-box width based on `MaxContentWidth(child)` without invoking full internal layout.

### Shared used-edge resolution
Intrinsic measurement also calls `ResolveUsedEdges(node, HUGE)` to ensure padding/border are available and deterministic.

---

## 10. Validation (Debug Assertions)

### After BuildLayoutTree
- No block-level nodes under `BoxAnonymousInline`
- Block containers satisfy child-shape invariant (block-only or single anonymous inline)
- `BoxText` leaves only; `TextRange.End > Start`
- Anonymous boxes have `ID=0`

### After ComputeLayoutWithConstraints
- Non-negative, finite widths/heights; no NaNs
- `Frame.W/H == Content.W/H + padding + border` (epsilon)
- Block-only children are stacked monotonically (no accidental overlaps)
- Inline-only blocks: `Content.H == extent(lines)`
- `LinesByBlock` present for inline-only blocks with `ID != 0`

---

## 11. Explicit Deferrals (Not Implemented Yet)

- Vertical **margin collapsing**
- Floats, positioning, z-index, stacking contexts
- True CSS shrink-to-fit (beyond max-content approximation)
- Inline box fragments (`FragInlineBox`) for backgrounds/borders on spans
- Bidi/RTL, vertical writing modes
- Selection/caret mapping back to DOM/text
- Optimizations (text node merging, caching)

---

## 12. Implementation Checklist (Suggested Build Order)

1) Implement `FlowItem`, `buildBlockContainer`, `buildInlineFlow`, `normalizeBlockChildren`
2) Implement `ValidateLayoutTree`
3) Implement `ResolveUsedEdges` and `computeHorizontalGeometry`
4) Implement `layoutBlockContainer` + vertical stacking (no collapsing)
5) Define `InlineLayouter` interface (black box) and stub for testing
6) Implement `IntrinsicMeasurer` (memoized) using inline layouter with `HUGE`
7) Implement `AtomicSizer` for inline-block sizing (border-box reservation)
8) Implement `ValidateLayoutGeometry`
9) Run targeted tests:
   - mixed inline/block siblings → anonymous blocks inserted
   - inline contains block → split+hoist
   - nested inline-block → intrinsic recursion

---

## 13. Minimal Interfaces to Implement Now (for integration)

### Inline layouter
- `LayoutInline(anonymousInlineRoot, maxWidth, atomicSizer) -> []LineBox`

### AtomicSizer
- `SizeAtomicInline(node, maxWidthRemaining) -> (borderBoxW, borderBoxH)`

### IntrinsicMeasurer
- `MaxContentWidth(node) -> contentBoxW`

---

## 14. Notes on “Correctness vs. Approximation”

- The **box tree structure** (anonymous wrappers, split+hoist, inline-block internal block formatting) is treated as correctness-critical and specified precisely.
- Numeric sizing is correct for:
  - content width available to line breaking (padding/border included)
  - vertical stacking without collapse
- The main approximation is intrinsic sizing (max-content via huge-width layout), chosen to be:
  - implementable now
  - replaceable later with true shrink-to-fit without changing APIs.

---
