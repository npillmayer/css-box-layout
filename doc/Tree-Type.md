# Render-tree input contract

Core accepts the concrete typed `layout.RenderNode` projection. The unused
string-based StyNodeView proposal and RenderNode.ComputedStyle accessor have
been removed. Adapters supply source IDs, text references, normalized display
and box values explicitly; traversal uses ChildrenNodes order.

## Current RenderNode input contract

BuildLayoutTree still accepts the local RenderNode projection. It now carries:

- `ID NodeID`: source identity supplied by the adapter; split fragments retain it.
- `HTML *html.Node`: required element/text kind; filter other DOM nodes outside
  the core. The input is a render subtree, not an HTML document wrapper.
- `Style *ComputedStyle`: copied typed box values; nil means default auto width
  and zero edges. Start with `layout.DefaultComputedStyle()` when overriding only
  some properties. A non-nil zero-valued style specifies width 0px, not auto.
- `Text *text.TextRef`: a text node's authoritative source ID and byte range,
  copied by value into its box. Nonempty HTML text without a reference is an
  error. Supplied empty ranges are dropped; reversed ranges are rejected.
  Source-backed text may have empty HTML.Data when a reader is provided; source ID 0 is valid.
- `Display Display`: explicit `DisplayBox` with `OuterBlock`/`OuterInline` and
  `InnerFlow`/`InnerFlowRoot`, or `DisplayNone` with neither axis. Elements have
  no implicit default; text leaves this field zero. Inline flow-root maps to
  atomic inline-block. Block flow-root maps to `BoxBlock` with
  `LayoutNode.EstablishesBFC` set; it keeps normal block widths and outer margins.
- `Style.WhiteSpace`: computed CSS whitespace mode (zero is normal). Nil styles
  inherit their parent's mode. Core derives suppression from source bytes and
  inline edges, with no adapter permission flags.
- `BuildOptions.ReadText`: exact referenced UTF-8 bytes; read errors, incorrect
  lengths and invalid UTF-8 fail the build. Without a reader, HTML.Data must hold
  exactly the referenced range. The callback is used only during construction.
- `Styles`: adapter metadata. `Styles["display"]` is rejected even alongside
  typed display; known length/font-size strings also fail, requesting typed
  input. Metadata is not copied into layout styles; use NodeID to join immutable
  external typography/paint data.

The public entry point accepts a non-replaced block flow or block flow-root element root. Nil or
an element with DisplayNone produces no tree; other roots are rejected rather
than blockified. Suppression prunes the whole subtree before style construction.
Missing/malformed display reports ErrInvalidDisplay; unsupported roots report
ErrInvalidRoot. These wrapped errors identify the source NodeID. No CSS parser
or w3c-dom dependency is introduced. Tytui now projects w3c-dom GetDisplay values into this contract.

Non-replaced inline flow-root containers also set `EstablishesBFC`. Ordinary and
anonymous block containers leave it false; `FCBlock` alone does not establish a
BFC. The flag contains descendant margins and prevents through-collapse, while
a block flow-root's own margins remain eligible in its parent's flow. Supplied
root and directly constructed inline-block containment remain independent of
the flag. No float/clearance or other new layout model is implied.

Supported typed values are Width and per-side Margin/Padding/Border using existing
Length kinds, plus FontSizePx. Lengths must be finite; width/padding/border cannot
be negative. Auto is allowed for width/margins, not padding/border. Negative
margins are allowed. Percent values use fractions; em resolves against the node's
own computed font size. The adapter is responsible for CSS cascade, shorthand
expansion, border style effects, and conversion into this subset; accepting a
Length kind does not establish full CSS conformance.

FontSizePx is either positive or zero (inherit/fallback sentinel in this minimal
API). Explicit CSS font-size:0 is not representable yet. Source-parent inheritance
is captured before split/hoist; unresolved root font size uses ResolveContext's
FontSizePx. Anonymous wrappers have auto width, zero edges, and inherited font
size; source widths and edges are not inherited. Copied styles are treated as
immutable, including styles shared by split fragments.

The adapter owns source storage, source-ID uniqueness within its snapshot, and
source/range validation against actual bytes (including UTF-8 boundaries). The
builder checks range ordering but cannot check source bounds without that storage.
It never derives source IDs by truncating NodeID. Source lookup, richer typography
and paint, and selection/caret mapping remain external integration work.


## Fixed branch construction (2026-11-20)

Use BuildLayoutTrees when the projection includes `Style.Position = PositionFixed`.
It returns separate Flow and Fixed roots with one shared BoxID namespace. Static (zero Position) is unchanged; positioning is
not inherited. Fixed boxes are diverted before outer anonymous grouping and
split/hoist, blockified to BoxBlock, and set EstablishesBFC. Fixed interiors keep
normal normalization, while nested fixed elements become separate roots in source
preorder. Source NodeID and computed font/white-space are preserved.

ComputedStyle.Fixed is an optional *FixedStyle containing Insets and Height.
Nil means auto constraints; DefaultFixedStyle initializes auto values; a non-nil
zero record means explicit zero. The record is copied, not shared with the input.
Width remains on ComputedStyle. Static boxes must leave Fixed nil. The builder
accepts valid auto/px/em/percentage values but performs no fixed sizing.
Adapters must project complete computed white-space as before; a non-nil style
with zero WhiteSpace explicitly means normal rather than inheritance.

Non-replaced fixed roots accept all supported display pairs, including an inline
supplied root. Fixed replaced elements, positioned text and other positioning
schemes are rejected. Nil/display:none yields empty branches, without inspecting
suppressed descendants. Errors publish neither branch. The normal-only builder
returns ErrFixedLayoutRequired if it would lose a fixed branch; normal resolution
and FlowLayout reject fixed input too. See [the API](Pass-Signatures.md#fixed-layout-api).
FixedLayout itself is not yet implemented. No page-layout behavior is introduced.
