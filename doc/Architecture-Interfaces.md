# Architecture and Integration Interfaces

The core layout algorithm consumes a styled DOM, text references, and external
inline-layout services. External implementations connect through adapters;
they should not dictate the core algorithm's structure.

This diagram shows the intended integration, not a package-import graph.
The algorithm contract is [CSS-Algo-Overview.md](CSS-Algo-Overview.md);
current Go signatures are documented in [Pass-Signatures.md](Pass-Signatures.md).

```mermaid
flowchart TB
    subgraph External["External implementations"]
        DOM["CSSDOM repository"]
        TEXT["Text storage"]
        BREAK["Line breaking / text layout"]
    end

    subgraph Boundary["Integration interfaces"]
        CSS["Read-only CSSDOM interface<br/>Proposed: StyNodeView"]
        TS["Text source interface"]
        IL["InlineLayouter<br/>and IntrinsicMeasurer"]
    end

    DOM -->|"adapter: identity, children, styles, text references"| CSS
    TEXT --> TS
    BREAK --> IL

    subgraph Core["Core layout algorithm"]
        BUILD["1. BuildLayoutTree"]
        TREE["Layout box tree"]
        USED["2. ResolveUsedValues"]
        FLOW["3. FlowLayout"]
        RESULT["Box geometry"]

        BUILD --> TREE
        TREE --> USED
        TREE --> FLOW
        USED -->|"used edges and widths"| FLOW
        FLOW --> RESULT
    end

    CSS --> BUILD
    TS -.->|"text access through references"| IL
    FLOW -->|"core-owned recursive preparation"| FLOW
    FLOW -->|"delegate ready group + prepared atomic items / intrinsic queries"| IL
    IL -->|"proposed: extent + atomic box placements"| FLOW
```

## Interface responsibilities and current status

- **CSSDOM:** intended to expose source identity, ordered children, computed
  styles, and text references through a read-only adapter. `StyNodeView` is an
  incomplete proposal; BuildLayoutTree currently accepts the concrete local
  `RenderNode` stub. The proposal still exposes `html.Node` and lacks explicit
  identity and text-source access. See [Tree-Type.md](Tree-Type.md).
- **Text:** layout boxes carry `TextRef` source IDs and byte ranges. The current
  `TextSource` interface exposes identity and byte length; source lookup and
  actual text access for external layout/shaping still need adapter design.
  The dashed edge represents that intended connection, not completed wiring.
- **Inline layout:** the current `InlineLayouter` returns overall extent only;
  task 2A proposes InlineResult with extent plus atomic-box placements, not lines.
  Its current request
  includes group/owner identities and a read-only used-values view. Typed paragraph
  results stay in an invocation-local adapter table; the integration layer joins
  them with geometry after success. `IntrinsicMeasurer` supplies max-content
  widths independently; it need not share an implementation.
- **Atomic preparation (target):** core discovers and recursively lays out atomic
  subtrees before delegating each group. The adapter receives read-only prepared
  border-box items keyed by BoxID, translates them for P, and retains their final
  placements. It does not initiate sizing. The diagram shows this target; Go still
  exposes the superseded AtomicSizer callback pending checklist 2A/3A migration.
  Concrete prepared lookup and placement output are proposed in Pass-Signatures.md,
  including branch lookup by BoxID and explicit geometry.RelativeTo. P's whatsit
  encoding/position storage remains private; the proposed types are not implemented;
  see [integration section 4.1](Document-Layout-Integration.md#41-core-owned-recursive-inline-block-preparation).

## Reading the pipeline

BuildLayoutTree normalizes structure. ResolveUsedValues resolves edges and known
widths. FlowLayout stacks blocks and delegates inline layout, producing box geometry. Lines and glyphs remain in the
adapter-owned paragraph result. CSSDOM creation, text storage, line breaking, and shaping remain
external responsibilities; shaping is outside this diagram's detail level.

The three passes express conceptual ownership. Auto-width inline-blocks still
need a scheduling design: descendants must use the final inline-block width,
and relevant edges must be resolved before their line breaking. The diagram
does not imply that one unconditional resolution traversal solves that dependency.
See [Current-Checklist.md](Current-Checklist.md), tasks 2 and 3.

## Layout process flowchart

This is a visual aid for the proposed fixed-width task-2A design, not a claim
about the current callback-based implementation. The written
[core contract](CSS-Algo-Overview.md) remains authoritative; concrete proposed
types are in [Pass-Signatures.md](Pass-Signatures.md#concrete-task-2a-proposal-not-implemented).
For the order of external calls, see the
[single-inline-block sequence](Document-Layout-Integration.md#single-inline-block-call-sequence).

After construction and used-value resolution, FlowLayout runs the recursive
container procedure below. Double-bordered boxes mean a recursive call to that
same procedure, followed by resuming the caller. Each atomic container's inner
groups finish before its outer group is delegated. Auto sizing is outside this
diagram's scope; fixed atomic descendant widths must be resolved before layout.

![Proposed layout flow: core recursively prepares atomic items before external paragraph layout, validates placements, and publishes only after root success.](diagrams/layout-process.svg)

[Editable DOT source](diagrams/layout-process.dot). Regenerate from the repository root:

```sh
dot -Tsvg doc/diagrams/layout-process.dot -o doc/diagrams/layout-process.svg
```

Publication happens only after the root returns successfully: the integration
layer publishes geometry and retained paragraph results together. Any error,
including during preparation or an external call, aborts the pending document;
the failure edge drawn above is representative, not the only failure source.
The previous successful snapshot remains unchanged.
