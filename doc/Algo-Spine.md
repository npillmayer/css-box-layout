# Algorithm Spine (Rationale + Call Order)

Authoritative algorithm contract: `doc/CSS-Algo-Overview.md`.
This note captures the rationale and sequencing logic so we do not lose the "why" behind the pass order.

---

## Why the pass order is fixed

- Correct line breaking requires padding, borders, and margins to be resolved first.
- Therefore, ResolveUsedValues must run before any inline layout decisions.
- Block layout must stack after used widths are known so available inline width is correct.

---

## Incremental path (no signature churn)

Phase A: BuildLayoutTree
- Structural only (anonymous boxes, split+hoist), no metrics.

Phase B: ResolveUsedValues
- Concrete edges and used widths, still no margin collapsing.

Phase C: FlowLayout
- Block stacking and inline delegation, with resolved edges.

Later refinements (without changing signatures):
- Margin collapsing
- More accurate inline metrics
- Caching/memoization

---

## Non-goals for now

- Exact CSS shrink-to-fit
- Bidi/RTL and vertical writing modes
- Span-level line-height and inline fragments
