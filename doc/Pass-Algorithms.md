# Pass Algorithms and Test Outlines

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

## Pass 3: FlowLayout

The sketch constructs fresh output tables. It does not authorize writes to
input nodes. Any optional geometry mirrors require an ownership/copying design.

```text
FlowLayout(root, used, inline, intrinsic, ctx):
  geom = fresh geometry table
  lines = fresh line table
  layoutContainer(root, frameOrigin=(0,0))
  return {Root: root, Geometry: geom, Lines: lines}

layoutContainer(node, frameOrigin):
  u = required used values for node
  frame = border box at frameOrigin with width contentWidth + horizontal edges
  content = box at frameOrigin + top/left border and padding
  content.W = u.ContentWidth

  if node has one BoxAnonymousInline child:
    lineBoxes = inline.LayoutInline(child, content.W, atomicSizer)
    if node is BoxBlock or BoxInlineBlock:
      lines[node.BoxID] = lineBoxes
    content.H = max(0, all line.Y + line.H)
  else:
    y = 0
    for child in node.Children:
      y += used[child].Margin.Top
      layoutContainer(child, (used[child].Margin.Left, y))
      y += geom[child].Frame.H + used[child].Margin.Bottom
    content.H = y

  frame.H = content.H + vertical padding and border
  geom[node.BoxID] = {frame, content}
```

Child origins are relative to the parent's content box: do not add the parent's
Content.X/Y to those origins. Each child's own Content origin includes that
child's border/padding offset. Line frames likewise remain relative to their
owner's content box.

Atomic sizing must choose content width (retaining the max-content approximation
for auto), establish correct descendant resolution, and lay out internal content
at that width before returning border-box dimensions. Returned sizes and stored
geometry must agree. Final atomic placement and geometry coverage for inline
nodes/text need an integration design; the block sketch does not supply them.

| Test | Expected result |
|---|---|
| Block stacking | Vertical margins retained without collapsing |
| Nested padded/bordered blocks | Parent-content-relative coordinates |
| Inline-only block | Content height equals line extent |
| Anonymous block owner | No stored line entry, but correct content height |
| Atomic fixed / auto / zero width | Correct internal width, height, border-box return |
| Geometry consistency | Frame equals Content plus edges; table agrees with sizing |
| Repeated layout | Input immutability and stable identities |

Geometry entries for structural inline nodes, text, and inline roots remain an
explicit coverage decision. Do not claim full-tree geometry from these sketches.
