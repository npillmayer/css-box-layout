# Core Algorithm Contract (CSS Box + Flow Layout)

Normalization provenance is available through `BuildLayoutTreesWithProvenance`.
It is an out-of-band result of the same builder, not layout material. Source
before/after endpoints, retained occurrences and per-root structural ordinals
preserve split/hoist and anonymous-group correspondence. Omitted/empty/suppressed
endpoints are explicit and do not imply a nearby position or consumer affinity.
Accessors return detached copies; identity is build-local. See
[boundary inspection](../layout/README.md#inspect-normalized-source-boundaries).

Implementation update: tasks 2A/3A and bounded auto-width tasks 2B/3B are
implemented. Core measures nested intrinsic bounds, selects auto content widths,
resolves descendants privately, then performs final layout. Fixed pixel widths
including zero remain supported; percent/em widths on atomic boxes remain rejected.
Percentage dependencies on unknown auto widths fail explicitly; definite bases
remain supported. Horizontal block auto margins are now resolved in the core
(2026-11-15), including after late containing-width selection. See [current API](Pass-Signatures.md#intrinsic-measurement).
Diagrams illustrate the fixed-width final-layout phase, not intrinsic measurement.
PREP-03 (2026-11-23) exposes `MeasureIntrinsic` for standalone border-box bounds
using the existing recursive pass. It adds no used sizing, final heights, geometry,
font dependencies or persistent cache; input and failure contracts remain unchanged.
Bounded tytui/typography integration is verified through pipeline stage 5
(2026-11-15): normalized input, intrinsic/final callbacks, nested geometry,
transactional result lifetime and retained real-font proofs. See
[the integration evidence](Document-Layout-Integration.md#7-bounded-integration-evidence).
Full CSS baseline behavior, speculative trials and broader core work remain open.


This document is the single, authoritative contract for the core CSS layout pipeline in this repo. It consolidates the spec surface previously spread across `doc/Algo-Spine.md` and `doc/Functional.md`.

Implementation status and open work: `doc/Exec-Roadmap.md` and
`doc/Current-Checklist.md`. Requirements here are not claims of completed code.

---

## 1) Scope and assumptions

Inputs and environment:
- A DOM annotated with computed styles (CSSDOM) already exists.
- A line-breaking module exists but is treated as a black box.
- Outer flow is LTR only; outer RTL and vertical writing modes are deferred.
  Delegated paragraphs may contain bidi text, as verified by tytui proofs.
- Text positions are `uint64` byte offsets; empty ranges are dropped.
- Coordinates are relative to the parent content box; root origin is (0,0).
- Adjacent text-node merging and core selection/caret mapping are deferred.
  Tytui retains decoded DOM byte provenance outside the core.

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
- Require explicit typed `RenderNode.Display`; reject legacy display strings
  and missing/malformed element values. Text nodes carry no display property.
- Accept a non-replaced block flow or block flow-root element root;
  nil or display:none yields no tree. Reject other roots instead of blockifying them. The supplied subtree
  root retains its existing containment boundary, not viewport semantics.
- Support block flow, block flow-root, inline flow and inline flow-root (atomic
  inline-block). Suppress display:none subtrees. Non-replaced flow-root containers
  set `LayoutNode.EstablishesBFC`; block flow-root retains `BoxBlock` outer
  participation and normal block widths, independent of this containment flag.
- Insert anonymous boxes for mixed inline/block children, except runs made
  entirely of text that collapses away under computed `Style.WhiteSpace` and
  inline wrappers with zero edges. `BuildOptions.ReadText` supplies source ranges;
  core derives suppression without adapter permissions. Atomic boxes remain
  significant. Explicit empty blocks retain their boxes; empty processed CSS
  paragraphs produce no line height in the inline service.
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
- Normal and anonymous blocks solve the horizontal width constraint under the
  LTR policy: positive free space goes to one auto margin or equally to two;
  auto width keeps auto margins zero. Overflow and all-fixed constraints solve
  the right margin, which may differ from its computed value and may be negative.
  Padding and borders count before distributing space. See
  [CSS 2.2 section 10.3.3](https://www.w3.org/TR/CSS22/visudet.html#blockwidth).
- Inline-block/ordinary-inline auto margins and vertical auto margins remain zero.
  The root remains anchored at (0,0); block children use their resolved left margin.
- Auto inline-block widths are selected by intrinsic sizing. Descendant block
  margins depend on that width even if their own width is definite: mark them
  EdgesPending and recompute on the private final used-values table before any
  layout callback. Intrinsic contributions use undistributed computed edges;
  centering space must not inflate min/max-content widths.
Notes:
- Percent lengths are represented as 0..1 in `Length.Value` (e.g., 0.10 = 10%).

### Phase 3: FlowLayout (E -> F)
Inputs:
- Layout box tree `E` + used values table + inline layouter + intrinsic measurer.

Outputs:
- Layout result `F` (structural root reference + authoritative box geometry).

Responsibilities:
- Block layout: vertical stacking with margin collapsing for the supported
  [normal-flow profile](#margin-collapse-contract).
- Inline layout: delegate to inline layouter when inline-only.
- Use the returned overall inline extent for content height. Line/glyph results
  are retained by the integration adapter, including for anonymous owners.

### Fixed-box branch (client API implemented 2026-11-20)

BuildLayoutTrees now diverts fixed boxes before normal-flow normalization and
returns LayoutTrees{Flow, Fixed}. Fixed roots use BoxBlock with independent
formatting-context containment; their interiors are normalized independently.
Position is orthogonal to BoxKind. BuildLayoutTree rejects input that would lose
a fixed branch; ResolveUsedValues and FlowLayout reject fixed participants before
resolution or callbacks, including in manually constructed trees.

Typed input uses ComputedStyle.Position (zero/static or fixed) and optional
ComputedStyle.Fixed constraints. Nil Fixed means auto insets/height; a supplied
FixedStyle is copied. DefaultFixedStyle initializes auto values, while a non-nil
zero FixedStyle means explicit zero. Static boxes cannot carry Fixed constraints;
position/insets are not inherited. Fixed replaced roots and other positioning
schemes are rejected. Non-replaced fixed roots accept all supported display pairs;
nested fixed roots are diverted in source preorder with one shared ID allocator.
Both-auto axes are retained at build time and rejected by the bounded fixed resolver.

The shared typed `prepareFixedConstraints` kernel accepts computed style and an
explicit viewport without a tree. The internal `resolveFixedSizing` adds tree
measurement when needed. Together they resolve px/em/percentage dimensions and insets,
auto/stretch/shrink-to-fit widths, auto margins, and overconstraints under the
current LTR profile. Horizontal percentages use viewport width; vertical insets
and height use viewport height. Percentage margins/padding retain a width basis.
Each axis requires at least one non-auto inset. The zero minimum content size is
applied by re-solving with size zero if a stretched tentative size is negative.

Root width is final before any final paragraph layout. Content-based height stays
explicitly pending; withContentHeight later returns a completed value without
mutating the prior one. Prescribed/stretch height is retained despite natural
content overflow. Intrinsic measurement is reused for shrink-to-fit, never for
height or final paragraph results. Inputs and prior results stay read-only.

FixedLayout accepts the diverted roots, viewport, inline/intrinsic services and
FixedLayoutOptions (fallback FontSizePx). It validates the whole forest, including
cross-root identities, before measurement. Root sizing and descendant preparation
share an invocation-local intrinsic cache; all widths resolve before final inline
callbacks. Shared recursive helpers lay out interiors once, then complete height
and position roots without translating descendants or repeating callbacks.
Fixed-root frame validation uses the completed height; no temporary natural-height
frame is constructed. Natural extents and descendant geometry still require finite values.
The fixed root contains child margins; ordinary internal sibling collapse remains.

FixedLayoutResult carries a copied Roots slice, the Viewport, and fresh Geometry.
Root Frame/Content coordinates are viewport-local with RelativeTo zero; add the
viewport origin once externally. Descendants keep their content-coordinate
parents. Empty input succeeds for a valid viewport, including definite zero size.
Any validation, service or geometry failure returns no partial result; the client
also discards that invocation's adapter-retained paragraphs. Trees/styles remain
shared and read-only across resize, replay and independent concurrent calls.
The normal-flow and fixed branches share one build's BoxID namespace but return
independent immutable geometry. Normal-flow widths, heights, paragraph grouping,
and margin collapse must not include fixed participants.

`ResolveFixedGeometry` (BOX-01, implemented 2026-11-22) exposes the same typed
kernel and rectangle construction directly. It accepts one `ComputedStyle`,
viewport and `FixedLayoutOptions`, returning a detached `LayoutGeometry` without
tree construction or callbacks. Both axes must have a definite content size or
opposing-inset stretch. `ErrFixedContentDependency` rejects unresolved content
sizing; `ErrUnsupportedPosition` rejects both-auto static-position fallback and
non-fixed positioning. `ErrInvalidFixedGeometry` classifies invalid typed inputs,
missing effective em font context and numeric overflow. Errors return zero
geometry. Local-coordinate, zero-minimum, percentage-basis and immutability rules
match `FixedLayout`; strict positive receivers and bounds remain caller policy.
Tree validation and content-dependent measurement/layout remain in the existing
tree path. Definite vertical validation still precedes intrinsic measurement.

The implementation boundary ends at an external client calling this branch and receiving
its geometry and adapter-retained paragraph results. There are no page-layout
rules in this extension: no automatic repetition, pagination, page assignment,
page-instance identities, or page composition. The viewport is supplied by the
client; core does not infer it from the normal-flow root or a page standard.

Agreed direction and completed milestones are in
[the roadmap](Exec-Roadmap.md#fixed-layout-plan); client signatures and
coordinate/ownership contracts are in
[Pass-Signatures](Pass-Signatures.md#fixed-layout-api).

<a id="margin-collapse-contract"></a>
### Normal-flow margin collapsing

Core eligibility, immutable summaries, and collapsed placement implemented
2026-11-16, with consuming-adapter integration verified. This slice covers the existing
normal/anonymous block model with auto height and zero minimum
height. `ComputedStyle` does not yet carry height or min/max-height properties;
these assumptions describe the supported input profile, not new style support.

Rules for this slice, based on
[CSS 2.2 section 8.3.1](https://www.w3.org/TR/CSS22/box.html#collapsing-margins):

- Adjoining vertical margins of in-flow block-level boxes in the same block
  formatting context collapse; horizontal margins do not.
- Include sibling bottom/top, parent/first-child top, and last-child/auto-height
  parent bottom margins, transitively across nested boxes.
- Separating borders, padding, or non-ignorable line boxes prevent adjacency.
  Check the relevant edge: a parent's bottom border does not block top collapse.
  Sibling borders do not separate their outside bottom/top margins.
- Eligible empty blocks collapse through: no vertical border/padding or
  separating lines, no new block formatting context, and all in-flow children
  (if present) also collapse through.
- Supplied root, flow-root and inline-block margins do not collapse with their
  children or through themselves, even at zero height. Eligible sibling blocks
  inside these containers still collapse. A block flow-root's own outside margins
  can collapse with adjacent blocks and with an eligible ordinary parent.
- For an entire adjoining group, use `max(0, margins...) + min(0, margins...)`.
  Keep both extrema until the group is complete; pairwise scalar reduction loses
  information. Zero margins participate normally.

Acceptance examples (CSS pixels):

| Adjoining margins | Result |
|---|---:|
| 13, 8 | 13 |
| 13, -8 | 5 |
| -13, -8 | -13 |
| 13, -8, 10 | 5 |
| 0, 0 | 0 |

Repository boundaries:

- Apply collapsing during FlowLayout after final used-value resolution; preserve
  the input margin values. Vertical auto margins already resolve to zero.
- Treat the supplied layout root as the containment boundary and retain its
  existing (0,0) origin. Viewport/body margin propagation is outside this slice.
- Preserve anonymous wrappers and inline delegation, including empty groups.
  `FCBlock` currently labels all block containers, not just formatting-context
  roots. `LayoutNode.EstablishesBFC` records display-derived flow-root containment;
  it is not inherited by ordinary or anonymous descendants. Internal eligibility
  also preserves explicit supplied-root and inline-block boundaries.
  `InlineResult.AllowsMarginCollapse` supplies inline permission;
  false is conservative even at zero extent. True requires zero height and no
  atomic items, and certifies no separating line boxes. Neither an empty wrapper
  nor zero extent alone establishes eligibility. Positive reserved paragraph
  space remains consumed. Eligibility propagates through block children.
- Recursive layout returns an invocation-local value summary: eligibility,
  leading/trailing margin struts, and completed border dimensions. Each strut
  preserves positive/negative extrema with pure merge/value operations. Leading
  and trailing groups include adjoining empty-child chains and stop at separating
  content; internal sibling groups do not escape. A through summary exposes the
  same group at both ends, to be consumed once by future placement. Containment
  boundaries retain their own margins without absorbing descendant groups.
  UsedValues margins remain unchanged. Dimensions match completed collapsed
  geometry and feed atomic realization.
- Placement carries a pending adjoining group until the next separating child.
  Groups escaping through an open parent edge are excluded from content height;
  contained groups are counted once. Through boxes do not advance the cursor.
  Their own origins use a separate hypothetical-bottom-border leading strut,
  unless they adjoin the parent top, where their border origins coincide.
  Descendant-local coordinates and paragraph results are not translated or
  recomputed. Negative margins may overlap boxes; content height is clamped to
  zero and non-finite placement/dimensions fail without publishing a result.
- Explicit heights, min/max heights, floats, clearance, positioning, additional
  overflow/display modes, and vertical writing modes remain deferred. This
  contract does not introduce their style fields, parsing, or layout semantics.

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
- No derived geometry on nodes; Frame and Content exist only in the result table.

TextRef:
- TextRef{ Source TextSourceID, Range: [Start,End) }
- Empty ranges are dropped at build time.

LayoutResult (F):
- Root *LayoutNode (shared immutable structural tree reference)
- Geometry table `LayoutGeometryTable[BoxID]` (fresh per invocation, authoritative)
- Published trees, styles, and result tables are read-only by ownership contract.
  Reuse the tree for resized constraints; build a new tree for style/structure edits.
- No core line table or opaque paragraph payload. The integration layer retains
  a typed group-BoxID -> owner-BoxID + paragraph-result table per invocation.
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
- Current API: LayoutInline(InlineRequest) -> InlineResult{Extent, Atomics}, error
- Implemented task-2A boundary: LayoutInline(InlineRequest) -> InlineResult{Extent, Atomics}, error
  - Request: anonymous inline Group (Group.BoxID is the retention key), OwnerID,
    definite ContentWidth in CSS pixels, a UsedValuesView, and read-only realized
    atomic items through Atomics; see the implemented realization boundary below
    and current signatures in Pass-Signatures.md.
  - Extent is finite and nonnegative, including at zero available width. H is
    authoritative consumed height, including paragraph-owned trailing spacing;
    W may exceed available width but does not resize the owner's content width.
  - Extent excludes owner margins/padding/borders and is not glyph ink bounds.
    Empty groups may return zero or reserved nonzero height; no dummy lines.
  - Paragraph-local lines, baselines, glyphs, and precise geometry remain in the
    adapter's concrete retained result, never in an opaque core payload.
  - Adapter retention is invocation-local, with group -> owner association for
    every group. Publish it with document geometry only on successful completion.
    No new BoxID is allocated for the vlist. Repeated/speculative group calls
    require an acceptance policy before they are supported.
  - UsedValuesView returns value copies. Treat Group and input tables as read-only
    for the lifetime of shared snapshots. Adapter-owned source/style lookup is
    explicit immutable adapter state.
  - Flow validates used-value presence and finite/nonnegative widths and box
    edges (margins may be negative) before callbacks. Invalid extents and callback
    failures return an error with no LayoutResult; adapter errors are wrapped.
    The core never mutates the input tree or used values, on success or failure.
    Callbacks must honor the read-only contract; discard partial retained state.

Atomic realization (implemented 2A/3A boundary):
- Core discovers atomic boundaries through ordinary inline descendants and lays
  out each internal block container recursively before delegating its outer group.
  Ordinary inline spans stay within the current group; nested block children are
  stacked inside their atomic owner. Prefer recursive core helpers; an explicit
  work stack is an equivalent implementation choice, not required machinery.
- External AtomicSizer callbacks are replaced by read-only realized-item lookup by
  existing atomic BoxID. Group-local items have complete finite border-box sizes
  before LayoutInline starts. Missing realized geometry is an error. Current Go code
  uses request.Atomics with
  RealizedAtomicsView.Lookup(BoxID) returning RealizedAtomic{BoxID, BorderWidth,
  BorderHeight}; see Pass-Signatures.md. One view serves one group; the branch's
  node.BoxID is its lookup key. Local whatsit handles map back to that same ID.
- The adapter translates text/styles and realized atomic leaves, not layout
  scheduling. It must not flatten inner text into outer input. Inner paragraph
  calls finish before the outer call starts; this no longer requires reentrancy
  of the adapter. Input trees/tables stay immutable; realized geometry is per invocation.
- Two adapter/P contracts are implemented in tytui/typography: anchored input with identity,
  dimensions, source mapping, alignment/margin/break policy; and identity-preserving
  final-placement output. Core owns internal geometry; P chooses line placement,
  retained by the adapter and reported as AtomicPlacement{BoxID, X, Y}.
  X/Y is the border-box origin relative to request.OwnerID's content origin,
  including alignment/margin effects. Core applies it without adding margins or
  changing dimensions/descendant-local coordinates. Geometry.RelativeTo
  identifies that coordinate parent explicitly (root=0, stacked block=container,
  atomic=request.OwnerID), not necessarily the structural parent. Validate exactly
  one placement per realized item, identity membership and finite coordinates
  before applying; negative positions are allowed. P's input/position storage
  remains private. Neither realized sizes nor provisional origins are positions.
- Start with fixed CSS-pixel widths including zero; do not clamp them to remaining
  line width. Resolve descendants against atomic content width before layout.
  Border-box dimensions include padding/borders but not margins. Lay out each
  atomic box once per invocation initially. Auto/intrinsic sizing follows the measurement-first 2B/3B design below;
  speculative trials and their acceptance semantics remain later work. Inner failure
  prevents dependent outer delegation and aborts publication of pending results.
- InlineResult deliberately adds atomic placements to extent-only
  returns, not line summaries or opaque paragraph payloads. The types and RelativeTo are implemented. Checklist 2A/3A covers migration, realized geometry
  access and bounded alignment policy; 2B/3B covers bounded auto sizing. Full
  baseline alignment remains deferred.

Intrinsic measurement (implemented bounded 2B/3B design):
- MaxContentWidth and InlineIntrinsic are removed. Auto sizing uses
  MeasureInline(InlineMeasureRequest) -> IntrinsicWidths{MinContent, MaxContent}.
  Current types are in Pass-Signatures.md section 4.1.
- Core owns recursive measurement and block aggregation. A request has Group,
  OwnerID, an edges-only view, and intrinsic atomic items keyed by existing BoxID.
  Atomic bounds include padding/borders, not margins; external measurement adds
  margins once. Its result is group content bounds, excluding owner edges.
- Both bounds are finite/nonnegative and ordered. Min-content uses supported
  minimum-content breaks; max-content preserves mandatory breaks but suppresses
  optional wrapping. Nested atomics remain indivisible in outer measurement.
- No available width, provisional content width, height, placement or retained
  paragraph result crosses this boundary. Measurement does not publish geometry.
- Core selects min(max(minContent, availableContent), maxContent), where available
  content is containing-block content width minus own horizontal edges/margins,
  clamped to zero, never remaining line space. It then re-resolves descendants
  in invocation-local used values before one final accepted layout.
- Reject percentage dependencies requiring an unknown auto width initially;
  unresolved is not zero. Independently definite percentage bases remain usable.
  No speculative-layout acceptance protocol is introduced in 2B/3B.

---

## 6) Explicit deferrals (not yet)

- Preserved-whitespace text layout and decorated inline metrics beyond the current
  normal-whitespace document profile. Empty CSS-processed paragraphs now have no
  line height and can collapse through under the [contract](#margin-collapse-contract).
- Floats, general positioning, z-index and painting/stacking contexts.
  Fixed construction and internal sizing are implemented above; public layout remains planned.
- Full CSS intrinsic-sizing and cyclic-percentage semantics (bounded shrink-to-fit
  is implemented under 2B/3B; this supersedes the max-content-only deferral)
- Span-level line-height / fine inline metrics
- Inline fragments for span backgrounds/borders
- Outer bidi/RTL flow and vertical writing modes (paragraph bidi stays external)
- Selection/caret mapping back to DOM/text
- Adjacent text-node merging
- Core/global or cross-invocation caching/memoization. Tytui already reuses
  paragraph preparation within one invocation; it never caches on Document.

---

## 7) Correctness emphasis

- Box tree structure (anonymous wrappers, split+hoist, inline-block internal block formatting) is correctness-critical.
- Used edges must be resolved before line breaking.
- IDs are stable; BoxID is unique per box; split inline fragments may share NodeIDs.

## 8) Open representation decisions

- Define geometry coverage for structural inline nodes, text, and anonymous
  inline roots; a single rectangle may not represent fragmented inline content.
- Auto-width state and LTR horizontal block margins are implemented as described
  above. Outer RTL, min/max constraints and other excluded sizing policies remain
  separate work.
- The current typed adapter subset and anonymous styles are specified in
  `doc/Tree-Type.md`: anonymous wrappers inherit font size, have auto width and
  zero edges; real boxes preserve copied styles and supplied text references.


### Explicit replaced-leaf boundary (2026-11-15)

RenderNode.Replaced opts into a childless atomic inline leaf, with fixed px width
from Style and finite nonnegative px content height from ReplacedContent. Build
copies the supplied size. No anonymous inline group is created inside it, and no
paragraph callback or strut supplies its height. The ordinary box-edge rules and
realized-atomic placement contract apply unchanged; fixed intrinsic width includes
padding/borders. Payloads remain outside CSS core. Auto/ratio image sizing and
block-level replaced items are not part of this bounded contract.

Verification: full ordinary/race suites, build and vet pass in css-box-layout and
tytui. The integration includes mixed direction, zero size and retained proofs.
