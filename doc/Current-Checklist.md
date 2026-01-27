# Current Checklist (Step 2 Draft)

This file captures the Step 2 draft checklist for freezing IRs and invariants.
It is a planning artifact; do not treat it as the final contract.

---

## Box Tree (E) checklist — fields and rules to freeze

Identity:
- Choose one: `BoxId` (unique per box) vs `NodeId` (shared across split fragments).
- If using `NodeId` only, side tables must use `(NodeId, BoxKind, fragmentIndex)` as key.
- If using `BoxId`, define how to derive for anonymous and split fragments.

Core node shape:
- `ID` (stable identity)
- `BoxKind` (enum; explicitly define block-level kinds)
- `StyleRef` (pointer or ID to computed style; nil only for anonymous?)
- `Children` (ordered, immutable list)
- `TextRef` (only for BoxText)

TextRef:
- `Source TextSourceID` (stable across passes)
- `Range [Start,End)` in bytes; drop empty at build time.

Used values:
- Either stored on node (as in overview) or in `UsedValuesTable[BoxId]`.
- Define `Margin/Padding/Border` units: px float.

Geometry fields:
- `Frame Rect` (border box, relative to parent content box)
- `Content Rect` (content box, relative to parent content box)

BoxKind definitions:
- Block-level kinds: BoxBlock, BoxAnonymousBlock, BoxInlineBlock.
- Inline-level kinds: BoxInline, BoxAnonymousInline, BoxText, BoxInlineBlock (as atomic in inline context).

Invariants representation:
- For B1 inline-only: define whether empty inline-only containers still get a BoxAnonymousInline child (recommend yes for structural determinism).
- For B3 purity: define a predicate `IsBlockLevel(BoxKind)` used by validation.

---

## Layout Result (F) checklist — fields and rules to freeze

Result container:
- `Root *LayoutNode` (if geometry stored in E) OR `Fragments` tree separate from E.
- `LinesByBlock map[BoxId][]LineBox` (prefer BoxId for uniqueness).

LineBox:
- `Frame Rect` (relative to owning block content box)
- `H` is line height; `Y` stacked by inline layouter
- Optional payload (opaque inline fragments)

Geometry completeness:
- Frame and Content are set for all layout nodes in scope.
- Frame/Content widths include/exclude padding/border per spec.

Margin representation:
- Margins remain in used values; Frame is border box (no margins).

Ownership:
- Lines are only stored for inline-only block containers and only for non-anonymous owners (if you keep that rule, specify why).
- Inline-blocks: `SizeAtomicInline` returns border-box size; stored in node/frame.

---

## Acceptance criteria tied to invariants (candidate test statements)

- B1 block-container normalization:
  - For every block container, children are either all block-level or a single BoxAnonymousInline.
- B2 explicit inline formatting:
  - Any inline formatting context is rooted at a BoxAnonymousInline.
- B3 inline subtree purity:
  - BoxAnonymousInline subtree contains no block-level kinds.
- B4 BoxText leaf:
  - BoxText has no children; text range is non-empty.
- Identity stability:
  - IDs for real boxes are stable across passes; split inline fragments keep identity policy consistent.
- Anonymous handling:
  - Anonymous boxes are deterministic (ID policy explicitly defined and tested).
- Geometry consistency:
  - `Frame.W/H == Content.W/H + padding + border` (within epsilon).
  - `Frame` and `Content` coordinates are relative to parent content box.
- Inline-only geometry:
  - If inline-only, `Content.H == extent(lines)`.
