package chart_test

import (
	"image"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/fyne-figure/chart"
)

// Fyne draws text through a shaper that falls back to another font for a
// character its theme font has no glyph for. The rasterizer that draws a chart
// is handed one font and cannot, so the same character comes out as nothing —
// silently, leaving a gap where the reader expected a symbol.
//
// Fyne's own theme font is NotoSans-Regular, which has no glyph for U+2264.
// So a chart drawn in it loses the ≤ from its title, which is why the labels
// are drawn in the rasterizer's own fonts unless asked otherwise.

func TestTheDefaultFontsDrawTheSymbolsAChartUses(t *testing.T) {
	for _, symbols := range []string{"≤≤≤≤", "≥≥≥≥", "∞∞∞∞"} {
		t.Run(symbols, func(t *testing.T) {
			plain := ink(t, titled(t, "xxxx"))
			symbol := ink(t, titled(t, symbols))
			if symbol == 0 {
				t.Fatalf("a title of %q drew nothing at all", symbols)
			}
			// Not a comparison of shapes, only of whether there is any: a
			// dropped glyph leaves the title band empty.
			if symbol*4 < plain {
				t.Errorf("a title of %q drew %d ink against %d for letters, want a comparable amount",
					symbols, symbol, plain)
			}
		})
	}
}

// titled renders a chart with the given title, at the package defaults.
func titled(t *testing.T, title string) image.Image {
	t.Helper()
	test.NewTempApp(t)

	p := figure.New(figure.Size(400, 250), figure.Title(title))
	p.Add(geom.Line(source(), geom.X("t"), geom.Y("y")))

	c := chart.New(p)
	win := test.NewTempWindow(t, c)
	win.Resize(fyne.NewSize(500, 300))
	c.Resize(fyne.NewSize(500, 300))
	if err := c.Err(); err != nil {
		t.Fatalf("laying the chart out: %v", err)
	}
	img := c.Target().Image()
	if img == nil {
		t.Fatal("the chart has no pixels")
	}
	return img
}

// ink counts the pixels in the title band that are not the page colour.
func ink(t *testing.T, img image.Image) int {
	t.Helper()
	b := img.Bounds()
	page := img.At(b.Min.X, b.Min.Y)
	band := b.Dy() / 8
	n := 0
	for y := b.Min.Y; y < b.Min.Y+band; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.At(x, y) != page {
				n++
			}
		}
	}
	return n
}
