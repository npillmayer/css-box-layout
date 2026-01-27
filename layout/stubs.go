package layout

import "errors"

type NodeID uint64

type RenderNode struct{}

type ComputedStyle struct{}

type BuildOptions struct{}

type LayoutOptions struct{}

var errNotImplemented = errors.New("not implemented")
