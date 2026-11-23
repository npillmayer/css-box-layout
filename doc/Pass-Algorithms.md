# Pass Algorithms and Test Outlines

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


These are target-behavior sketches, not executable code or claims of completed
implementation. Current API signatures are in `doc/Pass-Signatures.md`;
`doc/CSS-Algo-Overview.md` is authoritative. Detailed open work and acceptance
criteria are in `doc/Current-Checklist.md`.

## Pass 1: BuildLayoutTree

The entry point validates typed display and the supplied root. It never applies
implicit defaults or viewport blockification. Block flow-root stays a normal
outer block and records independent interior containment in EstablishesBFC.

```text
BuildLayoutTree(root):
  if root is nil: return nil
  require root is an element
  validate typed display; reject legacy string input
  if display(root) == none: return nil
  require non-replaced block flow or block flow-root root
  return buildBlockContainer(root, BoxBlock)

buildBlockContainer(node, kind):
  flows = concatenate(buildFlow(child) for child in node.Children())
  establishesBFC = (display(node).inner == flow-root)
  return box(kind, sourceIdentity(node), computedStyle(node),
             normalizeBlockChildren(flows), establishesBFC)

buildFlow(node):
  if node is nil: return []
  require element or text; validate typed display (zero for text)
  reject legacy string input
  if display(node) == none: return []
  if node is text:
    if textRange(node) is empty: return []
    return [FlowInline(textBox(node))]
  if node is replaced: validate fixed dimensions/inline outer; return atomic leaf
  if display(node) == inline flow-root:
    return [FlowInline(buildBlockContainer(node, BoxInlineBlock))]
  if display(node) == block flow or block flow-root:
    return [FlowBlock(buildBlockContainer(node, BoxBlock))]
  if display(node) == inline flow:
    flows = concatenate(buildFlow(child) for child in node.Children())
    return wrapInlineRunsForElement(node, flows)
  return unsupported-display error

normalizeBlockChildren(flows):
  if flows is empty: return [BoxAnonymousInline([])]
  if all items are FlowBlock: return their nodes
  if all items are FlowInline: return [BoxAnonymousInline(their nodes)]
  wrap each maximal inline run as:
    BoxAnonymousBlock(BoxAnonymousInline(copy of run))
  retain block items between runs in order

wrapInlineRunsForElement(node, flows):
  wrap each non-empty inline run in a BoxInline fragment
  preserve source NodeID and style; assign unique BoxID per fragment
  keep FlowBlock items between fragments unchanged
  if flows is empty: return one empty BoxInline for node
```

Each box, including anonymous wrappers, needs a unique deterministic BoxID.
Construction must preserve separate child lists for separate runs. Style
conversion, anonymous style rules, and external text-source plumbing are not
specified by the shorthand helpers. Atomic inline-block internals are excluded
from the outer B3 purity traversal and validated as their own block containers.

| Test | Expected result |
|---|---|
| Block-only children | Block children retained in order |
| Inline-only children | One anonymous inline root |
| Empty container / dropped empty text | Empty anonymous inline root |
| Multiple inline runs separated by blocks | Distinct wrappers and child lists |
| Nested inline containing block | Split fragments and hoisted block flow |
| Inline-block with internal blocks | Atomic outer inline; normalized inner container |
| Text leaf | Non-empty range, no children |
| Nil / display:none root | No tree, no error |
| Inline / replaced / text / non-element root | Explicit root error, no tree |
| Missing / malformed / legacy string display | Input error, no partial tree |
| Block flow-root, root or descendant | Normal outer block with independent interior containment |
| Suppressed subtree with invalid contents | Contents skipped before validation |

FlowLayout uses EstablishesBFC as a child-edge and through-collapse boundary,
alongside supplied-root and inline-block containment. Its margin summary retains
only the container's own leading/trailing margins. Parent/sibling placement
continues to merge those outside margins normally for a block-level flow-root.
Internal sibling collapse and ordinary parent/child collapse below the boundary
remain active; empty flow-roots separate leading and trailing groups.
| Styles and identity | Preserved source style/NodeID, unique BoxIDs |

## Pass 2: ResolveUsedValues

```text
ResolveUsedValues(root, ctx):
  table = fresh UsedValuesTable
  walk(root, ctx, table)
  return table

walk(node, ctx, table):
  resolve node edges and known width using explicit style/context
  preserve distinction between auto width and explicit zero
  record result by BoxID
  for each child:
    resolve using the containing block established by this node,
    or inherited containing block for structural inline nodes
```

Lengths use px, fractional percentages (0..1), em, or auto. A fixed-width
inline-block establishes its own content width for descendants. Auto-width
inline-blocks need a final width from intrinsic sizing before descendant
percentages, auto widths and horizontal block margins can be finalized. Pending
flags distinguish provisional values. Flow privately re-resolves descendants
against the selected containing width before final callbacks.

Horizontal auto margins distribute positive remaining width on normal/anonymous
blocks after padding and borders. Auto width keeps auto margins zero; overflow
and all-fixed constraints solve the right margin for the LTR contract. Inline-block
and vertical auto margins stay zero. Intrinsic measurement uses undistributed
edges, so centering space does not enter intrinsic widths. See the core contract.

| Test | Expected result |
|---|---|
| Px / percentage / em edges | Values use the documented width/font context |
| Auto block width | Available content width after edges |
| Fixed and percentage widths | Correct containing block basis |
| Auto margins | One/two-sided distribution, LTR overflow, zero inline-block margins, late resolution |
| Fixed-width inline-block descendants | Use inline-block content width |
| Auto versus zero inline-block width | Distinct states; correct later sizing |
| Build -> resolve | Render styles reach used values |

## Auto-width sizing before final flow

ResolveUsedValues marks unresolved widths/edges as pending rather than final zero.
FlowLayout copies the table before this phase and validates the supported subset:

```text
for each container in dependency order:
  if auto inline-block:
    measure its content bounds (recursive core traversal)
      fixed child: equal content bounds plus its edges
      auto child: measure its groups or combine block contributions
      inline group: external MeasureInline with intrinsic atomic border bounds
      reject percentage dependencies needing an unknown width
    available = max(0, containing width - own horizontal margins/edges)
    content width = min(max(min-content, available), max-content)
    re-resolve descendants against chosen width in the private table
  visit descendants, resolving further auto boundaries
validate that no pending values reach final layout
```

Measurement publishes no paragraphs, placements or geometry. Invocation-local
content bounds may be reused, but no speculative layout or global cache is needed.
Only after this phase succeeds does the final flow below run. See current intrinsic
interfaces in Pass-Signatures.md and acceptance coverage in checklist 2B/3B.

## Pass 3: FlowLayout

The implementation and sketch construct fresh output tables without writes to
input nodes. Results share the immutable structural root; there are no geometry
mirrors on nodes. Callbacks must honor the same read-only input contract.
Implemented fixed-width scheduling: block containers either stack block children
recursively or realize their inline atomics before external delegation:

```text
layoutAtomicInteriors(group):
  realized = fresh group-local collection
  walk ordinary inline descendants in order:
    at atomic boundary A:
      resolve descendants against A's fixed content width
      layoutContainer(A)  // recursively lays out inner groups; stacks block children
      realized[A.BoxID] = realized border-box item
      do not traverse A's descendants again as members of this group
  return readOnly(realized)

before calling inline.LayoutInline for a group:
  realized = layoutAtomicInteriors(group)  // failure prevents the external call
  result = inline.LayoutInline({Group: group, ..., Atomics: realized})
  validate extent and exactly one finite placement per realized item
  for p in result.Atomics:
    geom[p.BoxID].RelativeTo = groupOwnerID
    set Frame origin to (p.X, p.Y); set Content origin plus own border/padding
    preserve dimensions and descendant-local geometry
  content.H = result.Extent.H
```

Atomics and RelativeTo are implemented fields. Use zero
RelativeTo for the root and the containing block for stacked children. Adapter
lookup uses the atomic branch's BoxID; core validates placement IDs against the
same group-local view. Coordinates are border origins after margin/alignment;
core adds no further margin and rejects the entire result before application if
validation fails. Each atomic box is laid out
once per invocation initially; external inner calls finish before the outer call
starts. Adapters translate realized leaves without initiating layout. No explicit
TODO stack is required. Input trees/tables remain immutable. Auto sizing uses
the measurement phase above; speculative trials remain deferred.

Block stacking now returns immutable margin summaries as well as completed
geometry. In source order, merge a child's leading strut into the pending group;
through children keep the group open and do not advance the cursor. A separating
child consumes the pending value unless it escapes through the parent's top,
then starts a new pending group from its trailing strut. Through-box origins use
the hypothetical-bottom-border leading group (or zero when adjoining parent top),
not the eventual group's value. At the end, include only a contained trailing
group in content height, clamp height to zero, and reject coordinate overflow.
Only child Frame/Content origins move; descendants remain parent-content-relative.
See the core contract for eligibility and the inline empty-output permission.

Returned sizes and geometry must agree. The concrete realized-item/placement
proposal is in Pass-Signatures.md; P's whatsit representation remains adapter-owned.
Realized dimensions are not final positions. Geometry coverage for inline
nodes/text also remains open. See checklist 2A/3A.

| Test | Expected result |
|---|---|
| Block stacking | Signed adjoining groups collapse; escaped groups do not inflate parent height |
| Nested padded/bordered blocks | Parent-content-relative coordinates |
| Inline-only block | Content height equals returned overall extent, including trailing space |
| Anonymous block owner | Extent applied; adapter retains typed result by group and owner IDs |
| Atomic fixed / auto / zero width | Correct internal width, height, border-box return |
| Dependency scheduling (target) | Inner groups finish first; complete realized items; no reentrant outer adapter call |
| Geometry consistency | Frame equals Content plus edges; table agrees with sizing |
| Repeated layout | Input immutability and stable identities |

Geometry entries for structural inline nodes, text, and inline roots remain an
explicit coverage decision. Do not claim full-tree geometry from these sketches.
