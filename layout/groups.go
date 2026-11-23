package layout

import "iter"

// InlineGroup describes one normalized horizontal group and its structural
// relationships. Group is a BoxAnonymousInline; Owner is its nearest block
// container, including anonymous blocks and inline-blocks. Atomics lists the
// group's immediate BoxInlineBlock boundaries in inline order, including
// replaced leaves, but never their interiors. ParentAtomic is the nearest
// enclosing inline-block within the enumerated tree, or nil.
//
// All node pointers borrow the input tree. Nodes, styles and their child slices
// must remain read-only while any descriptor is used; Go does not enforce this.
// Atomics itself is a fresh caller-owned slice and can be retained or changed
// without affecting the tree, later descriptors or another enumeration.
// BoxIDs retain their tree-local scope; descriptors do not establish a global
// identity across independent trees.
type InlineGroup struct {
	Group, Owner *LayoutNode
	Atomics      []*LayoutNode
	ParentAtomic *LayoutNode
}

// InlineGroups enumerates actual BoxAnonymousInline groups in structural
// preorder, including empty groups and groups inside nested inline-blocks.
// Ordinary inline nodes are not separate groups. An outer group is yielded
// before entering its atomic interiors; a replaced leaf has no interior group.
//
// root must be a normalized block-container tree (or subtree), such as a result
// of BuildLayoutTree. Nil yields nothing. Ancestors outside root are not part of
// the enumeration; an inline-block root encloses its own interior groups.
// This is a traversal, not a validator for malformed trees or cycles.
//
// The iterator borrows root and is repeatable, including after early termination.
// Independent enumerations may run concurrently while the tree remains read-only.
// Stopping iteration stops traversal. No normalization, text processing, style
// resolution, font service, measurement or geometry operation is performed.
func InlineGroups(root *LayoutNode) iter.Seq[InlineGroup] {
	return func(yield func(InlineGroup) bool) {
		var walk func(n, owner, atomic *LayoutNode) bool
		walk = func(n, owner, atomic *LayoutNode) bool {
			if n == nil {
				return true
			}
			if IsBlockContainer(n.Box) {
				owner = n
			}
			if n.Box == BoxInlineBlock {
				atomic = n
			}
			if n.Box == BoxAnonymousInline {
				g := InlineGroup{Group: n, Owner: owner, ParentAtomic: atomic}
				collectGroupAtomics(n, &g.Atomics)
				if !yield(g) {
					return false
				}
			}
			for _, child := range n.Children {
				if !walk(child, owner, atomic) {
					return false
				}
			}
			return true
		}
		walk(root, nil, nil)
	}
}

// Scan only this group's inline content. Atomic descendants are visited later
// by the main traversal, not flattened into the containing group's descriptor.
func collectGroupAtomics(n *LayoutNode, atoms *[]*LayoutNode) {
	if n == nil {
		return
	}
	if n.Box == BoxInlineBlock {
		*atoms = append(*atoms, n)
		return
	}
	for _, child := range n.Children {
		collectGroupAtomics(child, atoms)
	}
}
