# Pass Algorithms + Test Outlines (Step 4)

This document sketches the algorithms for each pass and outlines normal-scope, table-driven test cases. It is intentionally concise and avoids rare edge cases for now.

Authoritative signatures and package layout: `doc/Pass-Signatures.md`.

---

## Pass 1: BuildLayoutTree (D -> E)

### Algorithm sketch (pseudocode)
```
function BuildLayoutTree(root StyNodeView) -> LayoutNode:
  return buildBlockContainer(root, BoxBlock)

function buildBlockContainer(node StyNodeView, boxKind BoxKind) -> LayoutNode:
  flowItems = buildInlineOrBlockFlow(node)
  children = normalizeBlockChildren(flowItems)
  return LayoutNode{
    BoxId: newBoxId(node),
    NodeId: sourceNodeId(node),
    Kind: boxKind,
    Style: node,
    Children: children,
  }

function buildInlineOrBlockFlow(node StyNodeView) -> []FlowItem:
  display = node.ComputedStyle("display")
  if display == "none": return []

  if isTextNode(node.HTMLNode()):
    text = textRange(node.HTMLNode())
    if text.isEmpty(): return []
    return [FlowInline(BoxText(node, text))]

  kind = boxKindFromDisplay(display)

  if kind is inline:
    flows = []
    for child in node.Children():
      flows += buildInlineOrBlockFlow(child)
    if containsBlockFlow(flows):
      return splitAndHoistInline(node, flows)
    return [FlowInline(BoxInline(node, inlineChildren(flows)))]

  if kind is block:
    flows = []
    for child in node.Children():
      flows += buildInlineOrBlockFlow(child)
    blockChildren = normalizeBlockChildren(flows)
    return [FlowBlock(BoxBlock(node, blockChildren))]

function normalizeBlockChildren(flowItems []FlowItem) -> []*LayoutNode:
  if all blocks:
    return block nodes
  if all inlines:
    return [BoxAnonymousInline(inline nodes)]
  else:
    wrap each maximal inline run into:
      BoxAnonymousBlock( BoxAnonymousInline(run) )
    keep block nodes between runs
    return children

function splitAndHoistInline(node StyNodeView, flows []FlowItem) -> []FlowItem:
  split inline runs into BoxInline fragments (same NodeId, unique BoxId)
  hoist FlowBlock items unchanged
  return concatenated flow items
```

### Test outline (normal scope)

| Case | Input shape | Expected output | Invariants checked |
|---|---|---|---|
| Block-only children | block with block children | children unchanged | B1 block-only |
| Inline-only children | block with inline children | single BoxAnonymousInline child | B1 inline-only, B2 |
| Mixed inline/block | block with inline + block | anonymous blocks for inline runs | B1, B2 |
| Inline contains block | inline node with block descendant | split inline fragments + hoisted block | B3 |
| BoxText leaf | text node | BoxText with no children | B4 |

---

## Pass 2: ResolveUsedValues (E -> used values)

### Algorithm sketch (pseudocode)
```
function ResolveUsedValues(root, ctx):
  table = new UsedValuesTable
  walk(root, ctx.ContainingBlock, table)
  return table

function walk(node, containingBlock, table):
  used = resolveEdges(node.style, containingBlock)
  used.ContentWidth = computeUsedWidth(node.style, containingBlock, used)
  table[node.BoxId] = used
  for child in node.Children:
    walk(child, childContainingBlock(containingBlock, used), table)
```

### Test outline (normal scope)

| Case | Input style | Expected used values | Notes |
|---|---|---|---|
| Fixed px padding | padding:10px | padding resolved to 10 | deterministic |
| Percent padding | padding:10% | resolved vs containing block width | percent resolution |
| Auto margins | margin-left/right:auto | flags or resolved per rule | horizontal behavior |
| Width auto | width:auto | fills remaining width | no margins collapsing |
| Border widths | border:1px | border resolved to 1 | known floats |

---

## Pass 3: FlowLayout (E + used values -> F)

### Algorithm sketch (pseudocode)
```
function FlowLayout(root, used, inline, intrinsic, ctx):
  geom = new LayoutGeometryTable
  lines = new LinesByBlock
  layoutBlockContainer(root, ctx.ContainingBlock, used, geom, lines, inline, intrinsic)
  return LayoutResult{ Root: root, Geometry: geom, Lines: lines }

function layoutBlockContainer(node, cb, used, geom, lines, inline, intrinsic):
  u = used[node.BoxId]
  frame = computeFrame(cb, u)
  content = computeContent(frame, u)
  geom[node.BoxId] = {Frame: frame, Content: content}

  if node is inline-only block container:
    lineBoxes = inline.LayoutInline(node.anonymousInline, content.W, makeAtomicSizer(...))
    if node.BoxId != 0:
      lines[node.BoxId] = lineBoxes
    content.H = extent(lineBoxes)
  else:
    y = 0
    for child in node.Children:
      layoutBlockContainer(child, childCB(content, used[child.BoxId]), ...)
      childGeom = geom[child.BoxId]
      y += used[child.BoxId].Margin.Top
      childGeom.Frame.Y = y
      y += childGeom.Frame.H + used[child.BoxId].Margin.Bottom
    content.H = y

  frame.H = content.H + u.Padding.Top + u.Padding.Bottom + u.Border.Top + u.Border.Bottom
  geom[node.BoxId] = {Frame: frame, Content: content}
```

### Test outline (normal scope)

| Case | Input tree | Expected layout | Notes |
|---|---|---|---|
| Block stacking | parent with 2 block children | y offsets stack with margins | no collapsing |
| Inline-only block | block with BoxAnonymousInline | lines stored + content height | delegate inline layouter |
| Frame/content math | node with padding/border | Frame = Content + edges | epsilon check |
| Coordinate convention | nested blocks | coordinates relative to parent content | origin (0,0) |
