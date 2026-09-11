package chart_test

import (
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/timzifer/figure"
	"github.com/timzifer/figure/data"
	"github.com/timzifer/figure/geom"
	"github.com/timzifer/figure/scale"
	fynefigure "github.com/timzifer/fyne-figure"
	"github.com/timzifer/fyne-figure/chart"
)

// keyedPlot is a scatter whose rows have names, so that a selection made here
// says something a chart over another table can act on.
func keyedPlot() *figure.Plot {
	const n = 24
	x := make([]float64, n)
	y := make([]float64, n)
	id := make([]string, n)
	for i := range n {
		v := float64(i) / 3
		x[i], y[i] = v, math.Sin(v)
		id[i] = string(rune('a' + i%26))
	}
	src := data.NewTable().Float64("t", x).Float64("y", y).String("id", id)

	p := figure.New(figure.Size(400, 250))
	p.X(scale.Linear())
	p.Add(geom.Scatter(src, geom.X("t"), geom.Y("y"), geom.KeyBy("id"), geom.Label("signal")))
	return p
}

// markAt finds a position over a data mark by asking the chart what is under
// each of a grid of points until one of them is a mark rather than furniture.
func markAt(t *testing.T, c *chart.Chart) (fyne.Position, figure.Hit) {
	t.Helper()
	var found figure.Hit
	var ok bool
	c.Plot().On(figure.Hover, func(ev figure.Event) {
		if ev.Found && !ev.Hit.Kind.Guides() && ev.Hit.Row >= 0 {
			found, ok = ev.Hit, true
		}
	})
	for y := float32(4); y < 300; y += 2 {
		for x := float32(4); x < 500; x += 2 {
			ok = false
			chart.PointerOf(c).MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(x, y)}})
			if ok {
				return fyne.NewPos(x, y), found
			}
		}
	}
	t.Fatal("no data mark was found anywhere in the chart")
	return fyne.Position{}, figure.Hit{}
}

// A click on a mark picks the row behind it, and says so in a vocabulary
// another chart understands: the key, not the row number.
func TestClickingAMarkPicksItsRow(t *testing.T) {
	c := chart.New(keyedPlot(), chart.Interactive(true), chart.ThemeFont(false), chart.Select(true))
	shownAt(t, c)

	var told fynefigure.Selection
	calls := 0
	c.OnSelect(func(s fynefigure.Selection) { told, calls = s, calls+1 })

	pos, hit := markAt(t, c)
	click(c, pos)

	sel := c.Selection()
	if len(sel) != 1 {
		t.Fatalf("clicking a mark picked %d rows, want 1", len(sel))
	}
	if sel[0].Row != hit.Row || sel[0].Layer != hit.Layer {
		t.Errorf("picked row %d of layer %d, want row %d of layer %d",
			sel[0].Row, sel[0].Layer, hit.Row, hit.Layer)
	}
	if sel[0].Key == "" {
		t.Error("the picked row carries no key, so nothing can act on it elsewhere")
	}
	if calls != 1 || !told.Equal(sel) {
		t.Errorf("the handler was told %v after %d calls, want %v after 1", told, calls, sel)
	}
	if err := c.Err(); err != nil {
		t.Errorf("drawing the chart with a selection: %v", err)
	}
}

// A chart that was not asked to select does not, which is the same rule the
// legend follows: figure wires nothing by itself and neither does a widget
// nobody asked.
func TestAChartDoesNotSelectUnlessAsked(t *testing.T) {
	// Row tracking on and selection off, so that the test is about the wiring
	// rather than about a hit that reported no row to pick.
	c := chart.New(keyedPlot(), chart.Interactive(true), chart.ThemeFont(false), chart.TrackRows(true))
	shownAt(t, c)

	pos, _ := markAt(t, c)
	click(c, pos)
	if len(c.Selection()) != 0 {
		t.Error("a chart without Select picked a row when a mark was clicked")
	}
}

// A click on nothing clears the selection. It is the gesture every reader
// already knows, and the only way to unpick the last row without finding it
// again.
func TestClickingNothingClearsTheSelection(t *testing.T) {
	c := chart.New(keyedPlot(), chart.Interactive(true), chart.ThemeFont(false), chart.Select(true))
	shownAt(t, c)

	pos, _ := markAt(t, c)
	click(c, pos)
	if len(c.Selection()) != 1 {
		t.Fatal("the row was not picked to begin with")
	}
	click(c, fyne.NewPos(2, 2))
	if got := c.Selection(); len(got) != 0 {
		t.Errorf("clicking outside every mark left %v picked", got)
	}
}

// Without MultiSelect a second click replaces the first, and clicking the row
// that is already picked unpicks it.
func TestOneClickPicksOneRow(t *testing.T) {
	c := chart.New(keyedPlot(), chart.Interactive(true), chart.ThemeFont(false), chart.Select(true))
	shownAt(t, c)

	pos, _ := markAt(t, c)
	click(c, pos)
	first := c.Selection()
	click(c, pos)
	if got := c.Selection(); len(got) != 0 {
		t.Errorf("clicking the picked row again left %v picked", got)
	}
	click(c, pos)
	if got := c.Selection(); !got.Equal(first) {
		t.Errorf("clicking it a third time picked %v, want %v", got, first)
	}
}

// A selection put into a chart is not a selection the reader made, so it
// reports nothing back. It is what stops two linked charts telling each other
// about one click for ever.
func TestSettingASelectionDoesNotReportIt(t *testing.T) {
	c := chart.New(keyedPlot(), chart.Interactive(true), chart.ThemeFont(false), chart.Select(true))
	shownAt(t, c)

	calls := 0
	c.OnSelect(func(fynefigure.Selection) { calls++ })

	c.SetSelection(fynefigure.Selection{{Key: "b", Layer: -1, Row: -1}})
	if calls != 0 {
		t.Errorf("SetSelection called the handler %d times", calls)
	}
	if got := c.Selection(); len(got) != 1 || got[0].Key != "b" {
		t.Errorf("the chart holds %v, want the row it was given", got)
	}
}

// Two charts are linked by one line each way, and the link settles rather than
// running away.
func TestTwoChartsShareOneSelection(t *testing.T) {
	left := chart.New(keyedPlot(), chart.Interactive(true), chart.ThemeFont(false), chart.Select(true))
	right := chart.New(keyedPlot(), chart.Interactive(true), chart.ThemeFont(false), chart.Select(true))
	shownAt(t, left)
	shownAt(t, right)

	left.OnSelect(func(s fynefigure.Selection) { right.SetSelection(s) })
	right.OnSelect(func(s fynefigure.Selection) { left.SetSelection(s) })

	pos, _ := markAt(t, left)
	click(left, pos)

	got := right.Selection()
	if !got.Equal(left.Selection()) {
		t.Fatalf("the right chart holds %v, the left %v", got, left.Selection())
	}
	if len(got) != 1 || got[0].Key == "" {
		t.Errorf("the row crossed as %v, which names no key", got)
	}
	if err := right.Err(); err != nil {
		t.Errorf("drawing the linked chart: %v", err)
	}
}

// A selection draws. The rings are figure's own overlay, over the finished
// chart, so the picture changes and the chart underneath does not.
func TestASelectionIsDrawn(t *testing.T) {
	c := chart.New(keyedPlot(), chart.Interactive(true), chart.ThemeFont(false), chart.Select(true))
	shownAt(t, c)

	pos, _ := markAt(t, c)
	before := pixels(t, c)
	click(c, pos)
	after := pixels(t, c)

	if before == after {
		t.Error("picking a row changed nothing on screen")
	}
	c.SetSelection(nil)
	if got := pixels(t, c); got != before {
		t.Error("clearing the selection did not put the chart back")
	}
}

// pixels is a cheap digest of what the chart is showing, for a test that only
// needs to know whether it changed.
func pixels(t *testing.T, c *chart.Chart) uint64 {
	t.Helper()
	img := c.Target().Image()
	if img == nil {
		t.Fatal("the chart has no pixels")
	}
	var sum uint64
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += 3 {
		for x := b.Min.X; x < b.Max.X; x += 3 {
			r, g, bl, a := img.At(x, y).RGBA()
			sum = sum*31 + uint64(r) + uint64(g)<<8 + uint64(bl)<<16 + uint64(a)<<24
		}
	}
	return sum
}
