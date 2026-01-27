## CSS Flow Layout Pipeline

This is the **algorithmic spine** of what we discussed: how we go from a styled DOM to **boxes**, then to **flow layout**, while deliberately postponing certain spec-heavy refinements (notably **margin collapsing**, caching, and span-level line-height intricacies).

---

# 1) Scope and “not yet” decisions

### Implement now

* **Box construction** must exist as a stable intermediate representation (IR), even if not all CSS behaviors are implemented yet.
* **Flow layout** for block and inline content, including **line breaking**, needs access to **padding/border/margins** early enough to be correct for line metrics and available width.
* A distinct **resolve-length** step exists (explicitly called out as required).

### Postpone (explicitly)

* **Margin collapsing** (we still keep margins in the model; we just don’t collapse them yet).
* **Caching** (we prioritize clarity + space-optimized structures over caching).
* **Span-level line-height** / fine inline metric refinements (structural first; we may never fully support span-level line-height).

### Representation choices that affect the algorithm

* **Same ID mapping** across stages (a stable identity strategy so later passes can refer back cleanly).
* **Drop truly empty ranges**, but **keep boxes** even if they are “empty” in some semantic sense, because the box is still needed for **sizing and layout**.
* Use a **(0,0)** origin convention for now.

---

# 2) The pipeline at a glance (passes + responsibilities)

Think of the engine as a small number of explicit passes, each with a narrow contract:

1. **Build boxes (structural IR)**
2. **Resolve lengths / used values**
3. **Flow layout (block + inline)**
4. *(later)* margin collapsing, caching, refined inline metrics

This ordering is not cosmetic: the transcript’s key insight was that **you can’t do correct line-breaking without knowing borders/padding/margins**, therefore the *resolve* step must happen before layout (or at least before line breaking).

---

# 3) Core IR: what the algorithm moves through

Below are the conceptual data structures implied by the discussion (names are representative; the point is the role):

### Identity

* `NodeId`: stable ID for DOM nodes.
* `BoxId`: stable ID for boxes (may be derived from node identity + kind).
* **Invariant:** “same ID” means if a node generates a principal box, that box keeps a stable identity across passes.

### Boxes (pre-layout)

* `BoxTree`: root box + children.
* `Box` variants:

  * `BlockBox`
  * `InlineBox`
  * `AnonymousBlockBox` (needed for mixed inline/block situations)
  * Possibly `TextRunBox` or text as inline items (depending on your split)
* Each box carries:

  * `style` / computed style reference
  * `edges`: margins/padding/borders (not yet collapsed)
  * `children` (box tree)
  * `content_source`: node reference, text range, etc.

### Layout outputs (post-layout)

* `FragmentTree` / `LayoutTree`:

  * `Fragment` has geometry: position + size (coordinates relative to containing block).
  * `LineBox` fragments for inline formatting contexts.
  * Block fragments stacked in normal flow.

### Inline items (inside inline formatting context)

* `InlineItem` list, e.g.:

  * `TextRun`
  * `InlineBoxStart/End` (or nested item lists)
  * `ReplacedElement`
* The “drop truly empty ranges” decision lives here: do not emit meaningless text ranges/items.

---

# 4) Call hierarchy (who calls whom)

## Top-level orchestration

### `layout_document(root_node) -> FragmentTree`

**Calls:**

1. `build_box_tree(root_node) -> BoxTree`
2. `resolve_used_values(box_tree) -> BoxTree` *(or produces a parallel “used-values” table keyed by BoxId)*
3. `layout_flow(box_tree) -> FragmentTree`

---

## Pass 1: Build boxes (structural)

### `build_box_tree(node) -> BoxTree`

**Calls:**

* `build_boxes_for_node(node) -> [Box]`
* `fixup_anonymous_boxes(children: [Box]) -> [Box]`

### `build_boxes_for_node(node) -> [Box]`

Pseudo-body (comment-style):

* Determine display/type from computed style (already assumed available).
* If `display:none`: return `[]`.
* Otherwise:

  * Create principal box with stable `BoxId` (“same ID” policy).
  * For text nodes:

    * Emit inline text representation (text range) but **drop truly empty ranges**.
  * Recurse into children and attach generated child boxes.
  * **Keep boxes** even if they look “empty” in content terms (because geometry still matters later).

### `fixup_anonymous_boxes(children) -> children'`

Pseudo-body:

* Enforce block formatting context rules for mixed children:

  * If a block container has both block-level and inline-level children:

    * Wrap consecutive inline-level runs into `AnonymousBlockBox`.
* This is purely structural in the current phase (no metrics yet).

**Interplay note:** This step ensures the later flow layout can treat a block container as a sequence of block children (some may be anonymous blocks that themselves contain inline content).

---

## Pass 2: Resolve lengths / used values

This is explicitly required in the transcript (“We need a resolve length step.”). Also: “border thickness are known floats” and “assume font sizes are computed”.

### `resolve_used_values(box_tree) -> box_tree_used`

**Calls:**

* `resolve_box_used_values(box, containing_block)`

### `resolve_box_used_values(box, containing_block)`

Pseudo-body:

* Resolve:

  * `margin-*`, `padding-*` (percentages vs containing block)
  * `border-*` thickness (already “known floats”)
  * `width/height` if specified (auto stays unresolved until layout, if needed)
* Store resolved edges on the box (or in a `UsedValues` table keyed by BoxId).

**Interplay note:** The transcript emphasized needing borders/margins/padding available for **line breaking** correctness. This is the gatekeeper pass that makes that true.

---

## Pass 3: Flow layout (block + inline)

### `layout_flow(box_tree) -> FragmentTree`

**Calls:**

* `layout_block_formatting_context(root_box, initial_containing_block)`

---

# 5) Block formatting context (normal flow stacking)

### `layout_block_formatting_context(block_box, containing_block) -> BlockFragment`

**Calls:**

* `compute_available_inline_size(containing_block, block_box.used_edges)`
* For each child box:

  * if block-level: `layout_block_child(child, this_block_content_box)`
  * if inline-level (or anonymous block containing inline): `layout_inline_formatting_context(child, this_block_content_box)`
* `stack_block_fragments(children_fragments)` (normal flow vertical stacking)

Pseudo-body highlights:

* Determine **content box** by subtracting padding/border from the assigned box width.
* Children are laid out in order; y-offset increments by each child’s margin-box height.
* **Margin collapsing is not performed**:

  * Margins are applied “as-is” (simple additive stacking).
  * Keep the representation so collapsing can be added later without redesign.

**Interplay note:** This pass consumes the earlier “resolve” results. Even without collapsing, having concrete edge sizes is essential to compute available width for inline layout.

---

# 6) Inline formatting context (line breaking + line boxes)

### `layout_inline_formatting_context(inline_container_box, containing_block) -> BlockFragment(or AnonymousBlockFragment)`

**Calls:**

1. `collect_inline_items(inline_container_box) -> [InlineItem]`
2. `break_into_lines(items, available_width) -> [Line]`
3. For each line: `layout_line(line, line_y, available_width) -> LineFragment`
4. `compute_inline_container_height(lines)`

### `collect_inline_items(box) -> [InlineItem]`

Pseudo-body:

* Traverse inline descendants in tree order.
* Emit:

  * text runs (with non-empty ranges only)
  * inline box boundaries / replaced elements as items
* Preserve enough structure to later apply inline box padding/border if present.

### `break_into_lines(items, available_width) -> [Line]`

Pseudo-body:

* Greedy fill:

  * Maintain `current_line_width`.
  * For each item:

    * Measure its inline advance.
    * If it fits: append.
    * If it doesn’t:

      * If break opportunity exists inside text: split text run.
      * Otherwise: move item to next line (or overflow policy later).
* Requires:

  * known available width
  * item contributions including **padding/border/margins** where relevant

### `layout_line(line, y, available_width) -> LineFragment`

Pseudo-body:

* Assign x positions in order from line start.
* Determine line height:

  * Currently “structural first”; detailed span-level line-height is postponed.
  * Use a baseline strategy that can later be refined (e.g., font metrics at line level).
* Produce `LineBox` fragment containing positioned child fragments.

**Interplay note:** Inline layout is where the earlier insistence on having padding/border/margins available matters most: without them you miscompute fit decisions and breakpoints.

---

# 7) Incremental path (how the functions evolve without rewriting them)

The transcript’s “incremental path” is essentially: **freeze the call graph early**, then deepen function bodies over time.

### Phase A (now): “purely structural”

* `build_box_tree` + `fixup_anonymous_boxes` produce a stable box IR.
* Inline collection exists, but metrics may be simplistic.
* IDs are stable; empty ranges dropped; boxes preserved.

### Phase B: resolve used values

* `resolve_used_values` becomes authoritative for edges.
* Border thickness treated as known floats.
* Font sizes assumed computed (so later text measurement can be meaningful).

### Phase C: correct-enough flow layout

* Block stacking works.
* Inline line breaking uses available width computed from resolved edges.
* Still no margin collapsing.

### Later refinements (without changing the shape of the pipeline)

* Add margin collapsing inside `layout_block_formatting_context` (or a dedicated pre-layout pass).
* Add more accurate inline metrics (possibly per-span if ever implemented).
* Add caching as an orthogonal concern.

---

# 8) Full function inventory (with “pseudo-bodies”)

Below is the consolidated “function surface” implied by the algorithm above.

### Orchestration

* `layout_document(root_node)`

  * // build boxes
  * // resolve used values
  * // layout flow

### Box construction

* `build_box_tree(node)`

  * // recursively generate boxes
  * // fixup anonymous wrappers
* `build_boxes_for_node(node)`

  * // return [] for display:none
  * // emit principal box with stable id
  * // keep boxes even if content-empty
  * // drop truly empty text ranges
* `fixup_anonymous_boxes(children)`

  * // wrap inline runs into anonymous block boxes when mixed with block children

### Used values / resolving

* `resolve_used_values(box_tree)`

  * // walk boxes; fill UsedValues keyed by BoxId
* `resolve_box_used_values(box, containing_block)`

  * // resolve percent/auto/lengths for margins/padding
  * // border thickness already concrete floats
  * // assumes font sizes computed

### Block flow

* `layout_flow(box_tree)`

  * // start BFC on root
* `layout_block_formatting_context(block_box, containing_block)`

  * // compute available width from used edges
  * // layout children in normal flow
  * // no margin collapsing yet
* `layout_block_child(child_box, parent_content_box)`

  * // layout child as block; return fragment

### Inline flow

* `layout_inline_formatting_context(inline_container_box, containing_block)`

  * // collect inline items
  * // break into lines
  * // layout each line to fragments
* `collect_inline_items(box)`

  * // flatten inline descendants into measurable items
  * // omit empty ranges
* `break_into_lines(items, available_width)`

  * // greedy packing + splitting at break opportunities
  * // uses padding/border/margins in width contributions
* `layout_line(line, y, available_width)`

  * // position items along x
  * // compute line height (coarse for now; span-level postponed)

---

# 9) The key interplay (why this specific call graph matters)

* **Box construction** gives you stable structure + identity. Without it, every later refinement turns into a rewrite.
* **Resolve lengths** is the bridge between “CSS values” and “numbers the layout engine can trust”. The transcript singled this out as non-optional.
* **Inline breaking** is the forcing function that dictates early numeric availability of edges (padding/border/margins), even if other spec features are postponed.
* **Margin collapsing** is explicitly decoupled: we preserve margins in the IR, but treat them simply during stacking until the collapse logic is added.
