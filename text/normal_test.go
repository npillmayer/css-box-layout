package text

import "testing"

func TestCSSNormalWhitespace(t *testing.T) {
	for _, tc := range []struct{ in, out string }{{" \t\r\n\f", ""}, {" a \n b ", "a b"}, {"\u00a0", "\u00a0"}, {"\u2028\u2029\u0085\v", "\u2028\u2029\u0085\v"}, {"é\t אב", "é אב"}} {
		got, spans := NormalizeNormal(tc.in)
		if got != tc.out {
			t.Fatalf("%q -> %q want %q", tc.in, got, tc.out)
		}
		var source, output uint64
		for _, s := range spans {
			if s.SourceStart != source || s.Start != output {
				t.Fatal("source mapping gap", spans)
			}
			source = s.SourceEnd
			output = s.End
		}
		if source != uint64(len(tc.in)) || output != uint64(len(got)) {
			t.Fatal("incomplete source map")
		}
	}
}
