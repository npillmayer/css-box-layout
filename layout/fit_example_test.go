package layout_test

import (
	"context"
	"fmt"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/css-box-layout/text"
	"golang.org/x/net/html"
)

// These example payloads are values with no mutable referenced data.
type exampleFitCheckpoint struct{ offset uint64 }

func (c exampleFitCheckpoint) Clone() layout.FitPayload { return c }

type exampleFitOutput string

func (o exampleFitOutput) Clone() layout.FitPayload { return o }

// An intentionally synthetic service: each byte is one unit wide, lines are
// one unit high. Real clients delegate to their prepared paragraph backend.
type exampleParagraphFitter struct{ body string }

func (p exampleParagraphFitter) EnumerateParagraph(_ context.Context, req layout.ParagraphFitRequest, yield func(layout.ParagraphAlternative) bool) (layout.FitEnumeration, error) {
	start := 0
	if req.Checkpoint != nil {
		c, ok := req.Checkpoint.(exampleFitCheckpoint)
		if !ok || c.offset != req.From.Offset {
			return layout.FitFailed, fmt.Errorf("invalid example checkpoint")
		}
		start = int(c.offset)
	}
	if req.Width < 1 {
		return layout.FitExhausted, nil
	}
	for end, lines := start, 0; end < len(p.body); {
		end = min(len(p.body), end+int(req.Width))
		lines++
		a := layout.ParagraphAlternative{
			End: layout.ParagraphPosition{Offset: uint64(end)}, Complete: end == len(p.body),
			Extent: layout.InlineExtent{W: min(req.Width, float32(end-start)), H: float32(lines)},
			Output: exampleFitOutput(p.body[start:end]),
		}
		if !a.Complete {
			a.Checkpoint = exampleFitCheckpoint{uint64(end)}
		}
		if !yield(a) {
			return layout.FitStopped, nil
		}
	}
	return layout.FitExhausted, nil
}

func ExampleStructuralFitter() {
	tree, err := layout.BuildLayoutTree(&layout.RenderNode{
		ID: 1, HTML: &html.Node{Type: html.ElementNode, Data: "p"},
		Display: layout.Display{Kind: layout.DisplayBox, Outer: layout.OuterBlock, Inner: layout.InnerFlow},
		ChildrenNodes: []*layout.RenderNode{{
			ID: 2, HTML: &html.Node{Type: html.TextNode, Data: "abcd"},
			Text: &text.TextRef{Source: 1, Range: text.TextRange{End: 4}},
		}},
	}, layout.BuildOptions{})
	if err != nil {
		panic(err)
	}
	fitter, err := layout.NewStructuralFitter(tree, exampleParagraphFitter{"abcd"})
	if err != nil {
		panic(err)
	}
	var prefix layout.FitAlternative
	status, err := fitter.Enumerate(context.Background(), layout.FitRequest{Width: 2, Height: 1}, func(a layout.FitAlternative) bool {
		prefix = a
		return true
	})
	if err != nil || status != layout.FitExhausted {
		panic("incomplete enumeration")
	}
	fmt.Println(prefix.Paragraphs()[0].Output, prefix.Complete())

	// Retain the selected checkpoint. A narrower receiver changes only the
	// suffix; the already accepted prefix stays fixed.
	status, err = fitter.Enumerate(context.Background(), layout.FitRequest{
		Width: 1, Height: 2, Continuation: prefix.Continuation(),
	}, func(a layout.FitAlternative) bool {
		if a.Complete() {
			fmt.Println(a.Paragraphs()[0].Output, a.Complete())
		}
		return true
	})
	if err != nil || status != layout.FitExhausted {
		panic("incomplete enumeration")
	}
	fmt.Println("retained:", prefix.Paragraphs()[0].Output)
	// Output:
	// ab false
	// cd true
	// retained: ab
}
