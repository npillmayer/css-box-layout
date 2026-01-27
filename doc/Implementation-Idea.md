# CSS-Layout Software Implementation Idea

We are on our way to implement an HTML/CSS styling and layout engine. This engine should then be available as a library/package for other software applications to use. This may not be restricted to browsers, but to all kinds of UI apps with a need to lay out textual information.

## Core Concept
We stick to the constructions of render- and layout-trees. No rendering on a canvas is done (to be done by possible host applications). 
We will focus on clarity and understandability of the information. Performance, caching, parallelism are no design goals at this point.

## Definition of Done
We are done when from a textual input of HTML and CSS respectively a layout-tree has been constructed. This layout-tree has to contain all the necessary positioning information to layout the CSS boxes on a canvas. This tree is in memory.
We will focus on CSS2 box layout first, and include Flexbox layout in a later stage.

## Functional Programming Paradigm
Tree modifications are a sweet-spot for functional design patterns. The final package will not be in a functional programming language, but we will always work our way with immutable data and functional paradigms in mind. The design phase may use some kind of functional pseudo-code or -- if appropriate -- Scala code. The final package will be written in Go (Golang), but we will include some packages to make it possible to emulate functional design patterns.
 
## Existing Code
Some code already exists for creating the render tree (all dynamic styles computed). For now, we will disregard this code, but at a later stage we may focus on this topic again. Let us start with the assumption, that creating the render tree is a solved problem.
