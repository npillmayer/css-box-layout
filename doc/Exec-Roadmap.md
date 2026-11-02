# Execution Roadmap

High-level milestones for the core layout pipeline, reviewed on 2026-11-02.
`doc/CSS-Algo-Overview.md` is the authoritative algorithm contract.
`doc/Current-Checklist.md` holds prioritized tasks and detailed acceptance criteria.
Implementation status below does not imply that all invariants are validated.

## 1) Consolidate the spec surface

Status: the core contract exists, supported by `doc/Algo-Spine.md` (sequencing)
and `doc/Functional.md` (purity and memoization readiness).

Remaining: reconcile inline-block boundaries in B3, geometry coverage, and
optional geometry mirrors with immutable pass inputs. Track these under
checklist task 7; settle any ambiguity needed by a fix before implementing it.

## 2) Freeze IRs and invariants

Status: the main identity and side-table decisions are established; some
representation details and validation semantics remain open.

Frozen decisions for the current phase:

- `BoxID` is unique per box; `NodeID` is for source mapping and may be shared by
  split inline fragments. All side tables use `BoxID`.
- Anonymous IDs are deterministic, derived from parent BoxID and run index;
  no shared sentinel ID. The implementation must guarantee uniqueness.
- Used values live in `UsedValuesTable[BoxID]`, outside the structural tree.
- `LayoutGeometryTable[BoxID]` is authoritative. Optional node mirrors must agree
  with it; their compatibility with pure pass inputs remains to be resolved.
- Frame is the border box; Content excludes padding and borders. Margins remain
  separate. Coordinates are relative to parent content boxes; root origin is (0,0).
- Empty block containers receive one empty `BoxAnonymousInline` child.
- Line boxes are stored only for inline-only `BoxBlock` and `BoxInlineBlock`
  owners, keyed by BoxID; anonymous block owners do not retain line entries.
- Text references use a stable `TextSourceID` and non-empty `[Start,End)` byte
  ranges. Empty ranges are dropped without destabilizing other IDs.
- Inline-blocks are atomic inline participants with an internal block container;
  atomic sizing returns border-box dimensions.

Remaining: define anonymous styles, explicit auto-width state, geometry scope,
and validation predicates that distinguish block containers from block-level
flow participants. The detailed validation checklist lives in
`doc/Current-Checklist.md`; these are requirements, not completed checks.

## 2.1) Frozen scope and deferrals

Keep frozen decisions here; no separate decision log is needed.

- CSSDOM creation, line breaking, and text shaping are external concerns.
- Used edges must be available before the line-breaking decisions that need them.
- Margin collapsing is deferred; margins are preserved without collapsing.
- True shrink-to-fit is deferred beyond the max-content approximation.
- Floats, positioning, z-index, and stacking contexts are deferred.
- Fine inline metrics, span-level line-height, and inline background/border
  fragments are deferred.
- LTR only; bidi/RTL and vertical writing modes are deferred.
- Adjacent text merging and selection/caret mapping to DOM/text are deferred.
- Caching/memoization is deferred; design for pure, explicit inputs now.

Auto-margin distribution has incomplete implementation and needs a decision;
this roadmap does not silently add it to the frozen deferrals.

## 3) Define pass boundaries and signatures

Status: all three entry points exist in `layout/`. `doc/Pass-Signatures.md`
describes intended boundaries, but signatures and types need reconciliation
with the code and `doc/Tree-Type.md`.

Remaining:

- Resolve concrete `RenderNode` versus read-only adapter interfaces.
- Align options, contexts, numeric precision, and result representation.
- Define how auto inline-block sizing supplies descendant containing widths
  while retaining explicit resolution dependencies and resolved edges before
  internal line breaking (checklist tasks 2 and 3).

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

Status: anonymous wrapping and split/hoist logic are implemented with basic
tests. BoxIDs are assigned deterministically, but uniqueness is not safeguarded.

Remaining: transport computed styles and define anonymous styles (task 1),
normalize empty containers (task 4), strengthen identity guarantees (task 5),
and validate distinct mixed inline runs and atomic inline-block boundaries.

CSSDOM adapter and text-source integration remain deferred plumbing work;
style transport through the existing stub is required for the current pipeline.

## 6) Complete Pass 2: ResolveUsedValues

Status: edge and width resolution, side-table output, and basic context tests
exist. Auto inline-block width is currently represented as zero.

Remaining: distinguish auto from explicit zero, correct inline-block descendant
contexts (tasks 2 and 3), and settle auto-margin behavior (task 6). Resolution
must preserve the agreed immutable-input contract.

## 7) Complete Pass 3: FlowLayout

Status: block stacking, inline delegation, geometry tables, and non-anonymous
line ownership are implemented with basic tests.

Remaining: repair atomic sizing and geometry consistency (task 2), verify nested
parent-content coordinates, define geometry coverage, and reconcile node
mutation with functional constraints (task 7 and the validation checklist).

## 8) Execution order and completion gate

Follow the checklist's priorities:

1. Preserve styles (task 1) and jointly resolve inline-block sizing and descendant
   widths (tasks 2 and 3). Settle the necessary pass-boundary design first.
2. Enforce empty normalization and identity guarantees (tasks 4 and 5).
3. Audit unused abstractions and settle incomplete behavior (task 6).
4. Finish documentation reconciliation (task 7), updating affected contracts as
   decisions are made rather than postponing required design work.

The current phase is complete when the agreed structural, identity, used-value,
geometry, and immutability acceptance criteria have tests; a styled end-to-end
pipeline is exercised; and docs describe implemented behavior and explicit
remaining deferrals. Run `go test ./...` when validating implementation changes.
Do not infer completion from the existence of pass entry points alone.
