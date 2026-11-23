package layout

import "fmt"

// BuildError locates a failure from BuildLayoutTree or BuildLayoutTrees without
// parsing diagnostic text. RootID identifies the input render root (the branch
// boundary), not a generated BoxID. NodeID is the offending source node when
// HasNode is true. Zero is a valid caller-supplied ID, so HasNode distinguishes
// an identified node from a failure attributable only to the tree as a whole.
// Errors retain no render or layout nodes. Err preserves the underlying cause
// for errors.Is/errors.As; diagnostic prose is not a stable interface.
type BuildError struct {
	RootID  NodeID
	NodeID  NodeID
	HasNode bool
	Err     error
}

func (e *BuildError) Error() string {
	if e.HasNode {
		return fmt.Sprintf("node %d: %v", e.NodeID, e.Err)
	}
	return fmt.Sprintf("tree rooted at node %d: %v", e.RootID, e.Err)
}

func (e *BuildError) Unwrap() error { return e.Err }

func buildError(id NodeID, err error) error {
	return &BuildError{NodeID: id, HasNode: true, Err: err}
}

// Attach the public input boundary without mutating an existing error (including
// an error retained by a caller's ReadText callback). Node wrappers are created
// at the failing operation, so a nested callback error cannot replace its source.
func buildRootError(root NodeID, err error) error {
	if err == nil {
		return nil
	}
	if located, ok := err.(*BuildError); ok {
		copy := *located
		copy.RootID = root
		return &copy
	}
	return &BuildError{RootID: root, Err: err}
}
