# Absolute positioning: overlap with fixed layout

Status: **postponed** at the user's request. This document preserves the
investigation of 2026-11-20. It is not an approved implementation plan or a frozen
API contract. No absolute-positioning implementation is scheduled by this note.
Page-layout rules are outside its scope. The authoritative implemented contract
remains [CSS-Algo-Overview.md](CSS-Algo-Overview.md); frozen decisions remain in
[Exec-Roadmap.md](Exec-Roadmap.md).

## Assessment

Most of the sizing and interior-layout machinery can be shared. General absolute
layout chiefly needs containing-block selection, dependency ordering, and
static-position information.

CSS treats fixed positioning as a specialization of absolute positioning. Both
remove boxes from normal flow and establish independent formatting contexts;
the principal distinction is how their containing block is selected. Our fixed
implementation supports the narrower case where every fixed root uses the
caller's viewport. See [CSS Positioned Layout](https://www.w3.org/TR/css-position-3/#position-property).

The proposed direction, if this topic resumes, is a shared internal positioned-box
layout engine, retaining `FixedLayout` as the public entry point for its existing
viewport contract.

## Reusable implementation

| Existing implementation | Reuse for absolute layout |
|---|---|
| `fixedAxis.solve` in [fixed_sizing.go](../layout/fixed_sizing.go) | Reuse the constraint equations for the current LTR, non-replaced profile. The arithmetic already works with a containing dimension without requiring viewport semantics. |
| `resolveFixedSizing` in [fixed_sizing.go](../layout/fixed_sizing.go) | Generalize root-style access and validation. Retain percentage resolution, stretch/shrink-to-fit width, auto margins, and pending content height. |
| `withContentHeight` in [fixed_sizing.go](../layout/fixed_sizing.go) | Reuse height completion and bottom anchoring, including preservation of prescribed heights under content overflow. |
| `layoutBlockContent` in [layout.go](../layout/layout.go) | Reuse directly: interior layout is separated from construction and validation of the positioned root's final frame. |
| [Intrinsic measurement](../layout/intrinsic.go) and [descendant resolution](../layout/used_values.go) | Reuse service interfaces, the invocation-local cache, atomic realization, and ordinary descendant layout. |
| `fixedRootGeometry` in [fixed_geometry.go](../layout/fixed_geometry.go) | Reuse local rectangle construction; generalize reference-space attachment. |
| `divertFixed` in [build.go](../layout/build.go) | Reuse early diversion, normalization, identity allocation, and style snapshots. Add containing-block associations. |

The solver already implements the relevant CSS 2.2 absolute-positioning width
and height equations. Reusing it preserves a deliberate supported subset; it
does not itself add replaced-element sizing, min/max constraints, RTL, or newer
alignment features. See [CSS 2.2 sizing rules](https://www.w3.org/TR/CSS22/visudet.html#abs-non-replaced-width).

## New design work

### 1. Preserve containing-block identity during construction

An absolute box uses the nearest ancestor establishing an absolute-positioning
containing block, falling back to the initial containing block. A block ancestor
normally supplies its padding box; it need not be the immediate parent. See
[containing-block rules](https://www.w3.org/TR/CSS22/visudet.html#containing-block-details).

The flat fixed-root list needs no such relationship because all roots share one
viewport. An absolute branch would need records associating each diverted root
with its containing-block owner or an external initial containing block.

Record this during source-tree traversal, before anonymous wrapping and
split/hoist normalization obscure ancestry. `NodeID` alone is insufficient when
an inline source produces several fragments.

Keep `EstablishesBFC` separate from this information. Formatting-context
containment and positioned containing-block ownership serve different purposes.

### 2. Adapt the coordinate contract explicitly

Existing descendant coordinates refer to an owner's content origin. Absolute
sizing ordinarily uses the owner's padding rectangle.

We can retain `LayoutGeometry.RelativeTo`: attach an absolute root to its
containing-block owner and convert its local position into that owner's content
coordinates. For an untransformed block containing rectangle, subtract the
owner's left/top padding from the solved position. Ordinary descendants then
continue to compose unchanged.

For example, an owner with 200px content width and 20px padding on each side
supplies a 240px width basis; a 50% absolute width becomes 120px. Borders are
excluded. See [percentage-width basis](https://www.w3.org/TR/CSS22/visudet.html#the-width-property).

The initial containing block also needs an explicit external reference space;
it should not silently inherit the fixed branch's viewport semantics.

### 3. Schedule layout by containing-block dependencies

An ancestor's auto height may become known only after its in-flow contents
finish layout. Percentage heights on absolute children can then resolve against
that completed containing block. See
[percentage-height resolution](https://www.w3.org/TR/CSS22/visudet.html#the-height-property).

Consequently, `FixedLayout`'s whole-forest preparation loop cannot be copied
unchanged. A proposed ordering is:

1. Complete the layout supplying a containing block.
2. Resolve and lay out its eligible absolute roots.
3. Complete their geometry before processing absolute roots that depend on them.

This also handles absolute descendants of fixed boxes and nested absolute boxes.
Fixed descendants would continue using the current viewport contract.

Structural validation can still happen upfront, and results can still be
published transactionally. The promise that every positioned width is ready
before any final callback must become a promise for each ready subtree.
An unresolved containing-block dimension must not be represented as a definite
zero dimension.

### 4. Preserve static-position information

When both insets on an axis are `auto`, positioning depends on where the box
would have participated in flow. That reference can come from a different
formatting context than its actual containing block. See
[static-position rules](https://www.w3.org/TR/css-position-3/#static-position).

Current diversion discards that insertion location, and `InlineResult` exposes
neither static anchors nor ordinary inline-fragment geometry. Supporting this
requires preserving source-position metadata and, for inline cases, extending
the typography interface to return anchor information.

Such anchors must contribute no flow size or intrinsic width. Treating the
positioned box as an ordinary realized atomic would violate the exclusion
established during fixed-box construction.

### 5. Broaden the typed model for containing-block establishment

`position: relative` is particularly relevant: it remains in flow while
establishing a containing block. Other properties can also establish containing
blocks, and fixed and absolute containing-block selection are distinct. See
[positioning and containing blocks](https://www.w3.org/TR/css-position-3/#def-cb).

Adding `PositionAbsolute` alone would not support the common relatively
positioned parent case. We need either supported relative positioning or an
explicit adapter-provided containing-block capability, with clearly defined
semantics. Inline containing blocks additionally need fragment geometry that
core does not currently receive.

## Possible implementation boundary when work resumes

A practical first slice could share the existing solver and support explicit
insets against resolved block containing rectangles, retaining today's LTR and
non-replaced restrictions. Full source-tree integration would then add owner
associations and dependency scheduling. Static-position anchors and inline
containing blocks warrant a separate extension.

The existing `FixedStyle` constraints could become a shared positioned-style
record while preserving compatibility. Keep the public `FixedLayout` contract
stable. An `AbsoluteLayout` signature should remain undecided until
containing-block ownership and result coordinates are agreed.

This investigation made no implementation changes. Work is postponed; these
proposals should be revisited before implementation is authorized.
