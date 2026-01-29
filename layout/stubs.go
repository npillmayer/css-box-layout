package layout

import "errors"

type NodeID uint64
type BoxID uint64

type BuildOptions struct{}

type LayoutOptions struct{}

var errNotImplemented = errors.New("not implemented")
