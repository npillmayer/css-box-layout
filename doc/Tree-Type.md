# Defining the Types and Interfaces for Tree Traversal

### Historical CSSDOM Sketch

The following types describe an earlier external CSSDOM design, not verified
current external code. This repository currently builds from the concrete
`layout.RenderNode` stub in `layout/render.go`.

```go
// StyNode is a style node, the building block of the styled tree.
type StyNode struct {
	tree.Node[*StyNode] // we build on top of general purpose tree
	htmlNode            *html.Node
	computedStyles      *style.PropertyMap // do not use
}
```

```go
// Node is the base type our tree is built of.
type Node[T comparable] struct {
	parent   *Node[T]         // parent node of this node
	children childrenSlice[T] // mutex-protected slice of children nodes
	Payload  T                // nodes may carry a payload of arbitrary type
	Rank     uint32           // rank is used for preserving sequence
}
```

---

## Proposed Tree Interface (Go)

Core passes should depend on a narrow, read-only interface rather than the concrete tree implementation. An adapter may wrap `tree.Node[T]` or another representation; the core should
not require a particular external tree implementation.

```go
// StyNodeView is the existing proposal, not a complete frozen adapter contract.
// Order is defined by Children() slice order (Rank must remain in sync).
// Aggregate styles ("margin") have to be broken up into their
// basic styles ("margin-left", "margin-top", etc.)
type StyNodeView interface {
	Children() []StyNodeView
	HTMLNode() *html.Node
	ComputedStyle(string) string // query a basic style
}
```

Notes:
- Traversal uses `Children()` order; `Rank` must stay consistent with that order.
- Access to `html.Node` is allowed.
- Computed styles must be accessed via `ComputedStyle(…)` (no direct field access).
- Parent access exists on the historical `tree.Node[T]` sketch but is not required by Pass 1.

## Remaining adapter work

- `StyNodeView` is declared in `layout/interfaces.go` but is not consumed by
  BuildLayoutTree. `RenderNode.Children()` returns `[]*RenderNode`, so RenderNode
  does not implement this interface as written.
- Specify stable source NodeID access and text-source/range access. The proposed
  interface currently supplies neither explicitly.
- External adapters must convert CSS values to the typed RenderNode.Style
  subset described below; the core does not parse CSS length strings.
- Reconcile this proposal with the concrete signatures in
  `doc/Pass-Signatures.md` before adopting it. External adapter integration stays
  deferred; typed style/source transport through the current stub is implemented.

## Current RenderNode input contract

BuildLayoutTree still accepts the local RenderNode projection. It now carries:

- `ID NodeID`: source identity supplied by the adapter; split fragments retain it.
- `Style *ComputedStyle`: copied typed box values; nil means default auto width
  and zero edges. Start with `layout.DefaultComputedStyle()` when overriding only
  some properties. A non-nil zero-valued style specifies width 0px, not auto.
- `Text *text.TextRef`: a text node's authoritative source ID and byte range,
  copied by value into its box. Nonempty HTML text without a reference is an
  error. Supplied empty ranges are dropped; reversed ranges are rejected.
  Source-backed text may have empty HTML.Data, and explicit source ID 0 is valid.
- `Styles["display"]`: block, inline, inline-block, or none; absent child display
  defaults to inline. The current entry point assumes a block root. Known box
  length/font-size properties supplied as strings are rejected with a request
  to use Style, rather than silently ignored. Other metadata is not copied into
  layout styles; use NodeID to join immutable external typography/paint data.

Supported typed values are Width and per-side Margin/Padding/Border using existing
Length kinds, plus FontSizePx. Lengths must be finite; width/padding/border cannot
be negative. Auto is allowed for width/margins, not padding/border. Negative
margins are allowed. Percent values use fractions; em resolves against the node's
own computed font size. The adapter is responsible for CSS cascade, shorthand
expansion, border style effects, and conversion into this subset; accepting a
Length kind does not establish full CSS conformance.

FontSizePx is either positive or zero (inherit/fallback sentinel in this minimal
API). Explicit CSS font-size:0 is not representable yet. Source-parent inheritance
is captured before split/hoist; unresolved root font size uses ResolveContext's
FontSizePx. Anonymous wrappers have auto width, zero edges, and inherited font
size; source widths and edges are not inherited. Copied styles are treated as
immutable, including styles shared by split fragments.

The adapter owns source storage, source-ID uniqueness within its snapshot, and
source/range validation against actual bytes (including UTF-8 boundaries). The
builder checks range ordering but cannot check source bounds without that storage.
It never derives source IDs by truncating NodeID. Source lookup, richer typography
and paint, and selection/caret mapping remain external integration work.
