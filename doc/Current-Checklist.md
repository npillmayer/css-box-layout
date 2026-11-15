# Current Checklist

Working implementation and validation checklist, reviewed against `layout/` on
2026-11-02; core M4 verified 2026-11-13; bounded cross-module integration
verified through stage 5 on 2026-11-15. See
[the evidence crosswalk](Document-Layout-Integration.md#7-bounded-integration-evidence).
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
  conversion policy is settled: independently round CSS primitives to DU using
  round(px * 49152), check signed sums, retain paragraph float64 big points and
  preserve CSS-owned border dimensions. Task 9 has integration rounding tests.
- Results: `LayoutResult.Root` shares the immutable structural tree;
  `LayoutGeometryTable[BoxID]` is fresh per invocation and exclusively owns
  derived geometry. No node mirrors remain. Published trees, styles, and result
  tables are read-only by ownership contract, not enforced by Go.
- Coordinates: Frame is the border box, Content is the content box, both relative
  to the parent content box; root origin is (0,0). Margins are separate.
- Empty block containers receive one empty `BoxAnonymousInline` child. This is
  implemented by normalization and covered by empty-container regressions.
- Inline layout returns extent plus placements of core-owned atomic boxes, not
  lines. H sets consumed content height;
  paragraph lines/baselines/glyphs stay in an invocation-local typed adapter table
  keyed by group BoxID with owner BoxID, including anonymous owners. Core line
  storage and opaque payloads are removed; see task 9.
- Atomic sizing returns border-box dimensions. An inline-block participates as
  atomic inline content and establishes a block container internally.

## Execution order and integration crosswalk

Task numbers are stable references, not execution order. The bounded document
experiment in [Document-Layout-Integration.md](Document-Layout-Integration.md) is
complete, including the real adapter. The crosswalk distinguishes that evidence
from broader unchecked core tasks; it does not declare the entire core complete.

| Stage | Checklist tasks | Integration milestone | Gate / owner |
|---|---|---|---|
| Preserve document input and normalized groups | 1, 8, 4, 5 | M1 | Core fixes and tests; tytui supplies the adapter fixture |
| Implement paragraph boundary | 9 and relevant parts of 7 | M2 | Core and actual tytui conversion/publication verified |
| Implement accepted boundary and prove placement | 9, 10 | M2-M3 | Core fake controls plus independent tytui geometry oracles |
| Prove resize and snapshots | 11 | M4 | Core immutability plus tytui publication/retention checks |
| Connect paragraph implementations and proof | Accepted boundary, 7 follow-through | M5-M6 | Synthetic and real-font tytui integration verified; fixes stay upstream |
| Fixed-width recursive inline-block extension | 2A, 3A, relevant 7 | Separate core/adapter track | Core controls plus real fixed-width document proof verified |
| Bounded auto sizing | 2B, 3B | Implemented and integrated | Actual nested measurement, geometry, lifetime and retained proofs verified |
| Broader cleanup/policies | Remaining 6 and 7 | Deferred/open | No speculative-call or general CSS completion claim |

Task 9 core changes, real adapter conversion and publication are verified.
M3-M4 establish extent-based flow; task 2A adds atomic placements;
general dead-code cleanup did not block the original fixture with explicit
non-auto margins. LTR horizontal block margin distribution is now implemented
and covered by separate core and tytui regressions.

## Implementation tasks

### 1. Preserve styles and source references - immediate / M1

- [x] Carry typed render styles into `LayoutNode.Style`, including split inline
  fragments; define anonymous-box style handling.
- [x] Accept and preserve adapter-supplied text-source IDs/ranges; define the
  supported typed-value subset and block-root input convention. NodeID and TextRef
  provide explicit keys into adapter-owned source/style storage.
- [x] Wire external source/style lookup in tytui; this integration work is not
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

### 2. Recursive inline-block sizing and placement - fixed-width first

Core-owned preparation and the task-2A API are implemented and integrated with
tytui/typography. Bounded auto/intrinsic sizing is implemented under task 2B.

#### 2A. Fixed-width implementation sequence (with 3A) - core complete

1. [x] Implement PreparedAtomicsView.Lookup(BoxID), one view per group, and
   InlineResult{Extent, Atomics}; remove external AtomicSizer. Adapter-local whatsit
   handles map back to existing BoxID. Paragraph payloads remain adapter-owned.
2. [x] Test two nested atomic boundaries beneath ordinary inline spans, including
   internal block stacks, source separation, inner-before-outer calls, and no
   external reentrancy. Separate IsBlockLevel from IsBlockContainer.
3. [x] Recursively prepare fixed-pixel atomics, including zero, before delegation;
   use correct descendant widths/edges. Reject unsupported percent/em atomic
   widths explicitly before callbacks (auto is added by 2B); no intrinsic calls or line-width clamping.
4. [x] Apply validated placements using RelativeTo, preserving descendant-local
   rectangles and adding no second margin. Test local whatsit handles, lookup
   copies, missing/duplicate/foreign/unknown IDs, non-finite coordinates, valid
   negative positions, and zero-sized items. Existing extent tests use a test-only
   wrapper; production adapters must migrate to InlineResult.
5. [x] Verify composed origins, margin/border/padding accounting, sibling movement,
   fixed-width overflow on resize, inner failure suppressing outer delegation,
   unchanged inputs/old snapshots, and deterministic recovery. Full tests, vet,
   and race checks pass.

The fake P breaks between input items, never within whatsits; an oversized item
on an empty line overflows. Bottom margin edges align to the line baseline. Real
P encoding, source mapping, font layout and publication have separate tytui tests
and fixed/auto real-font proofs; the fake tests alone do not establish those facts.

#### 2B. Auto width through intrinsic bounds - core implemented

- [x] Implement MeasureInline, IntrinsicWidths, BoxID-keyed IntrinsicAtomicsView,
  and edges-only ResolvedEdgesView. Remove MaxContentWidth and InlineIntrinsic.
- [x] Measure nested atomic boundaries before outer groups; combine block child
  contributions by maximum. Fixed atomics have equal border bounds; auto bounds
  derive from content. Add borders/padding and margins once at their own boundary.
- [x] Select min(max(minContent, availableContent), maxContent) against containing
  block content width minus own horizontal edges/margins, clamped to zero.
- [x] Validate missing services, unresolved edges/percentage dependencies, invalid
  bounds, zero/empty content, min-content overflow, mandatory-break fake metrics,
  nested fixed/auto items and definite percentage bases.
- [x] Keep measurement separate from final layout and publication. Tests cover
  ordered measurement/final calls, view copies, errors before final delegation,
  snapshot preservation, resize, deterministic recovery, and concurrent read-only
  inputs. Full tests, vet and race checks pass.

Real typography measurement stays external to CSS core and is now connected in
tytui. Fake measurers define explicit metrics; separate scheduling/geometry and
real-font tests establish integration (pipeline stages 5A–5D).
No speculative layouts or cross-invocation cache is introduced. Keep the existing
bounded alignment policy; full baseline behavior remains separate work.

### 3. Resolve inline-block descendant widths - paired with task 2

#### 3A. Fixed-width prerequisite - implemented

- [x] Resolve descendants against each inline-block's content width, preserving
  that basis through ordinary inline wrappers.
- [x] Verify nested fixed widths, percentage block widths/edges, and explicit-zero
  containing width before inner paragraph calls, without mutating inputs.

#### 3B. Late width determination - core implemented

- [x] Add WidthPending/EdgesPending state to UsedValues; unresolved auto width and
  dependent percentages cannot masquerade as resolved zero.
- [x] Expose only independently resolved BoxEdges during measurement. Reject
  percentage edges or contributing widths needing an unknown auto basis. Do not
  solve cyclic percentages or silently evaluate them against zero.
- [x] Copy used values per invocation, choose auto width, and re-resolve descendants
  against the chosen width. All pending resolution completes before any final
  LayoutInline call; trees, caller tables and published snapshots remain unchanged.
- [x] Test nested auto/fixed boundaries, definite percentage bases, border/margin
  accounting, min-content overflow, zero width, resize and error recovery.
- Acceptance met for the bounded subset: measure bounds -> select width -> resolve
  descendants -> one final layout. Full CSS intrinsic/cyclic-percentage semantics,
  speculative acceptance and global caching remain deferred.

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

- [x] Remove the unused `glyphing` package and its superseded line/glyph result
  sketches. No imports were found in this repository or the local Go repositories
  searched; unknown external consumers may need migration. Concrete paragraph
  results remain adapter-owned; no replacement core package is introduced.

- [ ] Audit repository-wide references to `BoxMetrics` and `StyNodeView` before removing or integrating them. Lack of internal use alone
  does not establish that an exported API can be removed safely.
- [ ] Define the role of unused `BuildOptions`, `LayoutOptions`, and
  `LayoutContext`; remove unnecessary internal parameters where appropriate.
- [x] Implement horizontal auto-margin distribution for LTR normal-flow blocks.
  One/two auto margins absorb positive free space after widths and edges;
  auto-width margins stay zero, and overflow/all-fixed constraints resolve the
  right margin. Inline-block auto margins remain zero. Descendant block margins
  stay pending until the containing auto width is chosen, then resolve privately.
  `layout/auto_margins_test.go` covers units, signed margins, zero/overflow,
  fractional rounding, intrinsic sizing, callback values and snapshot lifetime.
  `tytui/documentlayout/auto_margin_test.go` exercises the same boundary through
  real paragraph layout under both breakers. Existing used-value tests now assert
  distributed margins and LTR right-margin correction rather than the old zeros.
- Acceptance: retained abstractions have documented purposes, removals account
  for callers, and focused tests specify auto-margin behavior. Record any new
  semantic deferral in the contract and roadmap.

### 7. Reconcile architecture documentation - alongside affected decisions

- [ ] Align actual signatures, adapter boundaries, numeric types, and result
  representation across the contract, signature sketches, and package README.
- [x] Document task 11 ownership: structural inputs remain read-only; fresh
  result tables exclusively own geometry, with no node mirrors.
- B3 wording is now clarified in the core contract. Remaining work:
- [x] Separate IsBlockLevel (outer block flow) from IsBlockContainer (including
  atomic inline-block interiors) in resolution and container dispatch.
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
- [x] Implement real unit conversion/rounding and snapshot publication in tytui;
  core input immutability is implemented and tested under task 11.
- Acceptance: full extent flows without line summaries; typed results stay outside
  core. Fake tests pass for all three groups, trailing space, zero-width empty
  groups, and errors. The actual paragraph engine is wired and verified by
`geometry_test.go`, `nested_geometry_test.go` and `lifetime_test.go` in tytui.

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

### 11. Preserve snapshots through resize and errors - core complete / M4

- [x] Remove input-node geometry mutation, including atomic-sizing callbacks.
  Geometry lives only in fresh result tables; results share the structural root.
- [x] Recompute used values for a 120px -> 90px width change; use a fake whose
  height changes, and verify following siblings move by that height difference.
- [x] Verify a failed run leaves the previous successful result usable, including
  invocation-local typed paragraph retention and paint in a fake document adapter.
- [x] Exercise complete document publication with the actual tytui adapter
  through both breakers, nested resize/failure recovery and retained proofs.
- Verified 2026-11-13: root constraints 240px -> 180px change G2 width 120px ->
  90px, H2 36px -> 48px, G3 Y 60px -> 72px, and root height 72px -> 84px.
  Failure after an earlier successful callback preserves old results; a later
  successful rerun is deterministic. Shared read-only inputs support concurrent
  flows. Full tests, vet, and race checks pass.
- Ownership: caller and callbacks must not mutate shared trees/styles or published
  tables. New style/structure requires a new tree; resized constraints require
  fresh used values, not a new tree. No caching or defensive deep copy is added.
- Acceptance: old geometry and Root remain consistent; retained paragraph data
  stays valid. Core owns layout purity; tytui owns atomic publication of its
  complete document result. Separate tytui stage 5C/5D tests prove real wiring;
  shared resolvers/prepared owners still require serialized operations.

## Validation checklist

Checked entries have owning/integration evidence; unchecked broad invariant
targets remain open. Bounded pipeline completion does not check them implicitly.

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
- [x] Atomic sizes match stored geometry in core recursive tests and actual
  tytui geometry tests, preserving CSS dimensions and precise paragraph origins.
- [x] Repeated layout preserves input immutability and identity, including
  resize, failure, atomic callbacks, and concurrent read-only runs (core tests).

Margin collapsing, floats/positioning, fine inline metrics/fragments, outer RTL,
vertical writing modes, selection/caret DOM mapping, adjacent text merging, and global caching
remain deferred as described in the core contract.


Stage 5E validation (2026-11-15): ordinary/race suites, build and vet passed across
khipu, typography, tytui and css-box-layout. Dependency-boundary audits, backend
service regressions and fixed/auto proof plus replay controls passed unchanged.
Invocation-local paragraph preparation reuse is implemented; cross-invocation
caches, speculative callbacks and broader CSS policies remain deferred. Glyph
coverage is caller-owned; existing itemizer-check relocation remains future work.
