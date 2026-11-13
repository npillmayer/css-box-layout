# Algorithm Spine (Rationale + Call Order)

Authoritative algorithm contract: `doc/CSS-Algo-Overview.md`.
This note captures the rationale and sequencing logic so we do not lose the "why" behind the pass order.

---

## Required dependency order

- Correct line breaking requires padding, borders, and margins to be resolved first.
- Therefore, resolve the relevant used edges before each inline layout decision.
- Auto-width inline-blocks introduce a dependency: their final width is chosen
  during sizing, but descendants need that width for used-value resolution.
  The mechanism is still open (checklist tasks 2 and 3); the three conceptual
  passes do not yet establish a complete one-shot scheduling solution.
- Block layout must stack after used widths are known so available inline width is correct.

---

## Incremental path

Phase A: BuildLayoutTree
- Structural only (anonymous boxes, split+hoist), no metrics.

Phase B: ResolveUsedValues
- Concrete edges and known used widths, still no margin collapsing.
- Keep unresolved auto inline-block width distinct from explicit zero.

Phase C: FlowLayout
- Block stacking and inline delegation, with resolved edges.

Later refinements (interface compatibility must be evaluated when designed):
- Margin collapsing
- More accurate inline metrics
- Caching/memoization

---

## Non-goals for now

- Full CSS intrinsic-sizing semantics; bounded min/max shrink-to-fit is implemented
  under tasks 2B/3B (see Pass-Signatures.md section 4.1)
- Bidi/RTL and vertical writing modes
- Span-level line-height and inline fragments
