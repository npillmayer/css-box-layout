# CSS-Layout Software Implementation Idea (Historical Vision)

This note records the original project vision, not the current implementation
contract or completion gate. See `doc/CSS-Algo-Overview.md` for current scope
and `doc/Exec-Roadmap.md` for delivery milestones.

We are on our way to implement an HTML/CSS styling and layout engine. This engine should then be available as a library/package for other software applications to use. This may not be restricted to browsers, but to all kinds of UI apps with a need to lay out textual information.

## Core Concept
We stick to the constructions of render- and layout-trees. No rendering on a canvas is done (to be done by possible host applications).
We will focus on clarity and understandability of the information. Performance, caching, parallelism are no design goals at this point.

## Original End-to-End Vision
We are done when from a textual input of HTML and CSS respectively a layout-tree has been constructed. This layout-tree has to contain all the necessary positioning information to layout the CSS boxes on a canvas. This tree is in memory.
The current core starts from an already styled CSSDOM and produces geometry
with paragraph extents supplied by an external engine; HTML/CSS parsing and
styling are external responsibilities. Typed paragraph results stay in the
integration layer.
The CSS2 box/flow subset and its deferrals are defined in the core contract.
Flexbox is a possible future direction, not a milestone in the current roadmap.

## Functional Programming Paradigm
Tree modifications are a sweet-spot for functional design patterns. The final package will not be in a functional programming language, but we will always work our way with immutable data and functional paradigms in mind. The design phase may use some kind of functional pseudo-code or -- if appropriate -- Scala code. The final package will be written in Go (Golang), with pure transforms and explicit inputs/outputs as design requirements. No
functional-programming dependency is required by the current design.

## Original Assumption About External Code
Some code already exists for creating the render tree (all dynamic styles computed). For now, we will disregard this code, but at a later stage we may focus on this topic again. Let us start with the assumption, that creating the render tree is a solved problem.
