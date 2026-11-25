package layout

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
)

var (
	ErrInvalidFit     = errors.New("invalid structural fit request")
	ErrUnsupportedFit = errors.New("unsupported structural fitting")
	ErrFitProtocol    = errors.New("paragraph fitting protocol violation")
	ErrFitLimit       = errors.New("structural fitting work limit")
)

// FitPayload clones opaque, backend-owned data. Clone must return an independent
// snapshot, or the same object only if it is immutable. It must not retain
// borrowed trial scratch. Core clones on receipt, inspection and resumption.
// Checkpoints and frozen output are separate payloads; neither is a font API.
// Payloads may retain immutable render resources, but must not capture source
// nodes or execution services.
type FitPayload interface{ Clone() FitPayload }

// ParagraphPosition is a backend-defined, width-independent logical coordinate.
// Offset and Step compare lexicographically; Step distinguishes logical states
// at one offset (such as discretionary carry). They are not line/height counts
// and cannot be used to reconstruct a checkpoint.
type ParagraphPosition struct{ Offset, Step uint64 }

func (p ParagraphPosition) Compare(q ParagraphPosition) int {
	if c := cmp.Compare(p.Offset, q.Offset); c != 0 {
		return c
	}
	return cmp.Compare(p.Step, q.Step)
}

type ParagraphFitRequest struct {
	Root, Group, Owner BoxID
	Width, Height      float32
	From               ParagraphPosition
	Checkpoint         FitPayload
}

// ParagraphAlternative associates a legal prefix with exactly its checkpoint
// and output. Complete choices have no checkpoint; partial choices require one.
// Extents are CSS pixels, relative to the offered receiving rectangle.
type ParagraphAlternative struct {
	End                ParagraphPosition
	Complete           bool
	Extent             InlineExtent
	Checkpoint, Output FitPayload
}

// FitEnumeration distinguishes a complete enumeration from an interrupted one.
// FitFailed always accompanies an error; FitStopped does not imply exhaustion.
type FitEnumeration uint8

const (
	FitExhausted FitEnumeration = iota + 1
	FitStopped
	FitFailed
)

// ParagraphFitter enumerates legal whole-line prefixes, including shorter
// choices. Complete consumes the entire remaining group; partial choices need a
// strictly advancing End and checkpoint. Empty groups may complete at From.
// Output must be frozen/cloneable. A false yield must stop immediately and return
// FitStopped. Returning FitStopped voluntarily is interruption, not exhaustion.
// Call yield synchronously and serially, only during EnumerateParagraph.
type ParagraphFitter interface {
	EnumerateParagraph(context.Context, ParagraphFitRequest, func(ParagraphAlternative) bool) (FitEnumeration, error)
}
type FitError struct {
	Box BoxID
	Err error
}

func (e *FitError) Error() string { return fmt.Sprintf("structural fit box %d: %v", e.Box, e.Err) }
func (e *FitError) Unwrap() error { return e.Err }
func fitError(box BoxID, kind error, message string) error {
	return &FitError{box, fmt.Errorf("%w: %s", kind, message)}
}

type fitOpKind uint8

const (
	fitEnter fitOpKind = iota
	fitGroup
	fitExit
)

type fitOp struct {
	kind       fitOpKind
	box, owner BoxID
}
type fitEdge struct {
	box  BoxID
	edge BoundaryEdge
}
type fitCut struct {
	index    int
	boundary StructuralBoundary
}
type fitProgram struct {
	root BoxID
	ops  []fitOp
	cuts map[fitEdge]fitCut
	all  map[fitEdge]StructuralBoundary
}

// StructuralFitter binds one immutable structural snapshot to one paragraph
// service. Concurrent calls are safe only if that service supports concurrency;
// all core traversal scratch is local to Enumerate.
type StructuralFitter struct {
	program    *fitProgram
	paragraphs ParagraphFitter
}

// NewStructuralFitter snapshots a normalized block structure and binds one
// paragraph service. Initial profile: auto-width, zero-edge static blocks and
// ordinary inline/text groups. Used CSS widths/edges, collapsing margins and
// inline-atomic geometry are explicit unsupported cases until BOX-SHIP-03.
// The source tree is never mutated or retained by fragments/checkpoints.
func NewStructuralFitter(root *LayoutNode, paragraphs ParagraphFitter) (*StructuralFitter, error) {
	if root == nil || !IsBlockLevel(root.Box) || nilFitValue(paragraphs) {
		return nil, fitError(0, ErrInvalidFit, "block root and paragraph service required")
	}
	p := &fitProgram{root: root.BoxID, cuts: map[fitEdge]fitCut{}, all: map[fitEdge]StructuralBoundary{}}
	seen := map[BoxID]bool{}
	var order uint64
	var walk func(*LayoutNode, BoxID, BoxID) error
	walk = func(n *LayoutNode, owner, group BoxID) error {
		if n == nil || n.BoxID == 0 || seen[n.BoxID] {
			return fitError(0, ErrInvalidFit, "nil, duplicate or zero box identity")
		}
		seen[n.BoxID] = true
		style := styleOrDefault(n.Style)
		if err := validateComputedStyle(style); err != nil {
			return &FitError{n.BoxID, fmt.Errorf("%w: %w", ErrInvalidFit, err)}
		}
		if style.Position != PositionStatic || n.Replaced != nil || n.Box == BoxInlineBlock {
			return fitError(n.BoxID, ErrUnsupportedFit, "positioned/replaced/atomic material")
		}
		for _, edges := range []EdgeLengths{style.Margin, style.Padding, style.Border} {
			for _, edge := range []Length{edges.Top, edges.Right, edges.Bottom, edges.Left} {
				if edge.Kind != LenPx || edge.Value != 0 {
					return fitError(n.BoxID, ErrUnsupportedFit, "nonzero or unresolved CSS edges")
				}
			}
		}
		block := IsBlockLevel(n.Box)
		if block && style.Width.Kind != LenAuto {
			return fitError(n.BoxID, ErrUnsupportedFit, "CSS used-width resolution")
		}
		if block {
			owner = n.BoxID
		}
		if n.Box == BoxAnonymousInline {
			group = n.BoxID
		}
		order++
		before := StructuralBoundary{Root: p.root, Box: n.BoxID, Group: group, Edge: BoundaryBefore, Order: order}
		p.all[fitEdge{n.BoxID, BoundaryBefore}] = before
		if block {
			p.cuts[fitEdge{n.BoxID, BoundaryBefore}] = fitCut{len(p.ops), before}
			p.ops = append(p.ops, fitOp{fitEnter, n.BoxID, owner})
			if len(n.Children) == 0 {
				return fitError(n.BoxID, ErrInvalidFit, "normalized block needs children or an empty group")
			}
			inline := len(n.Children) == 1 && n.Children[0] != nil && n.Children[0].Box == BoxAnonymousInline
			for _, c := range n.Children {
				if c == nil || !inline && !IsBlockLevel(c.Box) {
					return fitError(n.BoxID, ErrInvalidFit, "mixed or unnormalized block children")
				}
				if err := walk(c, owner, group); err != nil {
					return err
				}
			}
			p.ops = append(p.ops, fitOp{fitExit, n.BoxID, owner})
		} else if n.Box == BoxAnonymousInline {
			p.cuts[fitEdge{n.BoxID, BoundaryBefore}] = fitCut{len(p.ops), before}
			p.ops = append(p.ops, fitOp{fitGroup, n.BoxID, owner})
			for _, c := range n.Children {
				if c == nil || (c.Box != BoxInline && c.Box != BoxText && c.Box != BoxInlineBlock) {
					return fitError(n.BoxID, ErrInvalidFit, "invalid inline group child")
				}
				if err := walk(c, owner, n.BoxID); err != nil {
					return err
				}
			}
		} else if n.Box == BoxInline {
			for _, c := range n.Children {
				if c == nil || (c.Box != BoxInline && c.Box != BoxText && c.Box != BoxInlineBlock) {
					return fitError(n.BoxID, ErrInvalidFit, "unnormalized inline children")
				}
				if err := walk(c, owner, group); err != nil {
					return err
				}
			}
		} else if n.Box == BoxText {
			if len(n.Children) != 0 || n.Text.Range.End < n.Text.Range.Start {
				return fitError(n.BoxID, ErrInvalidFit, "invalid text leaf")
			}
		} else {
			return fitError(n.BoxID, ErrUnsupportedFit, "box kind")
		}
		order++
		after := StructuralBoundary{Root: p.root, Box: n.BoxID, Group: group, Edge: BoundaryAfter, Order: order}
		p.all[fitEdge{n.BoxID, BoundaryAfter}] = after
		if block || n.Box == BoxAnonymousInline {
			p.cuts[fitEdge{n.BoxID, BoundaryAfter}] = fitCut{len(p.ops), after}
		}
		return nil
	}
	if err := walk(root, 0, 0); err != nil {
		return nil, err
	}
	// A trailing group/block carries the closing ancestors with it. A cut cannot
	// defer their closing facts into a later edge-only fragment.
	for key, cut := range p.cuts {
		if key.edge == BoundaryAfter {
			for cut.index < len(p.ops) && p.ops[cut.index].kind == fitExit {
				cut.index++
			}
			p.cuts[key] = cut
		}
	}
	return &StructuralFitter{p, paragraphs}, nil
}

// FitBoundary is qualified by this fitter, not just a numeric BoxID. The zero
// value means no explicit stop. After-cuts include any closing ancestors whose
// last child was consumed; they cannot defer an edge-only closing fragment.
type FitBoundary struct {
	program *fitProgram
	cut     fitCut
}

func (b FitBoundary) Info() StructuralBoundary { return b.cut.boundary }
func (f *StructuralFitter) Boundary(box BoxID, edge BoundaryEdge) (FitBoundary, error) {
	if f == nil || f.program == nil {
		return FitBoundary{}, fitError(box, ErrInvalidFit, "invalid fitter")
	}
	if cut, ok := f.program.cuts[fitEdge{box, edge}]; ok {
		return FitBoundary{f.program, cut}, nil
	}
	if _, ok := f.program.all[fitEdge{box, edge}]; ok {
		return FitBoundary{}, fitError(box, ErrUnsupportedFit, "cut inside inline material")
	}
	return FitBoundary{}, fitError(box, ErrInvalidFit, "unknown boundary")
}

// OpenContainer describes a suspended ancestor. NextChild is its normalized
// child index. Pending flags preserve unconsumed opening/closing edge and margin
// obligations; the current fitting profile admits only zero-valued edges.
type OpenContainer struct {
	Box                            BoxID
	NextChild                      int
	OpeningPending, ClosingPending bool
}
type fitCursor struct {
	index      int
	point      ParagraphPosition
	checkpoint FitPayload
	open       []OpenContainer
}

// FitContinuation is opaque resumption authority for exactly one fitter. Zero
// starts at the beginning. It retains neither source nodes nor the paragraph
// service; its checkpoint payload obeys FitPayload's snapshot contract.
type FitContinuation struct {
	program *fitProgram
	cursor  fitCursor
}
type FitCursorInfo struct {
	Next      BoxID
	Paragraph ParagraphPosition
	Open      []OpenContainer
	Complete  bool
}

func (c FitContinuation) Valid() bool { return c.program != nil }
func (c FitContinuation) Info() FitCursorInfo {
	if !c.Valid() {
		return FitCursorInfo{}
	}
	out := FitCursorInfo{Paragraph: c.cursor.point, Open: slices.Clone(c.cursor.open), Complete: c.cursor.index == len(c.program.ops)}
	if !out.Complete {
		out.Next = c.program.ops[c.cursor.index].box
	}
	return out
}

// FitPosition compares material progression within one fitter, independently of
// receiving width, line count or occupied extent. It cannot resume a fit.
type FitPosition struct {
	program   *fitProgram
	index     int
	paragraph ParagraphPosition
}
type FitPositionInfo struct {
	Next      BoxID
	Paragraph ParagraphPosition
	Complete  bool
}

func (p FitPosition) Info() FitPositionInfo {
	if p.program == nil {
		return FitPositionInfo{}
	}
	info := FitPositionInfo{Paragraph: p.paragraph, Complete: p.index == len(p.program.ops)}
	if !info.Complete {
		info.Next = p.program.ops[p.index].box
	}
	return info
}
func (p FitPosition) Compare(q FitPosition) (int, bool) {
	if p.program == nil || p.program != q.program {
		return 0, false
	}
	if c := cmp.Compare(p.index, q.index); c != 0 {
		return c, true
	}
	return p.paragraph.Compare(q.paragraph), true
}

type FitRequest struct {
	Width, Height float32
	Continuation  FitContinuation
	Stop          FitBoundary
	IncludeEmpty  bool
	// Zero means no local cap. Each structural step and paragraph alternative
	// counts; caller cancellation is checked between steps and after callbacks.
	MaxSteps uint64
}

// ParagraphFragment has its origin at (0,Y) in the receiving rectangle. Output
// belongs to precisely the prefix From..To. Paragraphs returns payload clones.
type ParagraphFragment struct {
	Group, Owner BoxID
	From, To     ParagraphPosition
	Complete     bool
	Y            float32
	Extent       InlineExtent
	Output       FitPayload
}

// ContainerFragment records whether a consumed container has its opening and
// closing in this fragment. It carries no used geometry or paint yet.
type ContainerFragment struct {
	Box         BoxID
	First, Last bool
}
type fitResult struct {
	next           FitContinuation
	fragments      []ParagraphFragment
	containers     []ContainerFragment
	extent         InlineExtent
	progress, stop bool
}

// FitAlternative is an immutable structural fragment and its exact remainder.
// A complete alternative may advance an empty group at zero occupied height;
// a non-progress alternative never consumes material. The zero value is empty.
type FitAlternative struct{ result *fitResult }

func (a FitAlternative) Progress() bool    { return a.result != nil && a.result.progress }
func (a FitAlternative) Complete() bool    { return a.result != nil && a.result.next.Info().Complete }
func (a FitAlternative) StopReached() bool { return a.result != nil && a.result.stop }
func (a FitAlternative) Extent() InlineExtent {
	if a.result == nil {
		return InlineExtent{}
	}
	return a.result.extent
}
func (a FitAlternative) End() FitPosition {
	if a.result == nil {
		return FitPosition{}
	}
	c := a.result.next
	return FitPosition{c.program, c.cursor.index, c.cursor.point}
}
func (a FitAlternative) Continuation() FitContinuation {
	if a.result == nil {
		return FitContinuation{}
	}
	return a.result.next
}
func (a FitAlternative) Containers() []ContainerFragment {
	if a.result == nil {
		return nil
	}
	return slices.Clone(a.result.containers)
}
func (a FitAlternative) Paragraphs() []ParagraphFragment {
	if a.result == nil {
		return nil
	}
	out := slices.Clone(a.result.fragments)
	for i := range out {
		out[i].Output = out[i].Output.Clone()
	}
	return out
}
func nilFitValue(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface, reflect.Func, reflect.Chan:
		return r.IsNil()
	}
	return false
}
func cloneFitPayload(v FitPayload) (FitPayload, error) {
	if nilFitValue(v) {
		return nil, nil
	}
	copy := v.Clone()
	if nilFitValue(copy) {
		return nil, fitError(0, ErrFitProtocol, "payload Clone returned nil")
	}
	return copy, nil
}
