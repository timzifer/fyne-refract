package chart

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"github.com/timzifer/refract"
)

// tooltip is what a hover says, drawn over the chart rather than into it.
//
// It is Fyne objects rather than refract marks on purpose: a tooltip is chrome
// and belongs to the toolkit, it has to sit outside the plot area without
// being clipped by it, and drawing one into the chart would mean rasterizing
// the whole frame again on every pointer move.
type tooltip struct {
	c    *Chart
	bg   *canvas.Rectangle
	text *canvas.Text
}

// tooltipPad is the space between the tooltip's text and its edge, and between
// the tooltip and the pointer.
const tooltipPad = 6

func newTooltip(c *Chart) *tooltip {
	th := c.Theme()
	var v fyne.ThemeVariant
	if app := fyne.CurrentApp(); app != nil {
		v = app.Settings().ThemeVariant()
	}

	bg := canvas.NewRectangle(th.Color(theme.ColorNameOverlayBackground, v))
	bg.StrokeColor = th.Color(theme.ColorNameSeparator, v)
	bg.StrokeWidth = 1
	bg.CornerRadius = th.Size(theme.SizeNameSelectionRadius)
	bg.Hide()

	text := canvas.NewText("", th.Color(theme.ColorNameForeground, v))
	text.TextSize = th.Size(theme.SizeNameCaptionText)
	text.Hide()

	return &tooltip{c: c, bg: bg, text: text}
}

func (t *tooltip) objects() []fyne.CanvasObject {
	if t == nil {
		return nil
	}
	return []fyne.CanvasObject{t.bg, t.text}
}

// show places the tooltip for a hover, or hides it when the hover found
// nothing.
func (t *tooltip) show(ev refract.Event) {
	// A plot keeps its handlers for good, so a chart that has been closed —
	// and a second chart on the same plot — can still be told about a hover
	// that is not its own. A chart with nothing open has nothing to say.
	if t == nil || !t.c.cfg.tooltip || t.c.live == nil {
		return
	}
	if !ev.Found {
		t.hide()
		return
	}
	label := t.c.cfg.format(ev.Hit)
	if label == "" {
		t.hide()
		return
	}

	if label != t.text.Text {
		t.text.Text = label
		t.text.Refresh()
	}
	size := fyne.MeasureText(label, t.text.TextSize, t.text.TextStyle)
	box := fyne.NewSize(size.Width+2*tooltipPad, size.Height+2*tooltipPad)
	at := t.place(fyne.NewPos(ev.Point.X, ev.Point.Y), box)

	t.bg.Move(at)
	t.bg.Resize(box)
	t.text.Move(at.AddXY(tooltipPad, tooltipPad))
	t.text.Resize(size)

	if !t.bg.Visible() {
		t.bg.Show()
		t.text.Show()
	}
	canvas.Refresh(t.bg)
	canvas.Refresh(t.text)
}

// hide takes the tooltip off the chart.
func (t *tooltip) hide() {
	if t == nil || !t.bg.Visible() {
		return
	}
	t.bg.Hide()
	t.text.Hide()
	canvas.Refresh(t.bg)
}

// place puts the tooltip beside the pointer, and on the other side of it
// rather than off the edge when there is no room.
func (t *tooltip) place(at fyne.Position, box fyne.Size) fyne.Position {
	size := t.c.Size()
	x, y := at.X+tooltipPad, at.Y+tooltipPad
	if x+box.Width > size.Width {
		x = at.X - tooltipPad - box.Width
	}
	if y+box.Height > size.Height {
		y = at.Y - tooltipPad - box.Height
	}
	return fyne.NewPos(max32(x, 0), max32(y, 0))
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
