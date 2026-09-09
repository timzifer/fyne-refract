package chart_test

import (
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	"github.com/timzifer/fyne-figure/chart"
)

// A chart has furniture a pointer can find: a legend row, a
// colourbar band, a size key sample. A hover in the margins reports one, which
// is what makes a clickable legend possible — and what a tooltip has to know
// about, because a guide carries no X and no Y.

// A chart with two named series, which is what gives it a legend: figure draws
// none for a single layer, and a legend is what half of these tests point at.
func legendPlot() *figure.Plot {
	const n = 60
	x := make([]float64, n)
	a := make([]float64, n)
	b := make([]float64, n)
	for i := range n {
		v := float64(i) / 6
		x[i], a[i], b[i] = v, math.Sin(v), math.Cos(v)
	}
	src := figure.Float64Columns(map[string][]float64{"t": x, "a": a, "b": b})

	p := figure.New(figure.Size(400, 250), figure.Title("Signal"))
	p.X(scale.Linear())
	p.Add(geom.Line(src, geom.X("t"), geom.Y("a"), geom.Label("alpha")))
	p.Add(geom.Line(src, geom.X("t"), geom.Y("b"), geom.Label("beta")))
	return p
}

// shownAt lays a chart out in a window, which is where a Live is born.
func shownAt(t *testing.T, c *chart.Chart) fyne.Window {
	t.Helper()
	win := test.NewTempWindow(t, c)
	win.Resize(fyne.NewSize(500, 300))
	c.Resize(fyne.NewSize(500, 300))
	c.Redraw()
	return win
}

// legendAt finds a position over a legend row, by asking the chart what is
// under each of a grid of points until one of them is furniture.
func legendAt(t *testing.T, c *chart.Chart) (fyne.Position, figure.Hit) {
	t.Helper()
	var found figure.Hit
	var ok bool
	c.Plot().On(figure.Hover, func(ev figure.Event) {
		if ev.Found && ev.Hit.Kind == figure.LegendRow {
			found, ok = ev.Hit, true
		}
	})
	for y := float32(4); y < 300; y += 4 {
		for x := float32(4); x < 500; x += 4 {
			ok = false
			c.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(x, y)}})
			if ok {
				return fyne.NewPos(x, y), found
			}
		}
	}
	t.Skip("no position over this chart found a legend row")
	return fyne.Position{}, figure.Hit{}
}

// A hover in the margins finds a legend row, and a legend
// row carries no X and no Y. A tooltip that described one would say "x 0, y 0"
// beside the series name, which is a lie about where the pointer is.
func TestATooltipSaysNothingAboutTheLegend(t *testing.T) {
	c := chart.New(legendPlot(), chart.ThemeFont(false))
	shownAt(t, c)

	pos, hit := legendAt(t, c)
	if hit.Series == "" {
		t.Fatal("the legend row that was found stands for no series")
	}
	if tooltipImage(t, c).Visible() {
		t.Errorf("hovering the legend row %q at %v showed a tooltip saying %q",
			hit.Series, pos, chart.TipText(c))
	}
}

// A hover over a mark still shows one. The gate is on the kind of hit and not
// on the tooltip.
func TestATooltipStillSaysWhatIsUnderAMark(t *testing.T) {
	c := chart.New(legendPlot(), chart.ThemeFont(false))
	win := shownAt(t, c)

	if !hoverAMark(t, c, win) {
		t.Skip("no position over this chart found a mark")
	}
	if !tooltipImage(t, c).Visible() {
		t.Fatal("hovering a mark showed no tooltip")
	}
}
