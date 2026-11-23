# Execution Roadmap

Implementation update: tasks 2A/3A and bounded auto-width tasks 2B/3B are
implemented. Core measures nested intrinsic bounds, selects auto content widths,
resolves descendants privately, then performs final layout. Fixed pixel widths
including zero remain supported; percent/em widths on atomic boxes remain rejected.
Percentage dependencies on unknown auto widths fail explicitly; definite bases
remain supported. See [current API](Pass-Signatures.md#intrinsic-measurement).
Diagrams illustrate the fixed-width final-layout phase, not intrinsic measurement.
Bounded tytui/typography integration is verified through pipeline stage 5
(2026-11-15): normalized input, intrinsic/final callbacks, nested geometry,
transactional result lifetime and retained real-font proofs. See
[the integration evidence](Document-Layout-Integration.md#7-bounded-integration-evidence).
Full CSS baseline behavior, speculative trials and broader core work remain open.

BOX-CONT-01 (2026-11-23) is implemented: construction failures expose
`layout.BuildError` with input RootID, offending NodeID when HasNode is true,
and preserved error causes. Tree-wide failures identify the input boundary.
Independent filtered-projection tests pin source/text/style preservation,
normalization after filtering, input immutability and build-local BoxID scope.
This adds no routing/forest API and changes no geometry pass. See
[construction diagnostics](../layout/README.md#construction-diagnostics-and-independent-trees).

BOX-CONT-02 (2026-11-23) is implemented: `registers.Register[T any]` and
`RegisterBank[T any]` replace the provisional FlowMaterial interface, with no
GetBoxID requirement or layout dependency. Existing queue/map algorithms retain
persistent versions, FIFO/front insertion, structural sharing and zero-value
usability. Empty reads return zero T/false; stored zero/nil items return true.
Referenced payload data must remain immutable. Tests cover concrete and
noncomparable payloads, independent/concurrent derivation, exhausted-name removal
and drained-storage release. Consumer-specific Box validity remains pagelayout's
responsibility; no routing or preparation behavior is added to storage.

BOX-PREP-01 (2026-11-23) is implemented: `layout.InlineGroups` returns a repeatable
`iter.Seq[layout.InlineGroup]` in normalized structural preorder. Descriptors
identify the actual group/owner, immediate atomics and nearest parent atomic;
empty groups and nested interiors remain separate. Nodes are borrowed read-only;
each descriptor owns its Atomics slice. Nil/subtree inputs and early termination
have explicit semantics. Pagelayout's content index now consumes this traversal
while retaining source-context validation and qualified immutable Box handles.
Tests cover split/anonymous owners, replaced leaves, empty groups, nested atomics,
tree-local ID collisions, slice independence and concurrent enumeration without
input changes. No typography, font or DOM dependency is added by enumeration,
and no preparation engine or geometry pass is implemented. See
[group enumeration](../layout/README.md#enumerating-normalized-groups).


High-level milestones for the core layout pipeline, reviewed on 2026-11-02;
integration sequencing aligned through bounded pipeline stage 5 on 2026-11-15.
`doc/CSS-Algo-Overview.md` is the authoritative algorithm contract.
`doc/Current-Checklist.md` holds prioritized tasks and detailed acceptance criteria.
Implementation status below does not imply that all invariants are validated.

## 1) Consolidate the spec surface

Status: the core contract exists, supported by `doc/Algo-Spine.md` (sequencing)
and `doc/Functional.md` (purity and memoization readiness).

B3 boundary wording is clarified in the core contract. Remaining: align
validation predicates and geometry coverage. Track these under checklist task 7;
settle any ambiguity needed by a fix before implementing it. M4 establishes
table-only geometry and immutable structural inputs, with no node mirrors.

## 2) Freeze IRs and invariants

Status: the main identity and side-table decisions are established; some
representation details and validation semantics remain open.

Frozen decisions for the current phase:

- `BoxID` is unique per box; `NodeID` is for source mapping and may be shared by
  split inline fragments. All side tables use `BoxID`.
- Anonymous IDs are deterministic from the allocation parent and run/traversal
  order, with build-local collision resolution; no shared sentinel ID.
- Generated BoxIDs are nonzero and unique within a build. Candidate arithmetic
  uses an allocated-ID set and deterministic probing, including wraparound.
  Identical builds and omission of empty text preserve IDs; arbitrary structural
  edits need not. Allocation parents may precede normalization wrappers, so IDs
  do not encode final tree paths. NodeID is the source association across edits.
- Used values live in `UsedValuesTable[BoxID]`, outside the structural tree.
- `LayoutGeometryTable[BoxID]` exclusively owns geometry and is fresh per flow.
  LayoutResult.Root shares the immutable structural tree. No node mirrors remain.
  Callers/adapters must treat published trees, styles, and results as read-only.
  Resize recomputes used values; changed styles/structure require a new tree.
- Frame is the border box; Content excludes padding and borders. Margins remain
  separate. Coordinates are relative to parent content boxes; root origin is (0,0).
- Empty block containers receive one empty `BoxAnonymousInline` child.
- The inline API returns extent and atomic placements; its H sets owner content height,
  including paragraph trailing space. Core line storage and opaque payloads are
  removed. This supersedes non-anonymous-only line retention.
- Typed paragraph results belong to an invocation-local adapter table keyed by
  group BoxID with owner BoxID, including anonymous groups. Publish with geometry
  only on success. Group/owner identities already exist; no vlist BoxID is added.
- Text references use a stable `TextSourceID` and non-empty `[Start,End)` byte
  ranges. Empty ranges are dropped without destabilizing other IDs.
- Inline-blocks are atomic inline participants with an internal block container.
  Core owns recursive discovery, sizing, and dependency scheduling: finish inner
  groups and atomic block stacks before delegating an outer group. Prefer recursive
  core helpers; an explicit work stack is optional. This supersedes adapter-driven
  AtomicSizer callbacks and removes that reason for adapter reentrancy.
- The request's external sizing callback is replaced by read-only group-local
  prepared items keyed by existing atomic BoxID. Implemented contract:
  request.Atomics PreparedAtomicsView with Lookup(BoxID) -> PreparedAtomic, copied
  border dimensions, one view per group, branch lookup by node.BoxID. Local whatsit
  IDs map back to BoxID; encoding/position storage stays private. Current code
  uses Atomics lookup; core prepares sizes, P determines line placements.
- Implemented output: InlineResult{Extent, Atomics}, with each
  AtomicPlacement carrying BoxID and border-origin X/Y relative to request.OwnerID
  content. This narrowly extends extent-only returns, without paragraph summaries.
  LayoutGeometry.RelativeTo makes coordinate parents explicit: atomic
  owner, stacked-block container, or zero at root. Core validates then applies
  placements once, without moving local descendants or adding margins again.
  These types are implemented and integrated. Tytui emits ordered U+FFFC anchors
  and retains occurrence-to-BoxID/source associations. The bounded profile uses
  normal CSSLineHeight, AlignBottomMarginEdge, zero side skips and NoHyphenation.
  General CSS baseline and per-span hyphenation policies remain deferred.
- Fixed CSS-pixel recursive support comes before auto sizing. Checklist 2A/3A
  specifies prepared access and both adapter/P contracts, then dependency-order
  fixtures, core scheduling/sizing/resolution, API migration, and fake-adapter
  proof. Prepare each atomic once per invocation initially. Auto sizing follows measurement-first 2B/3B; speculative trials and their
  acceptance/discard protocol remain outside that slice.

Anonymous style rules and explicit pending auto-width/edge state are implemented.
Remaining: broader geometry scope,
and validation predicates that distinguish block containers from block-level
flow participants. The detailed validation checklist lives in
`doc/Current-Checklist.md`; these are requirements, not completed checks.

## 2.1) Frozen scope and deferrals

Keep frozen decisions here; no separate decision log is needed.

- CSSDOM creation, line breaking, and text shaping are external concerns.
- Used edges must be available before the line-breaking decisions that need them.
- Margin-collapse scope is frozen as of 2026-11-16 in the
  [core contract](CSS-Algo-Overview.md#margin-collapse-contract): the existing
  normal-flow block profile, including nested and empty blocks. Steps 1 (scope),
  2 (eligibility and inline permission), 3 (immutable summaries), 4 (collapsed
  placement/height accounting), 5 (bounded core regression audit), and 6 (adapter
  integration) are complete. Empty CSS-processed paragraphs now have no line strut; height constraints and
  additional formatting modes remain deferred.
- Bounded shrink-to-fit is accepted for 2B/3B, superseding the max-content-only
  approximation plan. Full CSS intrinsic/cyclic-percentage semantics remain deferred.
- IntrinsicMeasurer measures normalized groups, not whole atomic subtrees:
  MeasureInline returns min/max content bounds, with edges-only input and nested
  atomic border bounds keyed by BoxID. Core owns recursion and width selection.
- Use containing-block available content width, not line remainder; select
  min(max(minContent, availableContent), maxContent), resolve descendants privately,
  then lay out once. Reject unknown-width percentage dependencies initially.
  Measurement never publishes geometry or paragraph results. No speculative-layout
  protocol or baseline expansion is required for this slice. The bounded interfaces are implemented;
  see Pass-Signatures.md section 4.1 and checklist 2B/3B.
- Floats, general positioning, z-index, and painting/stacking contexts remain
  deferred. Fixed branch construction is implemented below; its sizing/placement
  remain planned.
- Fine inline metrics, span-level line-height, and inline background/border
  fragments are deferred.
- Outer flow is LTR only; outer RTL and vertical writing modes are deferred.
  Delegated paragraphs already support bidi; real-font proofs verify that path.
- Adjacent text merging and selection/caret mapping to DOM/text are deferred.
- Core/global and cross-invocation caching remain deferred. Tytui preparation
  reuse is invocation-local, separate from final attempts and accepted results.
- Convert each CSS primitive with round(px * 49152), then check signed sums.
  Keep CSS border dimensions unquantized and paragraph positions float64 big
  points. Budget half a DU per primitive plus each float32 CSS narrowing.
  Positive overflow remains valid; negative completed line widths are unsupported.
- Tytui publishes geometry and all accepted paragraphs together after success.
  Failed final callbacks remain attempted; no same-invocation retry is allowed.
  Results retain immutable normalized input and stable font/scale services.
- Glyph coverage is caller-owned; no new validation is introduced. Relocation of
  existing itemizer checks is separate future work. Bounded explicit discretionary
  measurement/reuse exists below CSS; automatic intrinsic hyphenation and
  successive splits remain deferred.

Auto-margin distribution has incomplete implementation and needs a decision;
this roadmap does not silently add it to the frozen deferrals.

<a id="fixed-layout-plan"></a>
## 2.2) Fixed-layout branch plan (2026-11-20)

Status: all three implementation slices are complete. User-agreed direction:

- Divert fixed boxes before FlowLayout; classify before normal-flow normalization.
- Give fixed boxes a separate sizing resolver.
- Expose an independently callable FixedLayout branch taking a viewport rectangle.
- Stop at that external-client boundary. Do not infer or implement page-layout
  behavior from external paged-media standards.

Controlled PR sequence agreed with the user and implemented:

1. **Typed input and branch construction — implemented.** Added typed static/fixed positioning,
   insets with explicit auto defaults, and fixed-root height constraints. Position
   and insets are not inherited, including by anonymous wrappers. Preserves
   existing normal-flow default behavior and rejects unsupported positioning and
   legacy string values rather than silently treating them as static. The optional
   ComputedStyle.Fixed points to a copied FixedStyle (Insets and Height); nil means
   auto, DefaultFixedStyle sets auto, and a non-nil zero record means explicit zero.
   Static boxes reject Fixed constraints. The additive BuildLayoutTrees result is in
   [Pass-Signatures](Pass-Signatures.md#fixed-layout-api). The old normal-only builder
   returns an explicit error for otherwise discarded fixed roots. One ID allocator
   serves both branches; source inheritance/order is preserved and nested fixed
   boxes are diverted to the same viewport branch. No fixed participant may
   enter its outer flow's normalization, measurement, atomic preparation or stacking.
   Source association remains available for later static-position work; there is
   no contributing placeholder.

2. **Dedicated sizing resolver — implemented.** Internal resolveFixedSizing
   resolves one non-replaced fixed root in the LTR profile. It supports px/em/%,
   auto/stretch/shrink-to-fit widths, height constraints, auto margins and
   overconstraints, requiring at least one non-auto inset on each axis. Negative
   insets/margins and finite overflow are allowed. Percentages use axis-correct
   bases; percentage margins/padding use width. A negative stretch is re-solved
   at zero minimum content size, including auto-margin redistribution.
   Root used width/edges are separated from explicitly pending content height;
   withContentHeight completes that height later and preserves prescribed sizes.
   Intrinsic measurement uses the existing recursive service contract. Validation
   covers normalized subtree shape, unique IDs, styles, effective em font bases,
   numeric finiteness and the existing descendant atomic profile. No final inline
   callbacks, descendant used-value resolution or geometry publication occur
   in this resolver.
   Tests cover the equations, percentage/font bases, numeric limits, nested
   measurement and failure, height completion, resize, immutable snapshots and
   concurrent replay. See [the private contract](Pass-Signatures.md#fixed-layout-api).

3. **FixedLayout integration and client API — implemented.** Resolves fixed width and descendant
   widths before final inline callbacks, sharing an invocation-local intrinsic
   cache. Validates the entire forest and cross-root identities before measurement.
   Lays out each fixed interior as an independent formatting context using shared
   recursive helpers. Its margins never enter outer flow; internal normal-flow
   margin rules still apply. Separates
   natural content extent from prescribed fixed-root height, then finishes vertical
   sizing and placement. Keeps descendant coordinates local; does not rerun paragraph
   layout just to translate a bottom/right-anchored root. This branch has no
   scheduling dependency on FlowLayout within the supported explicit-inset profile.

   Exposes FixedLayout and a result carrying a copied root slice, viewport and
   fresh geometry. Resolves internally so callers cannot accidentally supply
   normal-flow used widths for fixed roots. Preserves
   callback transaction ownership, input immutability, and independent concurrent
   calls. The external-package ExampleFixedLayout builds branches and calls both
   APIs independently. Integration/lifecycle tests cover geometry, nested atomics,
   margin containment, descendant resolution, complete publication, late failures,
   resize/replay and concurrent calls. External clients can obtain positioned
   fixed-box geometry and retain their paragraphs; this completes the milestone.

4. **Geometry-only service (BOX-01) — implemented 2026-11-22.**
   `ResolveFixedGeometry(ComputedStyle, Rect, FixedLayoutOptions)` returns one
   viewport-local `LayoutGeometry`, sharing typed constraint preparation, the
   axis solver and rectangle construction with `FixedLayout`. It requires no
   DOM/tree/identity or content callbacks and rejects content-dependent sizing
   explicitly. Invalid input/overflow, unsupported static-position fallback and
   content dependencies have distinct errors. Zero sizes and finite overflow
   retain core semantics; page policy stays downstream. Tests cover direct
   geometry, parity with the tree path, callback validation order, invalid inputs,
   percentages/em, ownership, resize/replay and concurrency. Existing content
   layout and callback contracts remain unchanged.

Build accepts structurally valid auto constraints without performing sizing. PR 2
rejects static-position fallback (both insets auto on an axis) in its initial
profile. Defer ancestor-established fixed containing blocks, fixed replaced roots, additional writing modes and
broader size constraints. Unsupported dependencies must fail explicitly within
core's input contract or be rejected by the adapter before projection; they are
not approximated as viewport-relative supported inputs. Source identity is
preserved, but detailed static-position anchors/callbacks need a later design.

Also outside this plan: painting/z-index order, clipping policy, page assignment,
repetition, pagination, page-instance IDs, page composition and register-based
storage of fixed content. The client controls subsequent use of returned results.
PR 1 tests compare normal-flow structure, geometry, intrinsic measurements and
paragraph callbacks with fixed content omitted, including margins, whitespace and
auto inline-blocks. They cover branch order, shared identity, source style copying,
root/default/error contracts and concurrent deterministic builds. Acceptance cases
are in the [checklist](Current-Checklist.md#fixed-layout-checklist).

## 3) Define pass boundaries and signatures

Status: all three entry points exist in `layout/`. `doc/Pass-Signatures.md`
describes intended boundaries, but signatures and types need reconciliation
with the code and `doc/Tree-Type.md`.

Remaining:

- Resolve concrete `RenderNode` versus read-only adapter interfaces.
- Align broader options/context roles and geometry coverage. Current numeric
  boundaries and typed result ownership are settled above.
- Preserve implemented 2B/3B dependency resolution: select auto widths, privately
  re-resolve descendants, then expose only definite used values to final layout.

Success: explicit inputs/outputs, no hidden dependencies, and documented pass
ownership. The proposed package split is provisional; a package reorganization
is not a prerequisite for correcting behavior.

## 4) Algorithm sketches and test outlines

Status: `doc/Pass-Algorithms.md` contains sketches; tests already exist for tree
construction, used values, and flow layout. This is no longer a pre-code phase.

Remaining: update sketches as semantics are settled and add meaningful tests
for the acceptance targets in the current checklist, including integration
across passes. Existing tests do not establish full pipeline correctness.

## 5) Complete Pass 1: BuildLayoutTree

Display migration stages 3/4 (2026-11-16): typed `RenderNode.Display` replaces
string display input. All four block/inline, flow/flow-root combinations and
subtree suppression are supported. The entry point validates a non-replaced
block flow or block flow-root root instead of forcing BoxBlock; nil/display:none
produce no tree. `LayoutNode.EstablishesBFC` records independent flow-root
interiors separately from FCBlock and outer participation. It contains child
margins and blocks through-collapse without preventing a block flow-root's own
outer margins from collapsing. Ordinary/anonymous blocks do not inherit it;
supplied-root and inline-block behavior is retained. Numeric tests cover signed
margins, empty chains, parent/sibling adjacency, normal block sizing and nested
fixed/auto atomics. Core fixtures and tytui use typed display; tytui resolves normalized values
through w3c-dom GetDisplay. Unused string accessor/tree-view APIs are removed. Previous integration evidence below
predates this input-contract change. Floats/clearance remain outside the profile.

Status: anonymous wrapping and split/hoist logic are implemented with basic
tests. Mixed-run slice ownership and empty normalization are fixed. BoxIDs now
have collision protection and regression coverage for uniqueness and stability.

Typed style/source transport (task 1), empty normalization (task 4), identity
protection (task 5), and distinct mixed runs (task 8) have regression coverage.
Anonymous wrappers use auto width, zero edges, and inherited font size.
The external tytui adapter/source lookup is integrated and tested. Broader
supported behavior remains open.

Minimal source-reference plumbing and supported typed style transport are
implemented for task 1 / M1; `doc/Tree-Type.md` documents their input rules.
General CSSDOM adapter integration remains separate; selection/caret mapping is
still deferred. Separate tytui tests now establish the bounded actual wiring.

## 6) Complete Pass 2: ResolveUsedValues

Status: edge and width resolution, side-table output, and basic context tests
exist. Auto inline-block widths/edges use explicit pending flags until intrinsic
selection and private re-resolution; unresolved width is not a resolved zero.

Fixed-width descendant contexts and explicit zero behavior are tested (3A).
Late auto-width resolution (3B) and LTR horizontal block margin distribution
(task 6, 2026-11-15) are implemented. Block margins below pending auto widths are
also pending until private descendant re-resolution. Computed styles remain
unchanged; used right margins account for the LTR width constraint. Inline-block
and vertical auto margins remain zero. See the core contract and
`layout/auto_margins_test.go`; root origin and immutable-input contracts remain.

## 7) Complete Pass 3: FlowLayout

Status: block stacking, extent-and-placement inline delegation, and geometry tables are
implemented. Typed fake adapters verify group/owner requests, full extents,
trailing space, anonymous/empty groups, and input/output error handling.

Nested parent-content-relative coordinates are fixed and regression-tested with
three-level nesting and nonzero edges (task 10, 2026-11-13).
M4 removes all node geometry writes, including atomic callbacks. Resize/failure
and concurrent-read regressions pass, including full tests, vet, and race checks.
Fixed-width recursive atomic sizing, placement and descendant resolution are
implemented (2A/3A). Bounded auto/intrinsic sizing (2B/3B) is implemented. Broader geometry
coverage remains task 7.

## 8) Execution order and completion gate

The bounded document experiment is complete; see
[Document-Layout-Integration.md](Document-Layout-Integration.md#7-bounded-integration-evidence)
and the [checklist crosswalk](Current-Checklist.md#execution-order-and-integration-crosswalk).
The completed sequence is:

1. M1: preserve styles/source references and distinct runs, normalize empty
   containers and enforce identity; tytui extracts actual normalized groups.
2. M2: implement extent/placement callbacks, explicit unit conversion and typed
   invocation-local retention with transactional document publication.
3. M3-M4: verify fake-paragraph coordinates/snapshots, then actual nested geometry,
   resize, failure recovery, accessor detachment and retained-result lifetime.
4. M5-M6: exercise synthetic typography and real-font proofs without remeasurement,
   including independent glyph controls and unchanged paragraph replay evidence.
5. 2A/3A and 2B/3B: integrate fixed and bounded auto recursive atomics through
   actual measurement -> width selection -> resolution -> final layout. Verify
   preparation reuse, signed rounding and retained proofs under both breakers.

Validation on 2026-11-15: ordinary/race suites, build and vet passed in khipu,
typography, tytui and css-box-layout, including dependency audits and adapter/
proof/replay controls. No production changes were required for stage 5 verification.
The experiment uses explicit non-auto margins and normal line-height. Remaining
core geometry/validation/cleanup tasks stay unchecked; automatic intrinsic
hyphenation, speculative trials, cross-invocation caches, expanded percentage or
baseline policies, full painting, pagination, TUI wiring and document replay are
not part of that stage's completion. Subsequent horizontal auto-margin work is
implemented for LTR normal-flow blocks, with upstream and tytui regressions;
it does not broaden outer-direction or fragmentation policies.

Finishing the document experiment does not complete the broader core roadmap.

The current phase is complete when the agreed structural, identity, used-value,
geometry, and immutability acceptance criteria have tests; a styled end-to-end
pipeline is exercised; and docs describe implemented behavior and explicit
remaining deferrals. Run `go test ./...` when validating implementation changes.
Do not infer completion from the existence of pass entry points alone.

<a id="margin-collapse-review-groups"></a>
### Margin-collapse review groups

The six implementation steps form three logical review groups. They are still
working-tree changes, not three commits or independently applicable patches.

| Review group | Main code and evidence |
|---|---|
| 1. Eligibility, contracts, and immutable summaries | `layout/interfaces.go`, `box.go`, `margin_collapse.go`; eligibility and summary tests |
| 2. Collapsed placement and geometry | `layout/layout.go`, `atomic.go`; placement/regression tests and updated coordinate expectations |
| 3. Adapter integration and documentation | Tytui `documentlayout/layout.go`, `margin_collapse_test.go`, `margin_permission_test.go`, README; core contract, checklist and integration evidence |

Group 2 depends on group 1; group 3 depends on both. Existing unrelated tytui
tracing/workbench changes are outside these review groups. No Git history or
unrelated changes were staged or rewritten during this integration.
