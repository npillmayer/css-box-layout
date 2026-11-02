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
- Define conversion from string style queries to `layout.ComputedStyle`,
  including defaults and anonymous styles. BuildLayoutTree currently reads
  display but does not populate LayoutNode.Style.
- Reconcile this proposal with the concrete signatures in
  `doc/Pass-Signatures.md` before adopting it. External adapter integration stays
  deferred; style transport through the current stub is checklist task 1.
