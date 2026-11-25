# Repository Guidelines

## Project Structure & Module Organization
- `doc/` holds design and algorithm notes.
  - `doc/CSS-Algo-Overview.md`: single authoritative core algorithm contract.
  - `doc/Algo-Spine.md`: rationale + sequencing logic (why the pass order is fixed).
  - `doc/Functional.md`: functional-style constraints and memoization readiness.
- `doc/Exec-Roadmap.md`: execution roadmap with frozen decisions/checklists (no separate decision log).
- `doc/Current-Checklist.md`: working checklist for IR freeze and invariants (planning artifact).
- `doc/Pass-Signatures.md`: pass boundaries, signatures, and provisional package structure.
- `doc/Pass-Algorithms.md`: per-pass pseudocode sketches and test outlines.
- `doc/Tree-Type.md`: CSSDOM tree types and traversal interface.
- Core layout code lives under `layout/`; source-reference types live under `text/`.
- `BuildLayoutTree`/`BuildLayoutTrees` return `*layout.BuildError` with RootID,
  optional offending NodeID (HasNode), and an unwrapped cause. Preserve these
  source locations and errors.Is/errors.As compatibility without parsing prose.
  BoxIDs are scoped to a construction call; independent filtered projections
  preserve source IDs/styles/text ranges, not globally unique BoxIDs.
- Glyph representations and line breaking belong to external typography services.
- BOX-PREP-02 distinguishes realized atomic geometry from paragraph preparation.
  `RealizedAtomic` and `RealizedAtomicsView` expose copied border-box dimensions
  in CSS pixels for one layout invocation; margins and final placement remain
  separate. Private `layoutAtomicInteriors` recursively lays out those interiors
  before outer delegation. Keep input trees/used values immutable and publish
  only complete results. The previous prepared-atomic names have no aliases.
- `layout.MeasureIntrinsic` exposes the existing recursive intrinsic pass without
  final layout. Return border-box bounds excluding the root's margins; definite
  widths contribute their prescribed size and auto widths recurse. Each call has
  fresh scratch; preserve inputs, callback causes and zero output on failure.
  Keep unknown-auto percentage restrictions and the normal-flow atomic profile.
- `layout.InlineGroups` enumerates normalized groups in structural preorder,
  including empty groups and atomic interiors. Descriptors borrow read-only
  nodes; each Atomics slice is caller-owned. Keep BoxIDs tree-local, and keep
  source interpretation, shaping and sizing out of enumeration.
- `registers` provides persistent `Register[T any]` and `RegisterBank[T any]`.
  Payloads need no identity/comparability interface; empty reads return zero T
  and false. Keep queue/map versions immutable, share only read-only referenced
  payload data, and release drained storage. No layout dependency or consumer
  box/preparation semantics belong in this package.

## Build, Test, and Development Commands
- Run `go test ./... -count=1`, `go test -race ./... -count=1`, `go build ./...`
  and `go vet ./...` with the local dependency wiring.
- Focused margin regressions: `go test ./layout -run 'TestBlockHorizontalMargins|TestAutoMarginsAfterIntrinsicWidthSelection' -count=1`.

## Coding Style & Naming Conventions
- No repository-specific style guide yet.
- If you add Go code, follow standard Go formatting (`gofmt`) and idiomatic naming (CamelCase for exported, lowerCamelCase for unexported).

## Testing Guidelines
- No test framework or coverage target is defined yet.
- If you add tests, prefer table-driven Go tests and locate them alongside code as `*_test.go`.

## Commit & Pull Request Guidelines
- Only an initial commit exists, so no commit message convention is established.
- For new work, use clear, imperative commit messages (e.g., “Add layout tree builder stub”).
- PRs should include: a short summary, rationale, and references to any design updates in `doc/`.

## Agent-Specific Instructions
- Keep the docs concise and aligned with the algorithm outline in `doc/CSS-Algo-Overview.md`.
- Focus on the core CSS layout algorithm described in the docs; do not let external interface shapes dictate signatures yet.
- Assume CSSDOM creation, line-breaking, and text shaping live in other repos and are integrated via interfaces defined here.
- Prefer functional-style design (pure transforms, explicit inputs/outputs), but defer `fp-go` adoption for now.
- It is acceptable to sketch logic in comments or Markdown and to design unit test cases up-front without implementing them.
- We are not optimizing for speed of delivery; correctness and clarity of the algorithm come first.
- Do not create or maintain a separate decision log; frozen decisions live in `doc/Exec-Roadmap.md`.

- BOX-SHIP-01 provides opt-in `BuildLayoutTreesWithProvenance`, using the same
  normalizer with private emitted-flow tracing. Keep records detached and copied;
  no markers, nodes, text processing or pagination semantics belong in them.
  Source IDs must be unique for this API, with zero allowed. Structural ordinals
  are root/build-local; source occurrences may split or disappear. Preserve
  explicit omitted/empty/suppressed endpoints without nearest-box guessing.
