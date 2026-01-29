# Core Algorithm Contract (CSS Box + Flow Layout)

This document is the single, authoritative contract for the core CSS layout pipeline in this repo. It consolidates the spec surface previously spread across `doc/Algo-Spine.md` and `doc/Functional.md`.

---

## 1) Scope and assumptions

Inputs and environment:
- A DOM annotated with computed styles (CSSDOM) already exists.
- A line-breaking module exists but is treated as a black box.
- LTR only; bidi/RTL and vertical writing modes are deferred.
- Text positions are `uint64` byte offsets; empty ranges are dropped.
- Coordinates are relative to the parent content box; root origin is (0,0).
- Adjacent text-node merging and DOM-to-text mapping are deferred.

---

## 2) Pipeline phases (passes, inputs, outputs, responsibilities)

ASCII pipeline:

D (CSSDOM) -> E (Layout Box Tree) -> Used Values -> F (Layout Result)

### Phase 1: BuildLayoutTree (D -> E)
Inputs:
- CSSDOM render tree with computed styles.

Outputs:
- Layout box tree `E` (`LayoutNode`), structurally normalized.

Responsibilities:
- Insert anonymous boxes for mixed inline/block children.
- Split + hoist when inline elements contain blocks.
- Represent inline-block as atomic inline with internal block container.
- Enforce structural invariants B1-B4 (see below).

### Phase 2: ResolveUsedValues (E -> E + used values)
Inputs:
- Layout box tree `E` and containing block context (width, etc.).

Outputs:
- Used values table (edges + used widths), keyed by BoxId.

Responsibilities:
- Resolve lengths (px/%/em/auto) into used values.
- Make padding/border/margins available before line breaking.

### Phase 3: FlowLayout (E -> F)
Inputs:
- Layout box tree `E` + used values + inline layouter + intrinsic measurer.

Outputs:
- Layout result `F` (geometry + line boxes).

Responsibilities:
- Block layout: vertical stacking, no margin collapsing yet.
- Inline layout: delegate to inline layouter when inline-only.
- Store line boxes for non-anonymous block owners.

---

## 3) Core data model (E and F)

Box kinds:
- BoxBlock
- BoxInline (structural only)
- BoxText (leaf)
- BoxAnonymousBlock
- BoxAnonymousInline (root of inline formatting context)
- BoxInlineBlock (atomic inline; contains a block formatting context internally)

LayoutNode (minimal):
- ID NodeID (DOM/render ID; split inline fragments may share the same ID)
- Box BoxKind
- Style *ComputedStyle
- Children []*LayoutNode
- Text TextRef (BoxText only)
- Geometry: Frame Rect (border box), Content Rect (content box)

TextRef:
- TextRef{ Source TextSourceID, Range: [Start,End) }
- Empty ranges are dropped at build time.

LayoutResult (F):
- Root *LayoutNode (same tree with geometry filled)
- LinesByBlock map[BoxId][]LineBox (only for non-anonymous owners, per policy)
- Used values table is a separate pass output: `UsedValuesTable[BoxId]`.

---

## 4) Structural invariants (frozen)

B1. Block-container normalization
- Every block container (BoxBlock, BoxAnonymousBlock, BoxInlineBlock) has children in exactly one shape:
  1) Block-only: all children are block-level boxes
  2) Inline-only: exactly one child of kind BoxAnonymousInline

B2. Inline formatting contexts are explicit
- Inline formatting always starts at BoxAnonymousInline.

B3. Inline subtree purity
- No block-level boxes appear under BoxAnonymousInline.
- Inline elements that contain blocks are split and hoisted.

B4. BoxText is leaf
- BoxText has no children.

---

## 5) Interfaces (conceptual signatures)

- BuildLayoutTree(renderRoot) -> *LayoutNode
- ResolveUsedValues(root, containingBlock) -> UsedValuesTable
- ComputeLayoutWithConstraints(root, inlineLayouter, intrinsicMeasurer, containingBlock) -> LayoutResult

Inline layouter (black box):
- LayoutInline(anonymousInlineRoot, maxWidth, atomicSizer) -> []LineBox
  - LineBox.Frame is relative to owning block content box.
  - LineBox.Frame.Y is already stacked by the inline layouter.

Atomic sizer:
- SizeAtomicInline(node, maxWidthRemaining) -> (borderBoxW, borderBoxH)

Intrinsic measurer:
- MaxContentWidth(node) -> contentBoxW

---

## 6) Explicit deferrals (not yet)

- Margin collapsing (margins preserved, not collapsed)
- Floats, positioning, z-index, stacking contexts
- True shrink-to-fit (beyond max-content approximation)
- Span-level line-height / fine inline metrics
- Inline fragments for span backgrounds/borders
- Bidi/RTL, vertical writing modes
- Selection/caret mapping back to DOM/text
- Adjacent text-node merging
- Caching/memoization (design for it, do not implement yet)

---

## 7) Correctness emphasis

- Box tree structure (anonymous wrappers, split+hoist, inline-block internal block formatting) is correctness-critical.
- Used edges must be resolved before line breaking.
- IDs are stable; BoxId is unique per box; split inline fragments may share NodeIDs.
