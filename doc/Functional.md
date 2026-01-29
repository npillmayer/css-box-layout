# Functional Constraints (Core Passes)

Authoritative algorithm contract: `doc/CSS-Algo-Overview.md`.
This document defines the functional-style constraints and memoization-readiness rules for implementing the contract.

---

## 1) Core rules

- Passes are pure transforms: immutable input -> immutable output.
- No hidden mutation or global state in core passes.
- All context is explicit in function signatures (containing block, writing mode stub, used values tables, etc.).
- Deterministic traversal order: DOM order -> box tree order -> fragment order.

---

## 2) Data separation

Prefer structural trees + side tables:
- BoxTree / LayoutNode for structure
- UsedValuesTable keyed by BoxId
- Layout results (fragments, line boxes) keyed by BoxId

Avoid "fat mutable nodes" that mix structure with frequently changing derived data.

---

## 3) Stable identity

- Boxes have stable IDs across passes.
- BoxId is unique per box (including anonymous).
- Split inline fragments may share the same NodeId.
- Dropping empty text ranges must not destabilize other IDs.

---

## 4) Memoization readiness (not now, design for it)

- Only memoize pure functions with complete input keys.
- Include policy/version toggles for unfinished semantics (margin collapsing, line-height policy, bidi).
- Key choices: stable IDs + explicit context, or hashes of immutable inputs.
- Explicit invalidation strategy (generation counters or content hashes).

---

## 5) Layering model

1. Pure core: BuildLayoutTree, ResolveUsedValues, FlowLayout
2. Orchestrator: connects DOM and external systems
3. Cache layer: optional, isolated, replaceable

---

## 6) Review checklist

Functional correctness:
- No global mutable state in core passes
- Deterministic output for identical inputs
- Explicit data dependencies
- Stable traversal order

Memoization readiness:
- Complete input keys
- Versioned semantics included in keys
- Clear invalidation story
- Cached outputs immutable and reusable
