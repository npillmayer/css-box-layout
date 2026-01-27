# Repository Guidelines

## Project Structure & Module Organization
- `doc/` holds design and algorithm notes. Start with `doc/CSS-Algo-Overview.md`.
- Source code and tests are not present yet; this repo is currently documentation-only.

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
