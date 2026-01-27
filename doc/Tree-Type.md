# Defining the Types and Interfaces for Tree Traversal

### Current State of CSSDOM Types

- package in the midst of refactoring
- to be updated anyway

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

Core passes should depend on a narrow, read-only interface rather than the concrete tree implementation. The underlying tree remains `tree.Node[T]` and can be swapped later without changing pass logic.

```go
// StyNodeView is the minimal interface needed by core passes.
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
- Parent access exists on `tree.Node[T]` but is not required by Pass 1.
