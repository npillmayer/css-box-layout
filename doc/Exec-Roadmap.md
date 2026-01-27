# Execution Roadmap (Draft)

This roadmap captures the high-level, step-by-step plan to move from the current documentation to a working core layout pipeline. It is intentionally concise and avoids implementation details.

## 1) Consolidate the spec surface
Align `doc/CSS-Algo-Overview.md`, `doc/Algo-Spine.md`, and `doc/Functional.md` into a single “core algorithm contract”:
- phases, inputs/outputs, and responsibilities
- frozen invariants (B1–B4)
- explicit “not yet” features

## 2) Freeze IRs and invariants
Finalize the minimal IRs for:
- Box Tree (E)
- Layout Result (F)

Codify invariants as acceptance criteria for tests and later reviews.

Decision outputs (Step 2):
- Identity model: introduce `BoxId` unique per box; keep `NodeId` for source mapping only.
- Anonymous boxes: deterministic `BoxId` derived from parent `BoxId` + run index; no shared sentinel ID.
- Split inline fragments: unique `BoxId` per fragment, shared `NodeId` for source mapping.
- Used values: `UsedValuesTable[BoxId]` (keep out of the structural tree).
- Geometry: `LayoutGeometryTable[BoxId]` (Frame/Content), or if geometry stays on nodes, then all tables key by `BoxId`.
- Frame semantics: border box only; margins are separate in used values.
- Coordinate convention: all Frame/Content coords relative to parent content box; root origin (0,0).
- Inline-only empty containers: still create a `BoxAnonymousInline` child to preserve B1.
- Line boxes: `LinesByBlock` keyed by `BoxId` for inline-only block containers; decide whether to store for anonymous blocks, but be consistent.
- TextRef: stable `TextSourceID` and `[Start,End)` byte offsets; drop empty ranges at build time.

Validation checklist (to encode in tests later):
- B1: Block containers are either block-only or single BoxAnonymousInline child.
- B2: Inline formatting always starts at BoxAnonymousInline.
- B3: No block-level kinds under BoxAnonymousInline.
- B4: BoxText has no children and non-empty range.
- BoxId uniqueness across the tree; NodeId stable for real boxes.
- UsedValuesTable has entries for every BoxId in E.
- Geometry table has entries for every BoxId in layout scope.
- Frame/Content satisfy: Frame = Content + padding + border (epsilon).
- Frame/Content coords are relative to parent content box.
- Inline-only blocks: Content.H equals extent(lines); LinesByBlock stored per chosen rule.

## 2.1) Freeze decision log
Maintain a short log of decisions that are explicitly frozen for the current phase (and can be revisited later), e.g.:
- margin collapsing: deferred (margins kept, not collapsed)
- caching/memoization: deferred (design for it, do not implement yet)
- span-level line-height / fine inline metrics: deferred (structural first)
- bidi / RTL: deferred (LTR only)
- adjacent text-node merging: deferred
- mapping back to DOM/text for selection: not required now
- line breaking module: treated as a black box
- coordinate convention: origin (0,0), boxes relative to parent content box

## 3) Define pass boundaries and signatures
Lock down pure pass interfaces for:
- D → E (BuildLayoutTree)
- Resolve Used Values
- E → F (Flow Layout)

Do not allow external modules to dictate these signatures.

Artifacts:
- `doc/Pass-Signatures.md` (authoritative signatures + package structure).

Success:
- Fixed signatures with explicit inputs/outputs and no hidden dependencies.
- Provisional package layout is clear and acyclic.

## 4) Algorithm sketches + test outlines per pass
For each pass, add:
- short pseudocode sketches in docs
- draft table-driven test cases targeting invariants and expected outputs

No implementation yet.

Artifacts:
- `doc/Pass-Algorithms.md` (per-pass sketches + test outlines).

## 5) Implement Pass 1 (BuildLayoutTree)
Implement the structural box tree:
- anonymous box insertion
- split + hoist for inline containing block
- stable IDs and deterministic ordering

## 6) Implement Pass 2 (Resolve Used Values)
Implement a pure resolution step:
- margins, padding, borders, and width handling
- context-aware resolution via explicit inputs

## 7) Implement Pass 3 (Flow Layout)
Implement block layout + inline delegation:
- block stacking (no margin collapsing yet)
- inline layout delegated to the black-box layouter

## 8) Iterate with tests and doc updates
Use drafted tests to validate each pass.
Update docs to reflect actual behavior before adding refinements.
