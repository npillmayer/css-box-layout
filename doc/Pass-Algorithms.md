# Pass Algorithms and Test Outlines

Implementation update: tasks 2A/3A and bounded auto-width tasks 2B/3B are
implemented. Core measures nested intrinsic bounds, selects auto content widths,
resolves descendants privately, then performs final layout. Fixed pixel widths
including zero remain supported; percent/em widths on atomic boxes remain rejected.
Percentage dependencies on unknown auto widths fail explicitly; definite bases
remain supported. See [current API](Pass-Signatures.md#intrinsic-measurement).
Diagrams illustrate the fixed-width final-layout phase, not intrinsic measurement.
Real tytui/P integration, full baseline behavior and speculative trials remain open.


These are target-behavior sketches, not executable code or claims of completed
implementation. Current API signatures are in `doc/Pass-Signatures.md`;
`doc/CSS-Algo-Overview.md` is authoritative. Detailed open work and acceptance
criteria are in `doc/Current-Checklist.md`.

## Pass 1: BuildLayoutTree

The root sketch assumes a block render root. The public entry point currently
forces BoxBlock; general root display handling remains unspecified.

```text
BuildLayoutTree(root):
  if root is nil: return nil
  return buildBlockContainer(root, BoxBlock)

buildBlockContainer(node, kind):
  flows = concatenate(buildFlow(child) for child in node.Children())
  return box(kind, sourceIdentity(node), computedStyle(node),
             normalizeBlockChildren(flows))

buildFlow(node):
  if node is nil or display(node) == none: return []
  if node is text:
    if textRange(node) is empty: return []
    return [FlowInline(textBox(node))]
  if display(node) == inline-block:
    return [FlowInline(buildBlockContainer(node, BoxInlineBlock))]
  if display(node) == block:
    return [FlowBlock(buildBlockContainer(node, BoxBlock))]
  if display(node) == inline:
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
percentages and auto widths can be finalized. The scheduling/representation
for this dependency is still open; this sketch does not prescribe a premature
walk using the outer width as the final descendant context.

Horizontal auto-margin distribution also needs explicit semantics. The current
implementation discards its auto flags; zero is not a statement of the intended
final rule.

| Test | Expected result |
|---|---|
| Px / percentage / em edges | Values use the documented width/font context |
| Auto block width | Available content width after edges |
| Fixed and percentage widths | Correct containing block basis |
| Auto margins | Agreed distribution or explicit documented deferral |
| Fixed-width inline-block descendants | Use inline-block content width |
| Auto versus zero inline-block width | Distinct states; correct later sizing |
| Build -> resolve | Render styles reach used values |

## Auto-width preparation before final flow

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
recursively or prepare their inline group before external delegation:

```text
prepareGroup(group):
  prepared = fresh group-local collection
  walk ordinary inline descendants in order:
    at atomic boundary A:
      resolve descendants against A's fixed content width
      layoutContainer(A)  // recursively prepares inner groups; stacks block children
      prepared[A.BoxID] = measured border-box item
      do not traverse A's descendants again as members of this group
  return readOnly(prepared)

before calling inline.LayoutInline for a group:
  prepared = prepareGroup(group)  // failure prevents the external call
  result = inline.LayoutInline({Group: group, ..., Atomics: prepared})
  validate extent and exactly one finite placement per prepared item
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
validation fails. Each atomic box is prepared
once per invocation initially; external inner calls finish before the outer call
starts. Adapters translate prepared leaves without initiating layout. No explicit
TODO stack is required. Input trees/tables remain immutable. Auto sizing uses
the measurement phase above; speculative trials remain deferred.

Returned sizes and geometry must agree. The concrete prepared-item/placement
proposal is in Pass-Signatures.md; P's whatsit representation remains adapter-owned.
Prepared dimensions are not final positions. Geometry coverage for inline
nodes/text also remains open. See checklist 2A/3A.

| Test | Expected result |
|---|---|
| Block stacking | Vertical margins retained without collapsing |
| Nested padded/bordered blocks | Parent-content-relative coordinates |
| Inline-only block | Content height equals returned overall extent, including trailing space |
| Anonymous block owner | Extent applied; adapter retains typed result by group and owner IDs |
| Atomic fixed / auto / zero width | Correct internal width, height, border-box return |
| Dependency scheduling (target) | Inner groups finish first; complete prepared items; no reentrant outer adapter call |
| Geometry consistency | Frame equals Content plus edges; table agrees with sizing |
| Repeated layout | Input immutability and stable identities |

Geometry entries for structural inline nodes, text, and inline roots remain an
explicit coverage decision. Do not claim full-tree geometry from these sketches.
