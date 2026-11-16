package layout

import (
	"errors"
	"reflect"
	"testing"

	"github.com/npillmayer/css-box-layout/text"
)

func TestWhitespaceBoxGeneration(t *testing.T) {
	for _, mode := range []WhiteSpace{WhiteSpaceNormal, WhiteSpaceNowrap, WhiteSpacePre, WhiteSpacePreWrap, WhiteSpacePreLine, WhiteSpaceBreakSpaces} {
		for _, content := range []string{" \t", "\n", "\r", "\u00a0", "\u2028", "x"} {
			for _, container := range []Display{blockDisplay, flowRootDisplay, inlineBlockDisplay} {
				ws := newRenderText(3, content)
				span := newRenderElement(2, inlineDisplay, ws, newRenderElement(4, blockDisplay), newRenderText(5, content))
				s := DefaultComputedStyle()
				s.WhiteSpace = mode
				s.FontSizePx = 99 // font size cannot create an empty line
				span.Style = &s
				parent := newRenderElement(6, container, span)
				root, err := BuildLayoutTree(newRenderElement(1, blockDisplay, parent), BuildOptions{})
				if err != nil {
					t.Fatal(err)
				}
				n := root.Children[0]
				if container == inlineBlockDisplay {
					n = n.Children[0]
				}
				collapse := (mode == WhiteSpaceNormal || mode == WhiteSpaceNowrap) && (content == " \t" || content == "\n" || content == "\r") || mode == WhiteSpacePreLine && content == " \t"
				count := 3
				if collapse {
					count = 1
				}
				if len(n.Children) != count {
					t.Fatalf("mode=%v content=%q container=%v children=%d want=%d", mode, content, container, len(n.Children), count)
				}
				if len(span.ChildrenNodes) != 3 {
					t.Fatal("mutated input")
				}
			}
		}
	}
}

func TestWhitespaceInheritanceAndEdges(t *testing.T) {
	for _, edge := range []string{"none", "margin", "padding", "border", "atomic", "text"} {
		span := newRenderElement(2, inlineDisplay, newRenderText(3, " "))
		s := DefaultComputedStyle()
		switch edge {
		case "margin":
			s.Margin.Left = lenPx(-1)
		case "padding":
			s.Padding.Top = lenPx(1)
		case "border":
			s.Border.Bottom = lenPx(1)
		case "atomic":
			span.ChildrenNodes = append(span.ChildrenNodes, newRenderElement(7, inlineBlockDisplay))
		case "text":
			span.ChildrenNodes = append(span.ChildrenNodes, newRenderText(8, "x"))
		}
		span.Style = &s
		root, err := BuildLayoutTree(newRenderElement(1, blockDisplay, span, newRenderElement(4, blockDisplay)), BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		count := 2
		if edge == "none" {
			count = 1
		}
		if len(root.Children) != count {
			t.Fatalf("%s: got %d", edge, len(root.Children))
		}
	}
	// nil child styles inherit pre; explicit normal overrides it.
	parent := newRenderElement(1, blockDisplay, newRenderText(2, " "), newRenderElement(3, blockDisplay))
	s := DefaultComputedStyle()
	s.WhiteSpace = WhiteSpacePre
	parent.Style = &s
	root, err := BuildLayoutTree(parent, BuildOptions{})
	if err != nil || len(root.Children) != 2 || root.Children[0].Style.WhiteSpace != WhiteSpacePre {
		t.Fatal("inheritance", root, err)
	}
	normal := DefaultComputedStyle()
	parent.ChildrenNodes[0].Style = &normal
	root, err = BuildLayoutTree(parent, BuildOptions{})
	if err != nil || len(root.Children) != 1 {
		t.Fatal("override", root, err)
	}
}

func TestEmptyBlocksKeepPrincipalBoxAndGroup(t *testing.T) {
	for _, children := range [][]*RenderNode{nil, {newRenderText(2, " \n")}} {
		root, err := BuildLayoutTree(newRenderElement(1, blockDisplay, children...), BuildOptions{})
		if err != nil || root.Box != BoxBlock || len(root.Children) != 1 || root.Children[0].Box != BoxAnonymousInline {
			t.Fatal(root, err)
		}
	}
}

func TestWhitespaceReadableSource(t *testing.T) {
	ws := newRenderText(2, "x") // misleading HTML text must not override authoritative reader
	ws.Text = &text.TextRef{Source: 9, Range: text.TextRange{Start: 20, End: 21}}
	input := newRenderElement(1, blockDisplay, ws, newRenderElement(3, blockDisplay))
	original := *ws.Text
	sentinel := errors.New("read failed")
	for _, tc := range []struct {
		content string
		err     error
		bad     bool
	}{{" ", nil, false}, {"xx", nil, true}, {"\xff", nil, true}, {"", sentinel, true}} {
		root, err := BuildLayoutTree(input, BuildOptions{ReadText: func(ref text.TextRef) (string, error) {
			if ref != original {
				t.Fatal(ref)
			}
			return tc.content, tc.err
		}})
		if tc.bad {
			if err == nil || root != nil {
				t.Fatal("invalid reader accepted")
			}
			if tc.err != nil && !errors.Is(err, sentinel) {
				t.Fatal(err)
			}
		} else if err != nil || len(root.Children) != 1 {
			t.Fatal(root, err)
		}
	}
	if !reflect.DeepEqual(*ws.Text, original) {
		t.Fatal("mutated reference")
	}
	s := DefaultComputedStyle()
	s.WhiteSpace = 99
	input.Style = &s
	if root, err := BuildLayoutTree(input, BuildOptions{}); err == nil || root != nil {
		t.Fatal("invalid white-space accepted")
	}
}
