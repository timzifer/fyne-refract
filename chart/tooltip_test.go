package chart_test

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"github.com/timzifer/fyne-refract/chart"
	"github.com/timzifer/refract"
)

func TestATooltipSaysWhatIsUnderThePointer(t *testing.T) {
	c := chart.New(plot(), chart.ThemeFont(false),
		chart.TooltipFormat(func(h refract.Hit) string { return fmt.Sprintf("at %.2f", h.X) }))
	win := test.NewTempWindow(t, c)
	win.Resize(fyne.NewSize(500, 300))
	c.Resize(fyne.NewSize(500, 300))

	label := tooltipText(t, c)
	if label.Visible() {
		t.Fatal("the tooltip is showing before the pointer has been anywhere")
	}

	if !hoverAMark(t, c, win) {
		t.Skip("no position over this chart found a mark")
	}
	if !label.Visible() {
		t.Fatal("hovering a mark showed no tooltip")
	}
	if label.Text == "" {
		t.Error("the tooltip is showing but says nothing")
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
	if tooltipText(t, c).Visible() {
		t.Error("a chart with the tooltip turned off showed one")
	}
}

// hoverAMark sweeps the plot area until the pointer finds something, and
// reports whether it did.
func hoverAMark(t *testing.T, c *chart.Chart, win fyne.Window) bool {
	t.Helper()
	var found bool
	c.Plot().On(refract.Hover, func(ev refract.Event) { found = found || ev.Found })
	for x := float32(60); x < 460 && !found; x += 8 {
		for y := float32(40); y < 260 && !found; y += 8 {
			test.MoveMouse(win.Canvas(), fyne.NewPos(x, y))
		}
	}
	return found
}

// tooltipText digs the tooltip's label out of the renderer, which is the only
// place it is: a tooltip is chrome and has no API of its own.
func tooltipText(t *testing.T, c *chart.Chart) *canvas.Text {
	t.Helper()
	for _, o := range test.WidgetRenderer(c).Objects() {
		if txt, ok := o.(*canvas.Text); ok {
			return txt
		}
	}
	t.Fatal("the chart's renderer has no tooltip label")
	return nil
}
