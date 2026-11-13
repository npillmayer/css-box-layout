# Current Checklist

Working implementation and validation checklist, reviewed against `layout/` on
2026-11-02; integration milestones updated through core M4 on 2026-11-13.
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
- Results: `LayoutResult.Root` shares the immutable structural tree;
  `LayoutGeometryTable[BoxID]` is fresh per invocation and exclusively owns
  derived geometry. No node mirrors remain. Published trees, styles, and result
  tables are read-only by ownership contract, not enforced by Go.
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
| Fixed-width recursive inline-block extension | 2A, 3A, relevant 7 | Separate core/adapter track | Contract and fake-adapter proof first; not a prerequisite for M5-M6 |
| Auto sizing and broader cleanup | 2B, 3B; remaining 6 and 7 | Follow-up | Intrinsics/speculation follow fixed-width acceptance |

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

### 2. Recursive inline-block sizing and placement - fixed-width first

Agreed direction: retain synchronous core -> paragraph -> AtomicSizer -> core
recursion. Core owns internal geometry; the paragraph adapter owns atomic line
placement, keyed by the existing BoxID. Construction and callback recursion exist;
correct recursive sizing/placement is not yet established. See integration
section 4.1 for the collaboration contract. All steps below are still open.

#### 2A. Fixed-width implementation sequence (with 3A)

1. [ ] Specify placement ownership concretely before code changes: typed adapter
   placement record keyed by atomic BoxID, containing-group association, coordinate
   origin, and once-only composition with core geometry. Decide how final placement
   is exposed without treating the current zero-origin entry as a final position.
   Define the first fixture's vertical alignment and margin policy explicitly;
   general baseline alignment remains separate. Preserve extent-only returns.
2. [ ] Add build -> resolve -> flow regression fixtures with ordinary inline
   spans surrounding at least two nested inline-block boundaries. Preserve IDs,
   source/style references, and block children inside an atomic boundary; do not
   hoist them into the outer paragraph. Separate outer-flow classification from
   block-container capability where validation needs it (task 7).
3. [ ] Implement fixed CSS-pixel content widths, including explicit zero, without
   calling intrinsic measurement or clamping to remaining line width. Distinguish
   auto from zero and define explicit rejection of unsupported auto sizing in the
   bounded fixture. Complete 3A before inner paragraph callbacks. Return finite
   border-box W/H including padding/borders; margins remain separate.
4. [ ] Exercise a reentrant fake paragraph adapter: stop outer traversal at each
   atomic item, call AtomicSizer, retain inner groups separately, then resume the
   outer call and retain its final placement by BoxID. Use one accepted sizing
   call per atomic box per invocation; reject unsupported repeated trials.
5. [ ] Verify final nested document origins, outer line fitting/overflow and
   following-sibling heights, nonzero edges, explicit-zero/empty contents, and
   all inner/outer typed associations. Inject an inner failure after earlier
   success; discard the whole pending document while preserving previous results
   and immutable inputs. Re-run at new outer widths; fixed atomic widths stay
   fixed. Run full tests, vet, and race checks.

Acceptance: independently specified widths, border-box sizes, composed positions,
and call nesting agree across at least two atomic boundaries. Fake-adapter proof
must not be described as real tytui support; actual adapter wiring has its own
integration gate. First step is the placement/alignment contract, not auto sizing.

#### 2B. Follow-up after fixed-width acceptance

- [ ] Apply chosen auto width before internal layout; add checked intrinsic
  dependencies and width-sensitive measurements. Coordinate with 3B.
- [ ] Design acceptance/discard for speculative widths and repeated group calls,
  covering geometry, typed paragraph results, and placement together.
- [ ] Extend atomic alignment to the required baseline/margin behavior without
  restoring core paragraph line summaries.
- Current sizing gaps: auto width is not propagated into internal layout, and
  returned width currently omits border/padding. M4 fixed mutation, not sizing.
- True shrink-to-fit remains deferred; the current max-content approximation is
  not a claim of CSS-complete auto sizing.

### 3. Resolve inline-block descendant widths - paired with task 2

#### 3A. Fixed-width prerequisite

- [ ] Use each fixed inline-block's content width as its descendants' containing
  width, recursively. Resolve child percentage widths and percentage edges before
  invoking inner LayoutInline; ordinary inline wrappers must preserve this basis.
- [ ] Test multiple nested fixed widths with percentage-sized block descendants,
  nonzero edges, and explicit-zero atomic content width. Available callback width
  must be the owner's content width, not its border-box or outer paragraph width.
- Current gap: `childResolveContext` still uses the outer containing width even
  for fixed-width inline-blocks. Repair without mutating input trees/tables.

#### 3B. Late width determination (after 2A/3A)

- [ ] Define invocation-local descendant resolution after auto width is selected,
  including the used-values view seen by nested callbacks. Never mutate caller
  tables or publish provisional geometry/retained results as accepted output.
- Acceptance: every internal line-breaking decision sees edges and descendant
  widths resolved against the selected content width; pass dependencies remain
  explicit. Caching and true shrink-to-fit are not part of this first extension.

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
- [x] Document task 11 ownership: structural inputs remain read-only; fresh
  result tables exclusively own geometry, with no node mirrors.
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
  core input immutability is implemented and tested under task 11.
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

### 11. Preserve snapshots through resize and errors - core complete / M4

- [x] Remove input-node geometry mutation, including atomic-sizing callbacks.
  Geometry lives only in fresh result tables; results share the structural root.
- [x] Recompute used values for a 120px -> 90px width change; use a fake whose
  height changes, and verify following siblings move by that height difference.
- [x] Verify a failed run leaves the previous successful result usable, including
  invocation-local typed paragraph retention and paint in a fake document adapter.
- [ ] Exercise complete document publication with the actual tytui adapter.
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
  complete document result. The fake fixture does not prove real tytui wiring.

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
- [x] Repeated layout preserves input immutability and identity, including
  resize, failure, atomic callbacks, and concurrent read-only runs (core tests).

Margin collapsing, floats/positioning, fine inline metrics/fragments, bidi,
vertical writing modes, selection/caret DOM mapping, adjacent text merging, and caching
remain deferred as described in the core contract.
