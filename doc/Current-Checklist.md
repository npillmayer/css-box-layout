# Current Checklist

Working implementation and validation checklist, reviewed against `layout/` on
2026-11-02. This is a planning artifact, not the algorithm contract.
`doc/CSS-Algo-Overview.md` is authoritative; frozen decisions are recorded in
`doc/Exec-Roadmap.md`. Unchecked tasks below remain open.

## Established requirements

- Identity: `BoxID` is unique per box, including anonymous boxes and split
  fragments; `NodeID` maps back to the source and may be shared by fragments.
  Side tables use `BoxID`. IDs remain stable across passes; dropping empty text
  must not destabilize other IDs.
- Structure: `LayoutNode` carries box kind, style, ordered children, and a text
  reference for text leaves. Style transport and anonymous style rules need work.
- Text: stable `TextSourceID`, non-empty `[Start,End)` byte ranges with `uint64`
  offsets. Empty ranges are dropped during construction. Source plumbing remains
  deferred; the current builder uses source 0 and node-local ranges.
- Used values: margins, padding, borders, and content widths live in
  `UsedValuesTable[BoxID]`, with resolved lengths in pixels. Percent values use
  0..1. Code uses `float32`; signature documentation also uses `float64`.
- Results: `LayoutResult.Root` references the tree;
  `LayoutGeometryTable[BoxID]` is authoritative. Geometry may be mirrored on
  nodes under the core contract, but input mutation conflicts with the functional
  constraints and needs reconciliation.
- Coordinates: Frame is the border box, Content is the content box, both relative
  to the parent content box; root origin is (0,0). Margins are separate.
- Empty block containers receive one empty `BoxAnonymousInline` child. This is
  already decided in the roadmap, but not implemented by normalization.
- Lines: store `LinesByBlock` entries only for inline-only `BoxBlock` and
  `BoxInlineBlock` owners. Line frames are relative to the owner's content box,
  with Y already stacked by the inline layouter; payload is opaque.
- Atomic sizing returns border-box dimensions. An inline-block participates as
  atomic inline content and establishes a block container internally.

## Prioritized implementation tasks

### 1. Preserve computed styles during construction - high

- [ ] Carry render styles into `LayoutNode.Style`, including split inline
  fragments; define anonymous-box style handling.
- Current gap: constructors in `layout/flow.go` leave Style nil, so resolution
  uses defaults even when the render tree supplies styles.
- Acceptance: build -> resolve tests preserve widths, margins, padding, and
  borders; split fragments retain their source style and anonymous boxes follow
  the documented style rules.

### 2. Repair inline-block sizing - high

- [ ] Distinguish auto width from explicit zero, apply the chosen content width
  before internal layout, and return dimensions including padding and borders.
- [ ] Keep authoritative geometry and any node mirrors consistent.
- Current gap: `atomicSizer` computes a width but lays out using the old used
  value, then overwrites only node widths with the content width.
- Acceptance: auto, fixed, and zero widths; nonzero padding/borders;
  width-sensitive line breaking; returned dimensions agree with geometry.
- Coordinate with task 3 before finalizing the sizing implementation.

### 3. Resolve inline-block descendants against their own container - high

- [ ] Use the inline-block's content width for descendant used values when its
  width is known. Define resolution when auto width is determined by sizing.
- Current gap: `childResolveContext` uses the outer containing width even for
  fixed-width inline-blocks.
- Acceptance: child widths and percentage padding use the final inline-block
  content width. Document how late width determination fits the pass boundaries
  and ensures resolved edges are available before internal line breaking.
- Preserve the current max-content approximation; true shrink-to-fit is deferred.

### 4. Enforce empty-container normalization - medium

- [ ] Return an empty `BoxAnonymousInline` wrapper for empty block containers.
- Acceptance: empty elements, empty text, and children removed by `display:none`
  produce the required structure, including inside inline-blocks. Verify empty
  inline layout produces zero content height.

### 5. Strengthen identity guarantees - medium

- [ ] Replace or safeguard arithmetic BoxID generation against collisions and
  counter overflow; document which structural edits preserve identity.
- Current gap: `layout/boxid.go` is deterministic but has no collision handling.
- Acceptance: unique IDs in nested trees and split fragments, deterministic
  rebuilds, stable IDs across passes, and unchanged IDs when empty text is
  omitted. Check anonymous identity derivation against the roadmap.

### 6. Resolve unused abstractions and incomplete behavior - medium

- [ ] Audit repository-wide references to `BoxMetrics`, `InlineIntrinsic`, and
  `StyNodeView` before removing or integrating them. Lack of internal use alone
  does not establish that an exported API can be removed safely.
- [ ] Define the role of unused `BuildOptions`, `LayoutOptions`, and
  `LayoutContext`; remove unnecessary internal parameters where appropriate.
- [ ] Implement or explicitly defer horizontal auto-margin distribution;
  `resolveUsedValues` currently discards the auto flags returned by `resolveEdges`.
- Acceptance: retained abstractions have documented purposes, removals account
  for callers, and focused tests specify auto-margin behavior. Record any new
  semantic deferral in the contract and roadmap.

### 7. Reconcile architecture documentation - after functional fixes

- [ ] Align actual signatures, adapter boundaries, numeric types, and result
  representation across the contract, signature sketches, and package README.
- [ ] Reconcile pure input/output requirements with FlowLayout's node mutation.
- [ ] Clarify B3 and validation predicates: `IsBlockLevel` currently includes
  `BoxInlineBlock`, although inline-blocks are permitted as atomic inline items.
  Distinguish outer flow participation from internal block-container behavior.
- [ ] Define which nodes require geometry entries; inline delegation currently
  does not populate geometry for ordinary inline/text nodes or the inline root.
- Acceptance: one consistent description of the implemented pipeline, with
  explicit remaining deferrals. The proposed package split remains provisional;
  keeping the passes in `layout/` is not itself a correctness defect.

## Validation checklist

These are acceptance targets, not claims that existing tests prove them.

- [ ] B1: block containers have block-only children or one anonymous inline root;
  empty containers follow the wrapper rule.
- [ ] B2: inline formatting begins at `BoxAnonymousInline`.
- [ ] B3: split and hoist block flow out of inline elements; validate atomic
  inline-block boundaries according to the clarification in task 7.
- [ ] B4: text boxes are leaves with non-empty ranges.
- [ ] Mixed inline runs retain distinct children in DOM order, including multiple
  runs separated by blocks; split fragments retain source identity and style.
- [ ] Used-values entries exist for every BoxID; geometry covers the agreed scope.
- [ ] Frame dimensions equal Content dimensions plus padding and border within
  epsilon; margins are excluded from Frame.
- [ ] Nested coordinates are relative to parent content boxes, tested with
  nonzero padding, borders, and margins. Check `layoutBlockChildrenVertical`,
  which currently adds the parent's Content offset to child positions.
- [ ] Inline-only content height equals line extent; line ownership follows the
  non-anonymous-owner rule and atomic sizes match stored geometry.
- [ ] Repeated layout preserves the agreed input-immutability and identity rules.

Margin collapsing, floats/positioning, fine inline metrics/fragments, bidi,
vertical writing modes, DOM/text mapping, adjacent text merging, and caching
remain deferred as described in the core contract.
