package chart_test

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"github.com/timzifer/figure"
	"github.com/timzifer/fyne_figure/chart"
)

func TestATooltipSaysWhatIsUnderThePointer(t *testing.T) {
	c := chart.New(plot(), chart.ThemeFont(false),
		chart.TooltipFormat(func(h figure.Hit) string { return fmt.Sprintf("at %.2f", h.X) }))
	win := test.NewTempWindow(t, c)
	win.Resize(fyne.NewSize(500, 300))
	c.Resize(fyne.NewSize(500, 300))

	label := tooltipImage(t, c)
	if label.Visible() {
		t.Fatal("the tooltip is showing before the pointer has been anywhere")
	}

	if !hoverAMark(t, c, win) {
		t.Skip("no position over this chart found a mark")
	}
	if !label.Visible() {
		t.Fatal("hovering a mark showed no tooltip")
	}
	if chart.TipText(c) == "" {
		t.Error("the tooltip is showing but says nothing")
	}
	if label.Size().IsZero() {
		t.Error("the tooltip is showing but has no size")
	}
	if err := c.Err(); err != nil {
		t.Errorf("drawing the tooltip: %v", err)
	}

	c.MouseOut()
	if label.Visible() {
		t.Error("the tooltip is still showing after the pointer left the chart")
	}
}

func TestATooltipCanBeTurnedOff(t *testing.T) {
	c := chart.New(plot(), chart.ThemeFont(false), chart.Tooltip(false))
	win := test.NewTempWindow(t, c)
	win.Resize(fyne.NewSize(500, 300))
	c.Resize(fyne.NewSize(500, 300))

	hoverAMark(t, c, win)
	if tooltipImage(t, c).Visible() {
		t.Error("a chart with the tooltip turned off showed one")
	}
}

// A tooltip is drawn by the rasterizer rather than by Fyne's text engine
// exactly so that it carries the symbols a chart uses: Fyne draws U+FFFD for a
// character the theme font has no glyph for, and NotoSans has none for U+2264.
func TestATooltipDrawsTheSymbolsAChartUses(t *testing.T) {
	plain := tooltipInk(t, "xxxx")
	for _, symbols := range []string{"≤≤≤≤", "≥≥≥≥", "∞∞∞∞"} {
		t.Run(symbols, func(t *testing.T) {
			if got := tooltipInk(t, symbols); got == 0 {
				t.Fatalf("a tooltip of %q drew nothing at all", symbols)
			} else if got*4 < plain {
				t.Errorf("a tooltip of %q drew %d ink against %d for letters, want a comparable amount",
					symbols, got, plain)
			}
		})
	}
}

func TestATooltipDrawsEveryLineOfALabel(t *testing.T) {
	one := tooltipBox(t, "one line")
	two := tooltipBox(t, "one line\nand another")
	if two.Height <= one.Height {
		t.Errorf("a two-line tooltip is %v high against %v for one line, want taller", two.Height, one.Height)
	}
}

func TestATooltipTakesItsStyleFromTheOptions(t *testing.T) {
	want := chart.TooltipStyle{Padding: 20, FontSize: 30}
	c := chart.New(plot(), chart.ThemeFont(false), chart.TooltipLook(want))
	win := test.NewTempWindow(t, c)
	win.Resize(fyne.NewSize(500, 300))
	c.Resize(fyne.NewSize(500, 300))

	got := chart.TipContent(c, figure.Hit{X: 1, Y: 2})
	if got.Style.Padding != want.Padding || got.Style.FontSize != want.FontSize {
		t.Errorf("the tooltip's style is %+v, want the padding and size from TooltipLook", got.Style)
	}
	if got.Style.Background == nil {
		t.Error("a style that named neither colour lost the theme's background")
	}
}

// TooltipContentFunc says what a tooltip looks like as well as what it says,
// and what it leaves out still comes from the theme.
func TestATooltipContentCarriesItsOwnStyle(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	c := chart.New(plot(), chart.ThemeFont(false),
		chart.TooltipLook(chart.TooltipStyle{Padding: 3}),
		chart.TooltipContentFunc(func(h figure.Hit) chart.TooltipContent {
			return chart.TooltipContent{
				Text:  fmt.Sprintf("%.1f", h.X),
				Style: chart.TooltipStyle{Text: red, Bold: true},
			}
		}))
	win := test.NewTempWindow(t, c)
	win.Resize(fyne.NewSize(500, 300))
	c.Resize(fyne.NewSize(500, 300))

	got := chart.TipContent(c, figure.Hit{X: 1.5})
	if got.Text != "1.5" {
		t.Errorf("the tooltip says %q, want %q", got.Text, "1.5")
	}
	if got.Style.Text != color.Color(red) {
		t.Errorf("the tooltip's text colour is %v, want the one the content named", got.Style.Text)
	}
	if !got.Style.Bold {
		t.Error("the content asked for bold and did not get it")
	}
	if got.Style.Padding != 3 {
		t.Errorf("the padding is %v, want the 3 from TooltipLook", got.Style.Padding)
	}
}

// A Tooltipper is the same extension point for a type rather than a function.
type unitTip struct{ unit string }

func (u unitTip) Tooltip(h figure.Hit) chart.TooltipContent {
	return chart.TooltipContent{Text: fmt.Sprintf("%.0f %s", h.Y, u.unit)}
}

func TestATooltipperReplacesTheFormat(t *testing.T) {
	c := chart.New(plot(), chart.ThemeFont(false), chart.TooltipWith(unitTip{unit: "bar"}))
	win := test.NewTempWindow(t, c)
	win.Resize(fyne.NewSize(500, 300))
	c.Resize(fyne.NewSize(500, 300))

	if got := chart.TipContent(c, figure.Hit{Y: 7}).Text; got != "7 bar" {
		t.Errorf("the tooltip says %q, want %q", got, "7 bar")
	}
}

// hoverAMark sweeps the plot area until the pointer finds something, and
// reports whether it did.
func hoverAMark(t *testing.T, c *chart.Chart, win fyne.Window) bool {
	t.Helper()
	var found bool
	c.Plot().On(figure.Hover, func(ev figure.Event) { found = found || ev.Found })
	for x := float32(60); x < 460 && !found; x += 8 {
		for y := float32(40); y < 260 && !found; y += 8 {
			test.MoveMouse(win.Canvas(), fyne.NewPos(x, y))
		}
	}
	return found
}

// tooltipImage digs the tooltip out of the renderer, which is the only place
// it is: a tooltip is chrome and has no API of its own.
func tooltipImage(t *testing.T, c *chart.Chart) *canvas.Image {
	t.Helper()
	for _, o := range test.WidgetRenderer(c).Objects() {
		if img, ok := o.(*canvas.Image); ok {
			return img
		}
	}
	t.Fatal("the chart's renderer has no tooltip")
	return nil
}

// shownTooltip hovers a chart whose tooltip says label, and returns the object
// it was drawn into.
func shownTooltip(t *testing.T, label string) *canvas.Image {
	t.Helper()
	c := chart.New(plot(), chart.ThemeFont(false),
		chart.TooltipFormat(func(figure.Hit) string { return label }))
	win := test.NewTempWindow(t, c)
	win.Resize(fyne.NewSize(500, 300))
	c.Resize(fyne.NewSize(500, 300))
	if !hoverAMark(t, c, win) {
		t.Skip("no position over this chart found a mark")
	}
	if err := c.Err(); err != nil {
		t.Fatalf("drawing the tooltip: %v", err)
	}
	img := tooltipImage(t, c)
	if !img.Visible() || img.Image == nil {
		t.Fatal("hovering a mark drew no tooltip")
	}
	return img
}

// tooltipBox is the size a tooltip of the given label was drawn at.
func tooltipBox(t *testing.T, label string) fyne.Size {
	t.Helper()
	return shownTooltip(t, label).Size()
}

// tooltipInk counts the pixels of a tooltip that are neither its background
// nor its border — the label's own, which is what a dropped glyph loses.
func tooltipInk(t *testing.T, label string) int {
	t.Helper()
	img := shownTooltip(t, label).Image
	b := img.Bounds()

	// The middle of the box, away from the rounded corners and the outline.
	inset := b.Dy() / 4
	box := image.Rect(b.Min.X+inset, b.Min.Y+inset, b.Max.X-inset, b.Max.Y-inset)
	fill := img.At(box.Min.X, box.Min.Y)
	n := 0
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			if img.At(x, y) != fill {
				n++
			}
		}
	}
	return n
}
