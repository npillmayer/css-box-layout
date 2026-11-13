# Document Layout Integration

Assessment and implementation plan, 2026-11-12. Extent-only boundary now implemented
in the core; actual tytui integration remains outstanding.

This document evaluates the collaboration proposed in tytui's
[three document-layout views](../../tytui/doc/DOCUMENT_LAYOUT_VIEWS.md), especially
View 3, and its [exploration](../../tytui/doc/DOCUMENT_LAYOUT_EXPLORATION.md).
It records integration-specific reasoning and acceptance targets, not a second
general roadmap. The extent-only return and adapter-owned typed retention are agreed. Remaining
implementation gaps and integration decisions are identified separately.

The current authoritative contract remains
[CSS-Algo-Overview.md](CSS-Algo-Overview.md). General priorities and implementation
checks live in [Exec-Roadmap.md](Exec-Roadmap.md) and
[Current-Checklist.md](Current-Checklist.md). Current API details are in
[Pass-Signatures.md](Pass-Signatures.md).

## 1. Scope and ownership

The architectural direction fits this module: width goes down; height comes back.

- css-box-layout constructs normalized boxes, resolves supported used dimensions,
  supplies available content widths, and positions block containers.
- The inline-group adapter extracts one complete normalized group, preserves
  source correspondence, prepares typography inputs, and converts units.
- The paragraph engine shapes, breaks, and internally stacks lines, returning
  local geometry and retained glyph content.
- The document compositor places retained paragraphs through box geometry and
  combines them with separate paint information. It does not reshape text.

A paragraph is a leaf of the outer positioning problem. Its lines must not be
stacked again by block flow. Group boundaries come from normalized boxes, not
HTML paragraph tags or a second traversal that reselects DOM subtrees.

The first experiment is continuous normal-flow LTR layout with definite widths
and natural height. Begin with zero edges, then add padding, borders, and additive
margins. Pagination, floats, positioning, flex/grid, atomic inline items, intrinsic
sizing, margin collapse, and DOM line-height extraction remain outside this
experiment. Paragraph bidi capability does not imply outer RTL layout support.

## 2. Readiness assessment

The existing callback direction is suitable, but the complete document path is
not ready. The following table preserves the original pre-fix assessment from 2026-11-12;
it is not current implementation status. See milestones and section 3 for progress.

| Area | Current behavior | Integration consequence |
|---|---|---|
| Inline delegation | Flow passes an anonymous inline root and content width to InlineLayouter | Suitable connection point; flow does not restack returned lines |
| Mixed-run normalization | `normalizeBlockChildren` retains slices whose backing storage is reused for later runs | Later text can overwrite an earlier group's children |
| Styles | Box constructors omit LayoutNode.Style | Render-input widths and edges do not reach resolution |
| Text identity | Builder assigns source 0 and node-local byte ranges | TextRef alone cannot identify distinct source text nodes |
| Paragraph height | Flow derives height from maximum line Y + H | Full vlist extent, including trailing spacing, cannot be expressed independently |
| Result retention | Lines are retained only for BoxBlock and BoxInlineBlock owners | Anonymous groups lose their line payloads from LayoutResult |
| Coordinates | Child placement adds the parent's Content offset | Violates parent-content-relative coordinates when edges are nonzero |
| Snapshots | Flow writes geometry onto shared input nodes | A later layout changes geometry reachable through an older result's Root |
| Request data | Callback receives root, width, and atomic sizer, but no used-value view | Adapter access to resolved inline edges is unspecified |
| Resize | FlowLayout ignores LayoutContext | A constraint change requires used-value recomputation, not just a new flow context |

Relevant source: [flow.go](../layout/flow.go), [layout.go](../layout/layout.go),
[flow_layout_helpers.go](../layout/flow_layout_helpers.go), and
[interfaces.go](../layout/interfaces.go).

Verification during the assessment:

- `go test ./...` passed; existing tests do not cover all integration requirements.
- A temporary mixed-run probe expected source IDs 2 and 4, but observed 4 and 4.
- With parent padding of 10, a child with no margin had origin (10,10), rather
  than (0,0) relative to its parent's content box.
- After a second layout, an older result retained table width 100 while its
  shared Root geometry reported width 200.

These probes established implementation gaps. Mixed-run corruption now has a
fix and regression tests (M1). The coordinate probe now has a three-level
regression and fix (M3, 2026-11-13); snapshot purity remains open (M4). Empty-container normalization and build-local identity guarantees are also
implemented and regression-tested; see M1 and the current checklist.

## 3. Agreed paragraph boundary (core implemented)

The earlier proposal to return line summaries and an opaque retained payload is
superseded. Block flow needs only the vlist's overall layout extent:

```go
type InlineExtent struct { W, H float32 }
// See Pass-Signatures.md for the complete request and interfaces.
LayoutInline(InlineRequest) (InlineExtent, error)
```

H is consumed content height, including paragraph-owned trailing spacing, excluding
owner margins/padding/borders. W is reported overall width and may exceed available
width, but does not resize the containing block. Both dimensions must be finite
and nonnegative. Empty groups may reserve space without dummy lines. Ink bounds
are a different quantity. Core no longer derives height from line rectangles.

The request carries Group (BoxAnonymousInline), OwnerID, definite ContentWidth in
CSS pixels, a read-only UsedValuesView, and AtomicSizer. Group.BoxID identifies
an existing group B; OwnerID identifies its existing owner A. The returned extent
sets A's content height. No new box or BoxID is created for the vlist.

The selected retention contract is a typed, invocation-local adapter table:

```text
Group BoxID -> Owner BoxID + concrete paragraph result
```

P's concrete result or typed reference stays in the integration layer. No `any`,
type assertion, core line table, or core dependency on P is needed. All groups,
including those with anonymous owners, participate. Rendering uses owner content
geometry to place precise paragraph-local lines, glyphs and baselines.

The adapter explicitly records successful paragraph calls in local state and
publishes that table with geometry, source correspondence, and separate paint only
after the whole layout succeeds. Failure discards partial adapter state. The first
experiment assumes one accepted result per group per invocation; speculative or
repeated requests need an acceptance policy before being supported. Full snapshot
purity of shared node geometry is still task 11, not solved by this table alone.

Used.Lookup returns value copies; callers must not mutate the group or input maps
while layout runs. Source/style lookup is explicit immutable adapter state keyed
by source identities. Flow validates used-value presence, finite/nonnegative
widths and box edges (signed margins allowed) before invoking callbacks. Invalid
extents and wrapped adapter errors return nil LayoutResult. Tests demonstrate
three typed retained results, zero-width empty groups, trailing space, and errors.

## 4. Boundary rules and remaining integration work

| Topic | Agreed rule / remaining implementation |
|---|---|
| Width | Definite available content width in CSS pixels for the experiment; specify zero width and reject unsupported unresolved constraints |
| Local coordinates | Paragraph coordinates are relative to the owner's content origin; compose parent transforms exactly once |
| Baselines | Stay in the retained P result; no core baseline field or baseline translation is needed |
| Units | Adapter owns pixel, DU, and fractional-big-point conversion with explicit scale and checked rounding |
| Precision | Preserve precise retained glyph geometry; do not narrow all glyph positions into float32 summaries; final core precision remains open |
| Height | Preserve full layout extent including named compatibility-policy trailing spacing, without double-counting outer margins |
| Paint | Retain separately from geometric summaries and preserve source correspondence through text preparation |
| Errors | Missing used values, unsupported material, and invalid dimensions must fail explicitly rather than silently produce zero geometry |
| Result lifetime | Publish complete immutable snapshots; failed layout must not replace the previous successful document result |
| Resize | Recompute relevant used values and rerun paragraph layout at new widths; caching and incremental invalidation are deferred |
| Atomic items | Excluded initially; existing AtomicSizer signatures do not establish working sizing, descendant resolution, or placement |

Geometry tables already survive later runs independently, but shared node mirrors
break full snapshot consistency. Prefer keeping derived geometry out of input
nodes; if mirrors remain, define copying/ownership before claiming immutability.

## 5. Milestones and acceptance tests

Milestones are proposed work packages, not completion claims. General fixes belong
in css-box-layout; experimental adapter and document proof wiring belong in tytui.
Keep typography's existing single-paragraph results as controls.

Execution tracking lives in the [checklist crosswalk](Current-Checklist.md#execution-order-and-integration-crosswalk):
M1 maps to tasks 1/8/4/5; M2 to task 9 and relevant documentation in 7;
M3 to task 10; M4 to task 11. M5-M6 are primarily tytui integration work.
Inline-block tasks 2/3 remain a later track, not prerequisites for this fixture.
Task numbers are stable identifiers rather than scheduling order.

The M2 core API and fake-adapter tests are implemented under the agreed boundary.
Actual unit conversion and typed publication in tytui remain integration work.
M3 nested-coordinate fixes are core-tested; M4 snapshot purity remains open.

### M1. Preserve the document through box construction

- [x] Carry supported typed styles, source identities, and text references
  through real boxes and split fragments; specify anonymous styles.
  Build -> resolve regressions verify 240/120/240px widths and preserved ranges;
  `doc/Tree-Type.md` describes the input contract.
- [ ] Exercise the actual external document/source adapter fixture in tytui;
  passing core projection tests does not mark that connection implemented.
- [x] Fix mixed-run slice aliasing and test distinct retained runs.
  Each run now owns a copied child slice; four BuildLayoutTree regression cases
  reproduce the old failure and pass with the fix. `go test ./...` passes.
- [x] Exercise empty normalization for blocks and inline-blocks, including
  filtered children, deterministic wrapper identity, and zero-height empty layout.
- [x] Establish build-local generated identity invariants (checklist task 5):
  collision-safe nonzero IDs, deterministic rebuilds, empty-text omission, and
  stability across passes. Arbitrary structural edits may change BoxIDs; snapshot
  ownership must not assume IDs are globally unique across builds.

Acceptance: the shared sample yields G1/G2/G3 with the correct ordered source
content; the accent span stays within G1; resolved widths are 240, 120, and 240px.
The workbench does not repair or rebuild the normalized tree.

### M2. Agree and implement the paragraph boundary

- [x] Adopt extent-only return, typed adapter-owned retention, group/owner
  identity, and a read-only used-values request. Baselines stay inside P.
- [x] Update the authoritative contract and pass signatures; remove core line
  tables and opaque payloads.
- [x] Test trailing extent, empty/zero-width results, typed anonymous-group
  associations, invalid input/returns, and wrapped adapter errors.
- [ ] Implement actual DU/point conversion, rounding checks, and typed document
  snapshot publication in tytui; no real paragraph engine is connected yet.

Acceptance: a fake provider returns complete group extent and retains concrete
paragraph content in its own table without dummy lines or dependence on a named
owner. These core tests pass; real adapter conversion/publication remain open.

### M3. Prove outer positioning with fake paragraphs

- [x] Repair parent-content-relative coordinate handling.
- [x] Record requests by group identity and verify widths and order.
- [x] Return heights H1=24px, H2=36px, H3=12px for the zero-edge sample.
- [x] Add independently specified expectations for three-level nesting with
  padding, borders, and additive margins.

Acceptance: group origins are (0,0), (0,24), and (0,60), with root content height
72px. Parent offsets are applied once, and outer flow never restacks local lines.
These heights are supplied test data, not font measurements.

M3 core verification completed 2026-11-13: nested-coordinate tests specify exact
Frame/Content rectangles and paragraph document origins, including nonzero edges,
signed horizontal margins, sibling stacking, and invariance of descendant local
geometry under root-edge changes. `go test ./...` and `go vet ./...` pass.

### M4. Verify resize and snapshot ownership

- [ ] Change G2 width from 120px to 90px and recompute used values.
- [ ] Have the fake return a deliberately different H2 at that width.
- [ ] Preserve the previous geometry, paragraph payloads, and paint associations.
- [ ] Inject a failed run and verify the prior successful snapshot stays usable.

Acceptance: G3 moves from H1+H2 to H1+H2', and root height follows. The old result
remains internally consistent. A real narrower paragraph need not change height;
the fake intentionally does so to test propagation.

### M5. Connect synthetic typography

- [ ] Extract complete normalized groups and prepare text through the typography
  services, preserving semantic styles, bidi input, and source correspondence.
- [ ] Convert widths and overall extents at the adapter boundary; retain precise local
  paragraph content and full extent.

Acceptance: synthetic paragraph output agrees with existing paragraph controls;
all three groups remain accessible, and paint boundaries do not create unintended
shaping boundaries. No new outer RTL support is implied.

### M6. Render a real-font document proof

- [ ] Compose retained paragraph content and separate paint using box transforms.
- [ ] Display block frames and paragraph bounds as diagnostics before glyph proofs.
- [ ] Use retained font/glyph services without reopening fonts or reshaping text.

Acceptance: the shared sample can be traced from source through boxes, requests,
paragraph results, and final placement. Resize produces a new inspectable result.
The supported subset and remaining exclusions are recorded with the fixture.

## 6. Decision and tracking discipline

Remaining decisions/work: concrete adapter conversion/rounding and retained
precision, geometry mirror ownership, and future speculative-call acceptance.
Extent-only return and adapter-owned typed retention are settled; do not reopen
them as competing core-payload proposals.

When a proposal is accepted, update the core contract and pass signatures, record
frozen decisions in the execution roadmap, and link actionable checks to the
current checklist. Keep this file focused on integration rationale and milestones;
do not maintain a competing specification in tytui.

For implementation work, add owning-package regressions and affected integration
tests. Run `go test ./...`; use build/vet/race checks where changes to integration
or shared result ownership justify them. Mark milestones complete only when their
boundary is exercised, not merely because an isolated package works.
