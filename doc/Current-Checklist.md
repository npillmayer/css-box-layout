# Current Checklist

Working implementation and validation checklist, reviewed against `layout/` on
2026-11-02; cross-checked with the integration assessment on 2026-11-12.
This is a planning artifact, not the algorithm contract.
`doc/CSS-Algo-Overview.md` is authoritative; frozen decisions are recorded in
`doc/Exec-Roadmap.md`. Unchecked tasks below remain open.

## Established requirements

- Identity: `BoxID` is unique per box, including anonymous boxes and split
  fragments; `NodeID` maps back to the source and may be shared by fragments.
  Side tables use `BoxID`. IDs remain stable across passes; dropping empty text
  must not destabilize other IDs.
- Structure: `LayoutNode` carries box kind, style, ordered children, and a text
  reference for text leaves. Typed style transport and anonymous style rules are
  implemented; see `doc/Tree-Type.md` for the adapter input contract.
- Text: stable `TextSourceID`, non-empty `[Start,End)` byte ranges with `uint64`
  offsets. Empty ranges are dropped during construction. Supplied source
  references are preserved; nonempty text without a reference is rejected.
  External source storage/lookup and selection/caret mapping remain separate.
- Used values: margins, padding, borders, and content widths live in
  `UsedValuesTable[BoxID]`, with resolved lengths in pixels. Percent values use
  0..1. Code and the refreshed signature documentation use `float32`; the integration
  conversion/precision policy remains to be decided in task 9.
- Results: `LayoutResult.Root` references the tree;
  `LayoutGeometryTable[BoxID]` is authoritative. Geometry may be mirrored on
  nodes under the core contract, but input mutation conflicts with the functional
  constraints and needs reconciliation.
- Coordinates: Frame is the border box, Content is the content box, both relative
  to the parent content box; root origin is (0,0). Margins are separate.
- Empty block containers receive one empty `BoxAnonymousInline` child. This is
  implemented by normalization and covered by empty-container regressions.
- Inline layout returns overall extent only. H sets consumed content height;
  paragraph lines/baselines/glyphs stay in an invocation-local typed adapter table
  keyed by group BoxID with owner BoxID, including anonymous owners. Core line
  storage and opaque payloads are removed; see task 9.
- Atomic sizing returns border-box dimensions. An inline-block participates as
  atomic inline content and establishes a block container internally.

## Execution order and integration crosswalk

Task numbers are stable references, not execution order. The recommended next
track is the bounded document experiment in
[Document-Layout-Integration.md](Document-Layout-Integration.md). The extent-only API and typed retention are agreed and core-tested; this ordering
does not declare the external adapter or broader core complete.

| Stage | Checklist tasks | Integration milestone | Gate / owner |
|---|---|---|---|
| Preserve document input and normalized groups | 1, 8, 4, 5 | M1 | Core fixes and tests; tytui supplies the adapter fixture |
| Implement paragraph boundary | 9 and relevant parts of 7 | M2 | Core complete; real conversion/publication remains adapter work |
| Implement accepted boundary and prove placement | 9, 10 | M2-M3 | Core tests with fake paragraphs; explicit widths and full extents |
| Prove resize and snapshots | 11 | M4 | Core immutability plus tytui publication/retention checks |
| Connect paragraph implementations and proof | Accepted boundary, 7 follow-through | M5-M6 | tytui integration; core fixes remain upstream |
| Extend beyond the bounded experiment | 2, 3; remaining 6 and 7 | Later track | Inline-blocks/intrinsics and broader cleanup are not first-experiment prerequisites |

Task 9 core changes and error checks are implemented; real adapter conversion
and publication remain. M3-M4 now use the accepted extent-only contract;
general dead-code cleanup and auto-margin implementation need not block a fixture
using definite widths and explicit non-auto margins.

## Implementation tasks

### 1. Preserve styles and source references - immediate / M1

- [x] Carry typed render styles into `LayoutNode.Style`, including split inline
  fragments; define anonymous-box style handling.
- [x] Accept and preserve adapter-supplied text-source IDs/ranges; define the
  supported typed-value subset and block-root input convention. NodeID and TextRef
  provide explicit keys into adapter-owned source/style storage.
- [ ] Wire external source/style lookup in tytui; this integration work is not
  a new CSS parsing or text-storage subsystem in the core.
- Implemented: RenderNode.Style/Text are copied; source font-size inheritance
  survives hoisting; anonymous wrappers inherit font size but not edges/width.
  Own-font em resolution is corrected. Known string box values, invalid typed
  lengths, missing nonempty text references, and reversed ranges report errors.
- Verified: group widths 240/120/240px, preserved source ranges, split styles,
  anonymous defaults, input snapshot isolation, and error cases. Full tests and
  vet pass. General CSSDOM adapters and actual source storage stay external.
- Acceptance: build -> resolve tests preserve widths, margins, padding, and
  borders; split fragments retain their source style and anonymous boxes follow
  the documented style rules. Distinct text nodes remain identifiable through
  normalization; the shared sample resolves group widths 240/120/240px. No
  selection/caret subsystem or general CSSDOM adapter is required to prove this.

### 2. Repair inline-block sizing - later atomic-inline track

- [ ] Distinguish auto width from explicit zero, apply the chosen content width
  before internal layout, and return dimensions including padding and borders.
- [ ] Keep authoritative geometry and any node mirrors consistent.
- Current gap: `atomicSizer` computes a width but lays out using the old used
  value, then overwrites only node widths with the content width.
- Acceptance: auto, fixed, and zero widths; nonzero padding/borders;
  width-sensitive line breaking; returned dimensions agree with geometry.
- Coordinate with task 3 before finalizing the sizing implementation.

### 3. Resolve inline-block descendant widths - later atomic-inline track

- [ ] Use the inline-block's content width for descendant used values when its
  width is known. Define resolution when auto width is determined by sizing.
- Current gap: `childResolveContext` uses the outer containing width even for
  fixed-width inline-blocks.
- Acceptance: child widths and percentage padding use the final inline-block
  content width. Document how late width determination fits the pass boundaries
  and ensures resolved edges are available before internal line breaking.
- Preserve the current max-content approximation; true shrink-to-fit is deferred.

### 4. Enforce empty-container normalization - immediate / M1

- [x] Return an empty `BoxAnonymousInline` wrapper for empty block containers.
- Verified: block and inline-block containers with no children, empty text, or
  hidden children retain an empty wrapper with deterministic identity. A built
  empty root delegates inline layout at its content width and gets zero height
  from an empty result. `go test ./...` passes.
- Acceptance: empty elements, empty text, and children removed by `display:none`
  produce the required structure, including inside inline-blocks. Verify that
  an empty fake result produces zero height under the current contract. Task 9 now supports an explicit nonzero reserved extent for empty paragraphs.

### 5. Strengthen identity guarantees - immediate / M1

- [x] Safeguard arithmetic BoxID generation against collisions and counter
  wraparound; document which structural edits preserve identity.
- Implemented: a build-local allocated-ID set and deterministic linear probing
  protect parent/order-derived candidates; zero is reserved and allocation
  ordinals use uint64. Probe and ordinal wraparound cannot reuse an allocated ID.
- Verified: concrete tree collision, old uint32 boundary, uint64 wraparound,
  nested/split/anonymous boxes, identical rebuilds, omitted empty text, and
  pass-to-pass stability. `go test ./...` and `go vet ./...` pass.
- Scope: uniqueness is per build. Equal allocation sequences produce equal IDs;
  arbitrary insertions, removals, reordering, or display changes may change IDs.
  NodeID remains the source association; BoxID alone is not a cross-document key.
- Acceptance: unique IDs in nested trees and split fragments, deterministic
  rebuilds, stable IDs across passes, and unchanged IDs when empty text is
  omitted. Check anonymous identity derivation against the roadmap.

### 6. Resolve unused abstractions and incomplete behavior - scoped cleanup

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

### 7. Reconcile architecture documentation - alongside affected decisions

- [ ] Align actual signatures, adapter boundaries, numeric types, and result
  representation across the contract, signature sketches, and package README.
- [ ] Document the ownership solution implemented under task 11; do not treat
  node mutation as a documentation-only issue.
- B3 wording is now clarified in the core contract. Remaining work:
- [ ] Align validation predicates: `IsBlockLevel` currently includes
  `BoxInlineBlock`, although inline-blocks are permitted as atomic inline items.
  Distinguish outer flow participation from internal block-container behavior.
- [ ] Define which nodes require geometry entries; inline delegation currently
  does not populate geometry for ordinary inline/text nodes or the inline root.
- Acceptance: one consistent description of the implemented pipeline, with
  explicit remaining deferrals. The proposed package split remains provisional;
  keeping the passes in `layout/` is not itself a correctness defect.

### 8. Preserve distinct mixed inline runs - immediate / M1

- [x] Fix retained child slices sharing storage with the reused inline-run buffer.
- [x] Add a regression for text / block / text and unequal-length multiple runs.
- Evidence: the integration probe produced source IDs 4/4 instead of 2/4.
- Completed: each retained run owns a copied child slice. Four table-driven
  BuildLayoutTree cases failed before the fix and pass after it; `go test ./...`
  passes. Style transport remains task 1.
- Acceptance: all runs retain their own ordered children and source/style data;
  no later construction changes an earlier group.

### 9. Paragraph collaboration boundary - core complete / M2

- [x] Return only finite nonnegative overall extent, including trailing space
  and optional empty-group height; distinguish layout height from ink bounds.
- [x] Select typed invocation-local adapter retention by Group.BoxID with OwnerID,
  including anonymous groups. Remove core line tables and opaque payloads.
- [x] Provide definite CSS-pixel ContentWidth and read-only used-values access;
  source/style lookup remains explicit adapter state; baselines stay in P.
- [x] Validate missing/invalid used values before callbacks, invalid extents,
  and wrapped adapter failures. Return nil LayoutResult on failure.
- [x] Update core contracts/signatures and prove behavior with typed fake adapters.
- [ ] Implement real unit conversion/rounding and snapshot publication in tytui;
  full node-geometry immutability still belongs to task 11.
- Acceptance: full extent flows without line summaries; typed results stay outside
  core. Fake tests pass for all three groups, trailing space, zero-width empty
  groups, and errors. The actual paragraph engine is not wired yet.

### 10. Correct outer coordinates and prove fake-paragraph placement - M3

- [x] Fix child positions to be relative to parent content, without adding the
  parent's own Content offset again.
- [x] Test widths 240/120/240px and supplied heights 24/36/12px: origins
  (0,0), (0,24), (0,60), root content height 72px.
- [x] Add three-level nesting with padding, borders, and additive margins.
- Verified 2026-11-13: exact local Frame/Content rectangles, signed horizontal
  margins, sibling order, available widths, and once-only document transforms.
  Changing root edges leaves descendant local geometry unchanged. Regression
  failed before the fix; full tests and vet pass.
- Acceptance: independently specified positions match; paragraph-local lines
  are not restacked and parent transforms are applied exactly once.

### 11. Preserve snapshots through resize and errors - M4

- [ ] Remove input-node geometry mutation or implement agreed copy/ownership
  semantics; keep geometry, paragraph references, and paint snapshots consistent.
- [ ] Recompute used values for a 120px -> 90px width change; use a fake whose
  height changes, and verify following siblings move by that height difference.
- [ ] Verify a failed run leaves the previous successful result usable.
- Acceptance: old table and Root views do not disagree after a second layout;
  retained paragraph data stays valid. Core owns layout purity; tytui owns atomic
  publication of its complete document result. No caching is required.

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
- [x] Nested coordinates are relative to parent content boxes, tested with
  nonzero padding, borders, and margins. The stacking helper no longer adds
  parent Content offsets to child positions.
- [x] Validate explicit extent and adapter-owned typed associations, including
  anonymous groups, trailing space, and empty results (core fake-adapter tests).
- [ ] Atomic sizes match stored geometry when the later atomic-inline track runs.
- [ ] Repeated layout preserves the agreed input-immutability and identity rules.

Margin collapsing, floats/positioning, fine inline metrics/fragments, bidi,
vertical writing modes, selection/caret DOM mapping, adjacent text merging, and caching
remain deferred as described in the core contract.
