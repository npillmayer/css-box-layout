package layout

import "testing"

func TestStructuralFitDeferredVerticalBounds(t *testing.T) {
	for _, tc := range []struct {
		name        string
		top, bottom float32
	}{
		{"negative-leading", -20, 0}, {"oversized-trailing", 0, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := buildGroupFixture(t, newRenderElement(1, blockDisplay, newRenderText(2, "A")))
			style := DefaultComputedStyle()
			style.Margin.Top = lenPx(tc.top)
			style.Margin.Bottom = lenPx(tc.bottom)
			root.Style = &style
			f, err := NewStructuralFitter(root, fitTenPixelLines(map[BoxID]string{root.Children[0].BoxID: "A"}))
			if err != nil {
				t.Fatal(err)
			}
			if out := collectFits(t, f, FitRequest{Width: 100, Height: 10}); len(out) != 0 {
				t.Fatal("normal receiver accepted overflow")
			}
			out := collectFits(t, f, FitRequest{Width: 100, Height: 10, DeferVerticalBounds: true})
			if len(out) != 1 || !out[0].Complete() || out[0].Geometry()[root.BoxID].Frame.Y != tc.top {
				t.Fatal("lost intermediate geometry", out)
			}
			if out := collectFits(t, f, FitRequest{Width: 100, Height: 9, DeferVerticalBounds: true}); len(out) != 0 {
				t.Fatal("paragraph height ceiling bypassed")
			}
			style.Margin.Left = lenPx(-1)
			root.Style = &style
			f, err = NewStructuralFitter(root, fitTenPixelLines(map[BoxID]string{root.Children[0].BoxID: "A"}))
			if err != nil {
				t.Fatal(err)
			}
			if out := collectFits(t, f, FitRequest{Width: 100, Height: 10, DeferVerticalBounds: true}); len(out) != 0 {
				t.Fatal("horizontal bounds bypassed")
			}
		})
	}
}
