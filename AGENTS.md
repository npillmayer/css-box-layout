# Repository Guidelines

## Project Structure & Module Organization
- `doc/` holds design and algorithm notes.
  - `doc/CSS-Algo-Overview.md`: single authoritative core algorithm contract.
  - `doc/Algo-Spine.md`: rationale + sequencing logic (why the pass order is fixed).
  - `doc/Functional.md`: functional-style constraints and memoization readiness.
- `doc/Exec-Roadmap.md`: execution roadmap with frozen decisions/checklists (no separate decision log).
- `doc/Current-Checklist.md`: working checklist for IR freeze and invariants (planning artifact).
- `doc/Pass-Signatures.md`: pass boundaries, signatures, and provisional package structure.
- The repo includes early Go stubs under `layout/`, `glyphing/`, and `text/`.

## Build, Test, and Development Commands
- No build, run, or test commands are defined yet.
- When adding code, document new commands here (e.g., `go test ./...`, `go test -run TestName`, `go vet ./...`).

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
