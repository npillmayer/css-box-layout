# Document Layout Integration

Assessment and proposed implementation plan, 2026-11-12.

This document evaluates the collaboration proposed in tytui's
[three document-layout views](../../tytui/doc/DOCUMENT_LAYOUT_VIEWS.md), especially
View 3, and its [exploration](../../tytui/doc/DOCUMENT_LAYOUT_EXPLORATION.md).
It records integration-specific reasoning and acceptance targets, not a second
general roadmap. Contract changes below are proposals until explicitly agreed.

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
not ready. The following observations concern the code inspected on 2026-11-12.

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
fix and regression tests (M1); the coordinate and snapshot probes still need
owning regression tests. Empty-container normalization and build-local identity guarantees are also
implemented and regression-tested; see M1 and the current checklist.

## 3. Proposed contract changes

### Explicit group height

Replace the line-only return with a group-level result. Conceptual shape:

```go
// Proposal, not a finalized API.
type InlineLayoutResult struct {
    Lines         []LineBox
    ContentHeight float32
    Retained      any // immutable opaque result or handle
}
```

ContentHeight is the authoritative vertical space consumed by the group. Lines
remain paragraph-local summaries; their extent need not include trailing spacing
or other reserved space. Ink overflow is distinct from consumed layout height.
The core must not inspect typography's vlist or glyph representation.

This would change the current requirement that inline-only content height always
equals line extent. Do not encode full paragraph height using an extra visible
line or an artificially enlarged last line. Empty results must be representable.
The exact payload type and numeric precision remain open.

### Retention for every inline group

Preferred proposal: retain each successful group's result, keyed by its anonymous
inline root BoxID, together with its owning block BoxID. This treats named and
anonymous owners equally:

```text
Group BoxID -> owner BoxID + inline layout result
```

The group's origin is derived from its owner's content geometry. Avoid a second
independent position that could disagree with box layout. Per-line Payload may
still carry line-specific data, but cannot be the only owner of a paragraph:
there may be no lines on which to attach it.

Alternative for the first experiment: an adapter-owned association table scoped
to one layout invocation and published with the complete document result. This
avoids an immediate core retention API change, but requires explicit lifecycle,
error handling, and group-to-owner mapping. It does not solve explicit height.
A long-lived mutable registry is not recommended.

This proposal revisits the current non-anonymous-only line-retention policy;
accepting it requires corresponding contract, result type, and test updates.

### Explicit request dependencies

A future request should identify both group and owner, carry a definite available
content width, and provide read-only access to the relevant used values. Exact
Go types are not frozen. Source and typography styles can be resolved through an
explicit immutable adapter input; ComputedStyle need not absorb all font and
paint properties.

The adapter must consume the normalized group in order, preserving source/style
boundaries without turning each inline span into a separate paragraph. Stable
source references must survive split/hoist and anonymous wrapping.

## 4. Boundary details to settle

| Topic | Proposed agreement / remaining decision |
|---|---|
| Width | Definite available content width in CSS pixels for the experiment; specify zero width and reject unsupported unresolved constraints |
| Local coordinates | Paragraph coordinates are relative to the owner's content origin; compose parent transforms exactly once |
| Baselines | Choose and document line-top-relative or paragraph-origin-relative baseline values; current field has no explicit convention |
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

M2 starts with a decision gate. M1 and existing-contract coordinate fixes may
proceed while proposals are discussed; API changes and full integration acceptance
depend on the agreed boundary. The ownership design must be settled at M2 even
though resize/snapshot behavior is exercised comprehensively at M4.

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

- [ ] Decide explicit height, retained-result ownership, group/owner identity,
  baseline convention, and unit conversion.
- [ ] Update the authoritative contract and pass signatures when accepted.
- [ ] Test trailing extent independently of the last line, empty results,
  anonymous-group retention, invalid returns, and adapter errors.

Acceptance: a fake provider can return complete group extent and retained content
without dummy lines, hidden global state, or dependence on a named block owner.
The selected retention design preserves associations; implementing both
alternatives is not required.

### M3. Prove outer positioning with fake paragraphs

- [ ] Repair parent-content-relative coordinate handling.
- [ ] Record requests by group identity and verify widths and order.
- [ ] Return heights H1=24px, H2=36px, H3=12px for the zero-edge sample.
- [ ] Add independently specified expectations for three-level nesting with
  padding, borders, and additive margins.

Acceptance: group origins are (0,0), (0,24), and (0,60), with root content height
72px. Parent offsets are applied once, and outer flow never restacks local lines.
These heights are supplied test data, not font measurements.

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
- [ ] Convert widths and summaries at the adapter boundary; retain precise local
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

Open decisions: explicit-height semantics/API, core versus adapter-owned retention,
baseline convention, conversion/precision, and geometry mirror ownership.

When a proposal is accepted, update the core contract and pass signatures, record
frozen decisions in the execution roadmap, and link actionable checks to the
current checklist. Keep this file focused on integration rationale and milestones;
do not maintain a competing specification in tytui.

For implementation work, add owning-package regressions and affected integration
tests. Run `go test ./...`; use build/vet/race checks where changes to integration
or shared result ownership justify them. Mark milestones complete only when their
boundary is exercised, not merely because an isolated package works.
