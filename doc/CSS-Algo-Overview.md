# Core Algorithm Contract (CSS Box + Flow Layout)

This document is the single, authoritative contract for the core CSS layout pipeline in this repo. It consolidates the spec surface previously spread across `doc/Algo-Spine.md` and `doc/Functional.md`.

Implementation status and open work: `doc/Exec-Roadmap.md` and
`doc/Current-Checklist.md`. Requirements here are not claims of completed code.

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
- Used values table (edges + used widths), keyed by BoxID.

Responsibilities:
- Resolve lengths (px/%/em/auto) into used values.
- Make padding/border/margins available before the line-breaking decisions that use them.
- Auto inline-block width is deferred to intrinsic sizing. How descendant used
  values are resolved against that final width remains an open pass-boundary
  design question (checklist tasks 2 and 3); do not treat provisional widths as final.
Notes:
- Percent lengths are represented as 0..1 in `Length.Value` (e.g., 0.10 = 10%).

### Phase 3: FlowLayout (E -> F)
Inputs:
- Layout box tree `E` + used values table + inline layouter + intrinsic measurer.

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
- BoxID BoxID (unique box identity, including anonymous boxes and fragments)
- NodeID NodeID (source identity; split inline fragments may share it)
- Box BoxKind
- Style *ComputedStyle
- Children []*LayoutNode
- Text TextRef (BoxText only)
- Optional geometry mirrors: Frame Rect (border box), Content Rect (content box);
  ownership must satisfy the immutable-input requirement.

TextRef:
- TextRef{ Source TextSourceID, Range: [Start,End) }
- Empty ranges are dropped at build time.

LayoutResult (F):
- Root *LayoutNode (structural tree reference; geometry table is authoritative)
- Geometry table `LayoutGeometryTable[BoxID]` (authoritative)
- LinesByBlock map[BoxID][]LineBox (stored for BoxBlock and BoxInlineBlock owners only)
- Used values table is a separate pass output: `UsedValuesTable[BoxID]`.

---

## 4) Structural invariants (frozen)

B1. Block-container normalization
- Every block container (BoxBlock, BoxAnonymousBlock, BoxInlineBlock) has children in exactly one shape:
  1) Block-only: all children are block-level boxes
  2) Inline-only: exactly one child of kind BoxAnonymousInline
- Empty containers use the inline-only shape with an empty BoxAnonymousInline.
- Block-container capability is distinct from outer block-level participation:
  BoxInlineBlock is atomic inline, even though it is a block container internally.

B2. Inline formatting contexts are explicit
- Inline formatting always starts at BoxAnonymousInline.

B3. Inline subtree purity
- No block-level flow participants appear within an inline formatting context.
- An inline-block is an atomic boundary: its internal block descendants belong
  to its own context. Validation does not descend through that boundary as part
  of the outer inline-purity check.
- Inline elements that contain blocks are split and hoisted.

B4. BoxText is leaf
- BoxText has no children.

---

## 5) Interfaces (conceptual signatures)

- BuildLayoutTree(renderRoot) -> *LayoutNode
- ResolveUsedValues(root, containingBlock) -> UsedValuesTable
- FlowLayout(root, usedValues, inlineLayouter, intrinsicMeasurer, containingBlock) -> LayoutResult

Inline layouter (black box):
- LayoutInline(anonymousInlineRoot, maxWidth, atomicSizer) -> []LineBox
  - LineBox.Frame is relative to owning block content box.
  - LineBox.Frame.Y is already stacked by the inline layouter.

Atomic sizer:
- SizeInlineBlock(node, maxWidthRemaining) -> (borderBoxW, borderBoxH)

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
- IDs are stable; BoxID is unique per box; split inline fragments may share NodeIDs.

## 8) Open representation decisions

- Functional constraints require immutable pass inputs. Current code mutates
  node geometry; optional geometry mirroring needs an ownership/copying design
  compatible with that requirement. Mirroring is not permission to mutate inputs.
- Define geometry coverage for structural inline nodes, text, and anonymous
  inline roots; a single rectangle may not represent fragmented inline content.
- Auto-width state and horizontal auto-margin distribution need explicit
  semantics. These are tracked work, not newly frozen deferrals.
- The current typed adapter subset and anonymous styles are specified in
  `doc/Tree-Type.md`: anonymous wrappers inherit font size, have auto width and
  zero edges; real boxes preserve copied styles and supplied text references.
