# Coding Necessities for the CSS Box + Flow Layout Algorithm
*A functional-programming-first implementation with disciplined memoization*

This document summarizes the **non-negotiable coding constraints** implied by two recurring themes:

1) **Functional programming paradigm**: treat styling/layout as transformations of immutable trees.  
2) **Memoization requirement**: design explicit, safe caching boundaries—without entangling correctness.

The goal is to let a programmer implement the pipeline cleanly today, and add optimizations later **without rewriting the architecture**.

---

## 1) Functional paradigm: what it means here

### 1.1. Core rule: “passes are pure transforms”
Each stage of the pipeline should be expressible as:

- **Input:** immutable data (tree + context)
- **Output:** new immutable data (tree/fragment + derived tables)
- **No hidden mutation:** no in-place edits of input trees; no global state influencing results

This is crucial because the domain is “tree modifications.” Most complexity comes from *when* and *where* you allow structure to change. The transcript’s intent was to make these changes **explicit** and **composable**.

### 1.2. Separate data from computation (tables, not fields)
Do not cram everything into one “fat node.” Prefer:

- A *structural* tree (BoxTree / FragmentTree)
- **Side tables** keyed by stable IDs for derived results (used values, measurements, layout results, etc.)

This prevents circular dependencies and allows memoization at clear boundaries.

### 1.3. Stable identity is mandatory
Memoization and functional transforms both rely on stable keys.

Necessity:
- Keep **stable IDs** for nodes/boxes/fragments across passes.
- Derivations should be keyed by these IDs + relevant context.

Practical rule:
- Every box has a `BoxId` that survives all subsequent passes.
- If the algorithm “drops truly empty ranges,” it must not destabilize other IDs.

### 1.4. Structural sharing (persistent data structures)
Functional code must remain efficient. Therefore:

- Use **persistent vectors/maps** or equivalent (structural sharing).
- When you “modify a tree,” you rebuild only the path to the changed node.
- Keep “box lists” and “inline item lists” as shareable sequences.

This is not optional if you want memoization + immutability without blowing memory.

### 1.5. Localize impurity to the boundary
I/O, font system calls, and platform APIs must be isolated:

- Wrap font measurement, resource fetches, etc. behind interfaces.
- Treat them as deterministic functions by providing explicit inputs (font id, size, text).

Rule of thumb:
- If a function can’t be deterministic, **it does not belong in the core passes**.

---

## 2) Pipeline architecture: enforce explicit pass boundaries

The transcript’s algorithm naturally wants these boundaries:

1. **BuildBoxTree (structural only)**
2. **ResolveUsedValues (length resolution step)**
3. **FlowLayout (block + inline; line breaking depends on used values)**

Coding necessities:

- Every pass must accept a fully specified input artifact and return a fully specified output artifact.
- No pass may silently depend on “later passes” to fill missing fields.
- If something is not implemented yet (e.g., margin collapsing), keep the data but use a simplified rule—do not remove the field from the model.

---

## 3) Function design rules (functional coding constraints)

### 3.1. Signatures must be explicit about context
Layout functions cannot “look up” context implicitly. Their signatures must include:

- Containing block metrics (available width, coordinate origin, etc.)
- Writing mode or direction if relevant (even if stubbed)
- References to derived tables (used values, font metrics)

Rationale:
- Memoization requires a reliable notion of “inputs.”

### 3.2. No “hidden mutable accumulators”
Prefer folds / builders that return new values:

- `fold(children, state) -> state'`
- `map(boxes, f) -> boxes'`
- `scanl`-style accumulation for y-offset stacking

If you need performance, use ephemeral builders internally but return immutable structures at the boundary (a controlled “escape hatch”).

### 3.3. Deterministic ordering and traversal
Ensure stable traversal:
- DOM order → box tree order → layout fragment order
- Avoid iteration over hash maps where iteration order is not guaranteed unless stabilized.

Memoization + reproducibility depend on deterministic order.

---

## 4) Memoization: what must be true to do it safely

Memoization is only correct if:
- Function is *pure* (same inputs → same outputs)
- Key captures *all* inputs that affect output
- Cached results are invalidated when inputs change

The transcript’s stance was: *“not now” for caching*, but **architect now so memoization can be added later without redesign**.

### 4.1. Choose memoization boundaries carefully
Memoize at *pass-level* or *subpass-level* where inputs are stable.

Recommended boundaries (high value, low risk):

- **Computed style** per NodeId (if style system exists)
- **Box construction** per NodeId + computed display rules
- **Used values resolution** per BoxId + containing block inline size
- **Text measurement** per (font key, text slice)
- **Inline line breaking** per (InlineItems hash, available width, break policy)
- **Block layout** per (BoxId, available width, relevant used values)

Avoid memoizing “mid-stream mutable states” like partially filled lines unless the state is itself an explicit immutable value.

### 4.2. Use content-addressable keys (hashes)
Prefer keys based on:
- Stable IDs + versions, or
- Hashes of immutable inputs

Example concept:
- `LineBreakKey = hash(InlineItemList) + availableWidth + lineBreakPolicyVersion`

Key necessity:
- If any input is omitted from the key, memoization becomes a correctness bug.

### 4.3. Version everything that is “not implemented yet”
As the engine evolves, memoized results must not persist across semantic changes.

Practical rule:
- Include **policy/version numbers** in memo keys for:
  - margin collapsing (currently off)
  - line-height policy (currently simplified)
  - whitespace collapsing rules (if not final)
  - bidi/writing mode behavior (if stubbed)

This prevents “old behavior cached under new semantics.”

### 4.4. Invalidation strategy must be explicit
Even in a functional system, you still need a strategy for changes (DOM edits, style changes).

Recommended model:
- Maintain an immutable “world state” with **generation counters**:
  - `StyleGen`, `LayoutGen`, `FontMetricsGen`, etc.
- Keys include the relevant generation number(s), or
- You compute hashes from the exact immutable inputs (structural sharing makes this feasible)

Rule:
- If you can’t explain invalidation in one sentence for a cached function, don’t memoize it yet.

### 4.5. Don’t memoize too early; design for it
The transcript prioritized clarity and space-optimized structures over caching “for the immediate future.”

Actionable constraint:
- Implement passes as pure transforms first.
- Ensure every pass can be invoked independently with explicit inputs.
- Only then introduce memoization around the clean boundaries above.

---

## 5) Data-structure necessities to support functional + memoized layout

### 5.1. Immutable trees + side tables
Use:
- `BoxTree` (structure)
- `UsedValuesTable: BoxId -> UsedValues`
- `Fragments: BoxId -> Fragment` (or FragmentTree)
- `MeasurementsTable: (FontKey, TextKey) -> Metrics`

Avoid:
- Storing “computed fields” inside nodes that change frequently; it breaks sharing and complicates invalidation.

### 5.2. Stable box identities through transformations
Necessity:
- Box construction and fixups (anonymous boxes, dropping empty ranges) must preserve stable identity rules.

Guideline:
- Anonymous boxes get their own stable IDs derived deterministically from their parent and run index (not from memory addresses).

### 5.3. Explicit edge model even when algorithms are deferred
Because line-breaking needs margins/padding/borders:

- Represent `Edges { margin, border, padding }` explicitly and early.
- “No margin collapsing” means: **do not remove margins**; just skip the collapse step.

This keeps later additions local: you add a collapsing transform without changing the IR.

---

## 6) Practical “coding checklist” (what to enforce in reviews)

### Functional correctness checklist
- [ ] No global mutable state used by core passes
- [ ] Every pass is deterministic for the same input
- [ ] Every derived value has an explicit source (table or output)
- [ ] Traversal order is stable and documented
- [ ] Tree changes use structural sharing (no full rebuild when avoidable)

### Memoization readiness checklist
- [ ] Each candidate memoized function has a complete input key
- [ ] Keys include policy/version toggles for unfinished semantics
- [ ] Invalidation story is explicit (gens or hashed inputs)
- [ ] Cached outputs are immutable and reusable
- [ ] Measurement APIs are wrapped so they behave deterministically

---

## 7) Recommended layering

1) **Pure core** (library-like)
- Box building
- Used value resolution
- Flow layout (block + inline)
- Text measurement interface (purely functional wrapper)

2) **Orchestrator**
- Chooses when to run which passes
- Connects external systems (DOM, font backend)
- Owns memo tables (but calls pure functions)

3) **Memoization/caching layer**
- Key computation
- Cache lookup/store
- Invalidation policy

This separation ensures you can turn memoization on/off without altering core algorithms.

---

## 8) Summary: the “necessities” in one sentence
Implement the pipeline as **pure, deterministic transforms over immutable trees with stable IDs**, keep deferred behaviors as explicit data (not missing fields), and only memoize at well-defined boundaries with **complete keys + versioned semantics + explicit invalidation**.
