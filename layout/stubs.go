package layout

import "github.com/npillmayer/css-box-layout/text"

type NodeID uint64

// BoxID identifies a box within one construction call (including its fixed
// roots). Independent builds may reuse IDs; callers must qualify retained box
// references by tree/build identity. Source correspondence uses NodeID/TextRef.
type BoxID uint64

// BuildOptions supplies read-only access to text external to the render tree.
type BuildOptions struct {
	// ReadText returns exactly the UTF-8 bytes referenced by ref. When nil,
	// HTML.Data must contain exactly that referenced range (not the whole source).
	// Errors, invalid UTF-8 and incorrect lengths fail the build without a tree.
	ReadText func(ref text.TextRef) (string, error)
}

type LayoutOptions struct{}
