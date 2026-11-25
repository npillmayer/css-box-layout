package layout

import (
	"context"
	"fmt"
	"slices"
)

// Enumerate visits fitting alternatives depth first. Within each group it
// explores complete choices before partial choices, then descending End (stable
// for ties); extensions precede their shorter block prefix. IncludeEmpty adds
// the unchanged cursor last. A request
// already at its stop/end yields that fact even without IncludeEmpty.
//
// Exhausted certifies that all alternatives were visited. Stopped denotes a
// caller or paragraph-service interruption. Errors, cancellation and MaxSteps
// return Failed; previously yielded snapshots remain valid, but are not an
// exhaustive answer. Paragraph services must cooperate with context/yield.
func (f *StructuralFitter) Enumerate(ctx context.Context, req FitRequest, yield func(FitAlternative) bool) (FitEnumeration, error) {
	if f == nil || f.program == nil || ctx == nil || yield == nil {
		return FitFailed, fitError(0, ErrInvalidFit, "fitter, context and visitor required")
	}
	p := f.program
	if !finite(req.Width) || !finite(req.Height) || req.Width < 0 || req.Height < 0 {
		return FitFailed, fitError(p.root, ErrInvalidFit, "finite nonnegative receiving dimensions required")
	}
	c := fitCursor{}
	if req.Continuation.Valid() {
		if req.Continuation.program != p {
			return FitFailed, fitError(p.root, ErrInvalidFit, "foreign continuation")
		}
		c = req.Continuation.cursor
		c.open = slices.Clone(c.open)
	}
	stop := len(p.ops)
	if req.Stop.program != nil {
		if req.Stop.program != p {
			return FitFailed, fitError(p.root, ErrInvalidFit, "foreign stop boundary")
		}
		stop = req.Stop.cut.index
	}
	if stop < c.index || stop == c.index && c.point != (ParagraphPosition{}) {
		return FitFailed, fitError(p.root, ErrInvalidFit, "stop precedes continuation")
	}
	r := fitRun{ctx: ctx, fitter: f, req: req, stop: stop, yield: yield, status: FitExhausted}
	r.explore(fitState{cursor: c}, req.IncludeEmpty)
	return r.status, r.err
}

type fitState struct {
	cursor     fitCursor
	paragraphs []ParagraphFragment
	containers []ContainerFragment
	extent     InlineExtent
	progress   bool
}

func (s fitState) fork() fitState {
	s.cursor.open = slices.Clone(s.cursor.open)
	s.paragraphs = slices.Clone(s.paragraphs)
	s.containers = slices.Clone(s.containers)
	return s
}

type fitRun struct {
	ctx    context.Context
	fitter *StructuralFitter
	req    FitRequest
	stop   int
	yield  func(FitAlternative) bool
	steps  uint64
	status FitEnumeration
	err    error
}

func (r *fitRun) fail(box BoxID, err error) {
	if r.status == FitExhausted {
		r.status, r.err = FitFailed, &FitError{box, err}
	}
}

func (r *fitRun) check(box BoxID, step bool) bool {
	if r.status != FitExhausted {
		return false
	}
	if err := r.ctx.Err(); err != nil {
		r.fail(box, err)
		return false
	}
	if step {
		if r.req.MaxSteps > 0 && r.steps >= r.req.MaxSteps {
			r.fail(box, ErrFitLimit)
			return false
		}
		r.steps++
	}
	return true
}

func (r *fitRun) publish(s fitState) {
	if !r.check(r.fitter.program.root, false) {
		return
	}
	// All slices and backend payloads in s are private snapshots. Neither later
	// exploration nor accessors hand out a writable reference to them.
	a := FitAlternative{&fitResult{
		next:      FitContinuation{r.fitter.program, s.cursor},
		fragments: s.paragraphs, containers: s.containers,
		extent: s.extent, progress: s.progress, stop: s.cursor.index == r.stop,
	}}
	if !r.yield(a) {
		r.status = FitStopped
	} else {
		r.check(r.fitter.program.root, false)
	}
}

func (r *fitRun) explore(base fitState, offer bool) {
	p := r.fitter.program
	if !r.check(p.root, false) {
		return
	}
	s := base.fork()
	// Opening a container consumes no material. Speculative entries are not
	// published when the next group fails to fit; an explicit stop may retain
	// them with OpeningPending still set.
	for s.cursor.index < r.stop && p.ops[s.cursor.index].kind == fitEnter {
		op := p.ops[s.cursor.index]
		if !r.check(op.box, true) {
			return
		}
		s.cursor.open = append(s.cursor.open, OpenContainer{Box: op.box, OpeningPending: true, ClosingPending: true})
		s.cursor.index++
	}
	if s.cursor.index == r.stop {
		r.publish(s)
		return
	}
	op := p.ops[s.cursor.index]
	if !r.check(op.box, true) {
		return
	}
	choices := r.paragraphs(s, op)
	if r.status != FitExhausted {
		return
	}
	for _, choice := range choices {
		next := s.fork()
		for i := range next.cursor.open {
			frame := &next.cursor.open[i]
			if slices.IndexFunc(next.containers, func(c ContainerFragment) bool { return c.Box == frame.Box }) < 0 {
				next.containers = append(next.containers, ContainerFragment{Box: frame.Box, First: frame.OpeningPending})
			}
			frame.OpeningPending = false
		}
		next.paragraphs = append(next.paragraphs, ParagraphFragment{
			Group: op.box, Owner: op.owner, From: s.cursor.point, To: choice.End,
			Complete: choice.Complete, Y: s.extent.H, Extent: choice.Extent, Output: choice.Output,
		})
		next.extent = InlineExtent{W: max(s.extent.W, choice.Extent.W), H: s.extent.H + choice.Extent.H}
		next.progress = true // completing an empty group is structural progress
		if !choice.Complete {
			next.cursor.point, next.cursor.checkpoint = choice.End, choice.Checkpoint
			r.publish(next)
		} else {
			next.cursor.index++
			next.cursor.point, next.cursor.checkpoint = ParagraphPosition{}, nil
			next.cursor.open[len(next.cursor.open)-1].NextChild++
			// Closing ancestors belong to the last consumed child, never to a
			// later page containing only closing edges.
			for next.cursor.index < len(p.ops) && p.ops[next.cursor.index].kind == fitExit {
				exit := p.ops[next.cursor.index]
				if !r.check(exit.box, true) {
					return
				}
				if i := slices.IndexFunc(next.containers, func(c ContainerFragment) bool { return c.Box == exit.box }); i >= 0 {
					next.containers[i].Last = true
				}
				next.cursor.open = next.cursor.open[:len(next.cursor.open)-1]
				if len(next.cursor.open) > 0 {
					next.cursor.open[len(next.cursor.open)-1].NextChild++
				}
				next.cursor.index++
			}
			r.explore(next, true)
		}
		if r.status != FitExhausted {
			return
		}
	}
	if offer {
		r.publish(base)
	}
}

func (r *fitRun) paragraphs(s fitState, op fitOp) []ParagraphAlternative {
	checkpoint, err := cloneFitPayload(s.cursor.checkpoint)
	if err != nil {
		r.fail(op.box, err)
		return nil
	}
	req := ParagraphFitRequest{Root: r.fitter.program.root, Group: op.box, Owner: op.owner,
		Width: r.req.Width, Height: r.req.Height - s.extent.H, From: s.cursor.point, Checkpoint: checkpoint}
	var choices []ParagraphAlternative
	refused := false
	status, err := r.fitter.paragraphs.EnumerateParagraph(r.ctx, req, func(a ParagraphAlternative) bool {
		if refused {
			r.fail(op.box, fmt.Errorf("%w: callback continued after false yield", ErrFitProtocol))
			return false
		}
		accept := func() bool {
			if !r.check(op.box, true) {
				return false
			}
			if !finite(a.Extent.W) || !finite(a.Extent.H) || a.Extent.W < 0 || a.Extent.H < 0 ||
				a.End.Compare(req.From) < 0 || (!a.Complete && a.End.Compare(req.From) == 0) ||
				a.Complete != nilFitValue(a.Checkpoint) || nilFitValue(a.Output) {
				r.fail(op.box, fmt.Errorf("%w: invalid extent, progress or payload association", ErrFitProtocol))
				return false
			}
			// An oversized indivisible prefix is ordinary non-fit. No clipping
			// or inferred advancement takes place.
			if a.Extent.W > req.Width || a.Extent.H > req.Height ||
				float64(s.extent.H)+float64(a.Extent.H) > float64(r.req.Height) {
				return true
			}
			var err error
			if a.Checkpoint, err = cloneFitPayload(a.Checkpoint); err == nil {
				a.Output, err = cloneFitPayload(a.Output)
			}
			if err != nil {
				r.fail(op.box, err)
				return false
			}
			choices = append(choices, a)
			return true
		}
		ok := accept()
		refused = !ok
		return ok
	})
	if !r.check(op.box, false) {
		return nil
	}
	if err != nil {
		r.fail(op.box, err)
		return nil
	}
	switch status {
	case FitExhausted:
	case FitStopped:
		r.status = FitStopped
		return nil
	default:
		r.fail(op.box, fmt.Errorf("%w: invalid paragraph enumeration status without error", ErrFitProtocol))
		return nil
	}
	slices.SortStableFunc(choices, func(a, b ParagraphAlternative) int {
		if a.Complete != b.Complete {
			if a.Complete {
				return -1
			}
			return 1
		}
		return b.End.Compare(a.End)
	})
	return choices
}
