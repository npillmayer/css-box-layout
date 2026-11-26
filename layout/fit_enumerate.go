package layout

import (
	"context"
	"fmt"
	"maps"
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
	used, err := ResolveUsedValues(p.tree, ResolveContext{ContainingBlock: Rect{W: req.Width}, FontSizePx: f.options.FontSizePx})
	if err == nil {
		// These occurrences have already been accepted. Their used values are
		// part of the checkpoint, not candidates for the new receiving width.
		for _, a := range c.locked {
			maps.Copy(used, a.used)
		}
		err = validateUsedValues(p.tree, used)
	}
	if err != nil {
		return FitFailed, &FitError{p.root, fmt.Errorf("%w: %w", ErrInvalidFit, err)}
	}
	r := fitRun{used: used, ctx: ctx, fitter: f, req: req, stop: stop, yield: yield, status: FitExhausted}
	r.explore(fitState{cursor: c}, req.IncludeEmpty)
	return r.status, r.err
}

type fitState struct {
	cursor     fitCursor
	paragraphs []ParagraphFragment
	containers []ContainerFragment
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
	used   UsedValuesTable
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
	result, fits, err := r.fragmentGeometry(s)
	if err != nil {
		r.fail(r.fitter.program.root, err)
		return
	}
	if !fits {
		return
	}
	// All slices and backend payloads in s are private snapshots. Neither later
	// exploration nor accessors hand out a writable reference to them.
	result.next = FitContinuation{r.fitter.program, s.cursor}
	result.progress, result.stop = s.progress, s.cursor.index == r.stop
	a := FitAlternative{result}
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
			Complete: choice.Complete, Extent: choice.Extent, Output: choice.Output,
			allowsMarginCollapse: choice.AllowsMarginCollapse, atomics: choice.accepted,
		})
		next.progress = true // completing an empty group is structural progress
		if !choice.Complete {
			next.cursor.point, next.cursor.checkpoint = choice.End, choice.Checkpoint
			next.cursor.locked = choice.locked
			r.publish(next)
		} else {
			next.cursor.index++
			next.cursor.point, next.cursor.checkpoint = ParagraphPosition{}, nil
			next.cursor.locked = nil
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

func (r *fitRun) paragraphs(s fitState, op fitOp) []fitChoice {
	checkpoint, err := cloneFitPayload(s.cursor.checkpoint)
	if err != nil {
		r.fail(op.box, err)
		return nil
	}
	items, used, err := r.groupAtomics(op.box, s.cursor.locked, r.used)
	if err != nil {
		r.fail(op.box, err)
		return nil
	}
	if r.status != FitExhausted {
		return nil
	}
	req := ParagraphFitRequest{Root: r.fitter.program.root, Group: op.box, Owner: op.owner,
		Width: used[op.owner].ContentWidth, Height: r.req.Height, From: s.cursor.point, Checkpoint: checkpoint,
		Used: usedValuesView{used}, Atomics: atomicFitInputs(items, s.cursor.locked)}
	return r.enumerateParagraph(req, items, s.cursor.locked, false)
}

// Collect all choices before traversal so provider order cannot truncate shorter
// alternatives. Atomics are realized once for this call and shared immutably.
func (r *fitRun) enumerateParagraph(req ParagraphFitRequest, items []*fitAtomic, locked []*fitAtomic, interior bool) []fitChoice {
	var choices []fitChoice
	refused := false
	status, err := r.fitter.paragraphs.EnumerateParagraph(r.ctx, req, func(a ParagraphAlternative) bool {
		if refused {
			r.fail(req.Group, fmt.Errorf("%w: callback continued after false yield", ErrFitProtocol))
			return false
		}
		accept := func() bool {
			if !r.check(req.Group, true) {
				return false
			}
			if !finite(a.Extent.W) || !finite(a.Extent.H) || a.Extent.W < 0 || a.Extent.H < 0 ||
				a.End.Compare(req.From) < 0 || (!a.Complete && a.End.Compare(req.From) == 0) ||
				a.Complete != nilFitValue(a.Checkpoint) || nilFitValue(a.Output) ||
				a.AllowsMarginCollapse && (a.Extent.H != 0 || len(a.Atomics) != 0) {
				r.fail(req.Group, fmt.Errorf("%w: invalid extent, progress, collapse or payload association", ErrFitProtocol))
				return false
			}
			choice, err := freezeFitChoice(a, req.Owner, items, locked)
			if err != nil {
				r.fail(req.Group, err)
				return false
			}
			// Atomic interiors need a complete layout even when that layout
			// overflows. Physical bounds are checked on the outer fragment.
			if !interior && (a.Extent.W > req.Width || a.Extent.H > req.Height) {
				return true
			}
			choices = append(choices, choice)
			return true
		}
		ok := accept()
		refused = !ok
		return ok
	})
	if !r.check(req.Group, false) {
		return nil
	}
	if err != nil {
		r.fail(req.Group, err)
		return nil
	}
	switch status {
	case FitExhausted:
	case FitStopped:
		r.status = FitStopped
		return nil
	default:
		r.fail(req.Group, fmt.Errorf("%w: invalid paragraph enumeration status without error", ErrFitProtocol))
		return nil
	}
	slices.SortStableFunc(choices, func(a, b fitChoice) int {
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
